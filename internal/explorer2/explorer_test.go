package explorer2

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/vutran1710/dating-dev/internal/bucket"
	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/indexer2"
)

func buildTestIndex(t *testing.T, dir string) string {
	t.Helper()
	_, opPriv, _ := ed25519.GenerateKey(rand.Reader)
	opPub := opPriv.Public().(ed25519.PublicKey)

	usersDir := filepath.Join(dir, "users")
	os.MkdirAll(usersDir, 0755)

	profiles := []struct {
		hash    string
		profile map[string]any
	}{
		{"aaa", map[string]any{"name": "Alice", "role": "woman", "age": 25}},
		{"bbb", map[string]any{"name": "Bob", "role": "woman", "age": 28}},
		{"ccc", map[string]any{"name": "Carol", "role": "woman", "age": 26}},
	}

	for _, p := range profiles {
		pub, _, _ := ed25519.GenerateKey(rand.Reader)
		pJSON, _ := json.Marshal(p.profile)
		bin, _ := crypto.PackUserBin(pub, opPub, pJSON)
		os.WriteFile(filepath.Join(usersDir, p.hash+".bin"), bin, 0644)
	}

	indexPath := filepath.Join(dir, "index.db")
	err := indexer2.Build(indexer2.Config{
		UsersDir:     usersDir,
		OutputPath:   indexPath,
		OperatorKey:  opPriv,
		Partitions:   []bucket.PartitionConfig{{Field: "role"}},
		Permutations: 2,
		NonceSpace:   5,
		Salt:         "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	return indexPath
}

// grindAll calls GrindNext repeatedly until nil, collecting results.
func grindAll(t *testing.T, exp *Explorer, bucketID string, perm int) []GrindResult {
	t.Helper()
	var results []GrindResult
	for {
		r, err := exp.GrindNext(bucketID, perm)
		if err != nil {
			t.Fatal(err)
		}
		if r == nil {
			break
		}
		results = append(results, *r)
	}
	return results
}

func TestListBuckets(t *testing.T) {
	dir := t.TempDir()
	indexPath := buildTestIndex(t, dir)

	exp, err := Open(indexPath, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer exp.Close()

	buckets, err := exp.ListBuckets()
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) == 0 {
		t.Fatal("expected at least 1 bucket")
	}
	for _, b := range buckets {
		t.Logf("bucket: %s (profiles=%d, perms=%d)", b.ID, b.ProfileCount, b.Permutations)
	}
}

func TestGrindNext_SingleProfile(t *testing.T) {
	dir := t.TempDir()
	indexPath := buildTestIndex(t, dir)

	exp, err := Open(indexPath, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer exp.Close()

	buckets, _ := exp.ListBuckets()

	// GrindNext returns exactly 1 profile
	r, err := exp.GrindNext(buckets[0].ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if r == nil {
		t.Fatal("expected 1 result")
	}
	if r.Profile == nil {
		t.Error("profile should not be nil")
	}
	t.Logf("unlocked: %s (%d attempts, warm=%v) name=%v", r.Tag, r.Attempts, r.Warm, r.Profile["name"])
}

func TestGrindNext_AllProfiles(t *testing.T) {
	dir := t.TempDir()
	indexPath := buildTestIndex(t, dir)

	exp, err := Open(indexPath, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer exp.Close()

	buckets, _ := exp.ListBuckets()
	results := grindAll(t, exp, buckets[0].ID, 0)

	if len(results) != 3 {
		t.Errorf("expected 3 profiles, got %d", len(results))
	}

	for _, r := range results {
		t.Logf("  %s → %v (%d attempts)", r.Tag, r.Profile["name"], r.Attempts)
	}
}

func TestWarmResolve(t *testing.T) {
	dir := t.TempDir()
	indexPath := buildTestIndex(t, dir)

	exp, err := Open(indexPath, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer exp.Close()

	buckets, _ := exp.ListBuckets()
	bid := buckets[0].ID

	// Grind perm 0 cold — learn constants
	results0 := grindAll(t, exp, bid, 0)
	totalCold := 0
	for _, r := range results0 {
		totalCold += r.Attempts
	}

	// Grind perm 1 warm
	results1 := grindAll(t, exp, bid, 1)
	totalWarm := 0
	for _, r := range results1 {
		totalWarm += r.Attempts
		if !r.Warm {
			t.Errorf("%s should be warm", r.Tag)
		}
	}

	t.Logf("perm 0 (cold): %d total attempts", totalCold)
	t.Logf("perm 1 (warm): %d total attempts", totalWarm)

	if totalWarm >= totalCold {
		t.Errorf("warm (%d) should be less than cold (%d)", totalWarm, totalCold)
	}
}

func TestSeenSkip(t *testing.T) {
	dir := t.TempDir()
	indexPath := buildTestIndex(t, dir)

	exp, err := Open(indexPath, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer exp.Close()

	buckets, _ := exp.ListBuckets()
	bid := buckets[0].ID

	// Grind all
	results := grindAll(t, exp, bid, 0)

	// Mark all seen
	for _, r := range results {
		exp.MarkSeen(r.Tag, "skip")
	}

	// GrindNext should return nil (all seen)
	r, err := exp.GrindNext(bid, 0)
	if err != nil {
		t.Fatal(err)
	}
	if r != nil {
		t.Errorf("expected nil (all seen), got %s", r.Tag)
	}
}

func TestGrindNext_InvalidBucket(t *testing.T) {
	dir := t.TempDir()
	indexPath := buildTestIndex(t, dir)

	exp, err := Open(indexPath, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer exp.Close()

	_, err = exp.GrindNext("nonexistent_bucket", 0)
	if err == nil {
		t.Error("should fail with nonexistent bucket")
	}
}

func TestGrindNext_InvalidPermutation(t *testing.T) {
	dir := t.TempDir()
	indexPath := buildTestIndex(t, dir)

	exp, err := Open(indexPath, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer exp.Close()

	buckets, _ := exp.ListBuckets()
	_, err = exp.GrindNext(buckets[0].ID, 99)
	if err == nil {
		t.Error("should fail with invalid permutation")
	}
}

func TestEndToEnd(t *testing.T) {
	dir := t.TempDir()
	indexPath := buildTestIndex(t, dir)

	exp, err := Open(indexPath, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer exp.Close()

	buckets, _ := exp.ListBuckets()
	t.Logf("found %d buckets", len(buckets))
	bid := buckets[0].ID

	// Cold grind one by one
	coldTotal := 0
	coldResults := grindAll(t, exp, bid, 0)
	for _, r := range coldResults {
		coldTotal += r.Attempts
		t.Logf("  cold: %s → %v (%d attempts)", r.Tag, r.Profile["name"], r.Attempts)
	}

	// Warm grind (different permutation)
	warmTotal := 0
	warmResults := grindAll(t, exp, bid, 1)
	for _, r := range warmResults {
		warmTotal += r.Attempts
		t.Logf("  warm: %s → %v (%d attempts, warm=%v)", r.Tag, r.Profile["name"], r.Attempts, r.Warm)
	}

	t.Logf("cold total: %d, warm total: %d, speedup: %.1fx",
		coldTotal, warmTotal, float64(coldTotal)/float64(warmTotal))

	if warmTotal >= coldTotal {
		t.Errorf("warm should be faster than cold")
	}
}
