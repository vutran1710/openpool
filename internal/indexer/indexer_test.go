package indexer

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/vutran1710/dating-dev/internal/bucket"
	"github.com/vutran1710/dating-dev/internal/crypto"
)

func createTestBinFile(t *testing.T, dir string, binHash string, profile map[string]any, opPub ed25519.PublicKey) {
	t.Helper()
	userPub, _, _ := ed25519.GenerateKey(rand.Reader)
	profileJSON, _ := json.Marshal(profile)
	binData, err := crypto.PackUserBin(userPub, opPub, profileJSON)
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(dir, "users"), 0755)
	os.WriteFile(filepath.Join(dir, "users", binHash+".bin"), binData, 0644)
}

func TestBuild(t *testing.T) {
	dir := t.TempDir()
	_, opPriv, _ := ed25519.GenerateKey(rand.Reader)
	opPub := opPriv.Public().(ed25519.PublicKey)

	createTestBinFile(t, dir, "aaa111", map[string]any{
		"role": "woman", "age": 25, "interests": []string{"coding"},
	}, opPub)
	createTestBinFile(t, dir, "bbb222", map[string]any{
		"role": "man", "age": 28, "interests": []string{"hiking"},
	}, opPub)
	createTestBinFile(t, dir, "ccc333", map[string]any{
		"role": "woman", "age": 30, "interests": []string{"music"},
	}, opPub)

	indexPath := filepath.Join(dir, "index.db")
	err := Build(Config{
		UsersDir:     filepath.Join(dir, "users"),
		OutputPath:   indexPath,
		OperatorKey:  opPriv,
		Partitions:   []bucket.PartitionConfig{{Field: "role"}},
		Permutations: 2,
		NonceSpace:   5,
		Salt:         "testsalt",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(indexPath); err != nil {
		t.Fatal("index.db not created")
	}

	stats, err := Stats(indexPath)
	if err != nil {
		t.Fatal(err)
	}

	if stats.Buckets < 2 {
		t.Errorf("expected at least 2 buckets, got %d", stats.Buckets)
	}
	if stats.Entries < 3 {
		t.Errorf("expected at least 3 entries, got %d", stats.Entries)
	}
	t.Logf("index stats: %+v", stats)
}

func TestBuild_RangePartitions(t *testing.T) {
	dir := t.TempDir()
	_, opPriv, _ := ed25519.GenerateKey(rand.Reader)
	opPub := opPriv.Public().(ed25519.PublicKey)

	createTestBinFile(t, dir, "aa", map[string]any{"age": 22}, opPub)
	createTestBinFile(t, dir, "bb", map[string]any{"age": 27}, opPub)
	createTestBinFile(t, dir, "cc", map[string]any{"age": 35}, opPub)

	indexPath := filepath.Join(dir, "index.db")
	err := Build(Config{
		UsersDir:     filepath.Join(dir, "users"),
		OutputPath:   indexPath,
		OperatorKey:  opPriv,
		Partitions:   []bucket.PartitionConfig{{Field: "age", Step: 10, Overlap: 3}},
		FieldRanges:  map[string][2]int{"age": {18, 40}},
		Permutations: 1,
		NonceSpace:   5,
		Salt:         "testsalt",
	})
	if err != nil {
		t.Fatal(err)
	}

	stats, _ := Stats(indexPath)
	t.Logf("range partition stats: %+v", stats)
}

func TestBuild_InvalidBinFile(t *testing.T) {
	dir := t.TempDir()
	usersDir := filepath.Join(dir, "users")
	os.MkdirAll(usersDir, 0755)
	_, opPriv, _ := ed25519.GenerateKey(rand.Reader)
	opPub := opPriv.Public().(ed25519.PublicKey)

	// Write one valid and one corrupt .bin file
	createTestBinFile(t, dir, "valid", map[string]any{"role": "woman"}, opPub)
	os.WriteFile(filepath.Join(usersDir, "corrupt.bin"), []byte("garbage"), 0644)

	indexPath := filepath.Join(dir, "index.db")
	err := Build(Config{
		UsersDir:     usersDir,
		OutputPath:   indexPath,
		OperatorKey:  opPriv,
		Partitions:   []bucket.PartitionConfig{{Field: "role"}},
		Permutations: 1,
		NonceSpace:   5,
		Salt:         "test",
	})
	// Should succeed — corrupt file skipped, valid file indexed
	if err != nil {
		t.Fatal(err)
	}

	stats, _ := Stats(indexPath)
	if stats.Entries != 1 {
		t.Errorf("expected 1 entry (corrupt skipped), got %d", stats.Entries)
	}
}

func TestBuild_WrongOperatorKey(t *testing.T) {
	dir := t.TempDir()
	_, opPriv1, _ := ed25519.GenerateKey(rand.Reader)
	_, opPriv2, _ := ed25519.GenerateKey(rand.Reader)
	opPub1 := opPriv1.Public().(ed25519.PublicKey)

	// Create .bin encrypted to key 1
	createTestBinFile(t, dir, "aaa", map[string]any{"role": "woman"}, opPub1)

	indexPath := filepath.Join(dir, "index.db")
	// Try building with key 2 — can't decrypt
	err := Build(Config{
		UsersDir:     filepath.Join(dir, "users"),
		OutputPath:   indexPath,
		OperatorKey:  opPriv2, // wrong key
		Partitions:   []bucket.PartitionConfig{{Field: "role"}},
		Permutations: 1,
		NonceSpace:   5,
		Salt:         "test",
	})
	// Should succeed but with 0 profiles (all failed to decrypt)
	if err != nil {
		t.Fatal(err)
	}
	stats, _ := Stats(indexPath)
	if stats.Entries != 0 {
		t.Errorf("expected 0 entries (wrong key), got %d", stats.Entries)
	}
}

func TestBuild_MissingUsersDir(t *testing.T) {
	dir := t.TempDir()
	_, opPriv, _ := ed25519.GenerateKey(rand.Reader)

	err := Build(Config{
		UsersDir:     filepath.Join(dir, "nonexistent"),
		OutputPath:   filepath.Join(dir, "index.db"),
		OperatorKey:  opPriv,
		Partitions:   []bucket.PartitionConfig{{Field: "role"}},
		Permutations: 1,
		NonceSpace:   5,
		Salt:         "test",
	})
	if err == nil {
		t.Error("should fail with missing users dir")
	}
}

func TestBuild_EmptyUsersDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "users"), 0755)
	_, opPriv, _ := ed25519.GenerateKey(rand.Reader)

	err := Build(Config{
		UsersDir:     filepath.Join(dir, "users"),
		OutputPath:   filepath.Join(dir, "index.db"),
		OperatorKey:  opPriv,
		Partitions:   []bucket.PartitionConfig{{Field: "role"}},
		Permutations: 1,
		NonceSpace:   5,
		Salt:         "testsalt",
	})
	if err != nil {
		t.Fatal(err)
	}
}
