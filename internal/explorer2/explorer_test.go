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

func TestGrindCold(t *testing.T) {
	dir := t.TempDir()
	indexPath := buildTestIndex(t, dir)

	exp, err := Open(indexPath, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer exp.Close()

	buckets, _ := exp.ListBuckets()
	if len(buckets) == 0 {
		t.Fatal("no buckets")
	}

	results, err := exp.Grind(buckets[0].ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least 1 unlocked profile")
	}

	for _, r := range results {
		t.Logf("unlocked: %s (%d attempts, warm=%v) profile=%v", r.Tag, r.Attempts, r.Warm, r.Profile["name"])
		if r.Profile == nil {
			t.Error("profile should not be nil")
		}
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

	// Grind perm 0 — learn constants (cold)
	results0, _ := exp.Grind(bid, 0, 10)
	totalCold := 0
	for _, r := range results0 {
		totalCold += r.Attempts
	}

	// Grind perm 1 — should be warm
	results1, _ := exp.Grind(bid, 1, 10)
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

	// Grind once
	results, _ := exp.Grind(bid, 0, 10)

	// Mark all as seen
	for _, r := range results {
		exp.MarkSeen(r.Tag, "skip")
	}

	// Grind again — should return nothing (all seen)
	results2, _ := exp.Grind(bid, 0, 10)
	if len(results2) != 0 {
		t.Errorf("expected 0 results (all seen), got %d", len(results2))
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

	// Cold grind
	results0, _ := exp.Grind(bid, 0, 0)
	coldTotal := 0
	for _, r := range results0 {
		coldTotal += r.Attempts
		t.Logf("  cold: %s → %v (%d attempts)", r.Tag, r.Profile["name"], r.Attempts)
	}

	// Warm grind (different permutation)
	results1, _ := exp.Grind(bid, 1, 0)
	warmTotal := 0
	for _, r := range results1 {
		warmTotal += r.Attempts
		t.Logf("  warm: %s → %v (%d attempts, warm=%v)", r.Tag, r.Profile["name"], r.Attempts, r.Warm)
	}

	t.Logf("cold total: %d, warm total: %d, speedup: %.1fx",
		coldTotal, warmTotal, float64(coldTotal)/float64(warmTotal))

	if warmTotal >= coldTotal {
		t.Errorf("warm should be faster than cold")
	}
}
