package suggestions

import (
	"os"
	"path/filepath"
	"testing"

	gh "github.com/vutran1710/dating-dev/internal/github"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestDB_InsertAndLoadAll(t *testing.T) {
	db := openTestDB(t)
	db.Upsert("hash1", []float32{1.0, 2.0, 3.0})
	db.Upsert("hash2", []float32{4.0, 5.0, 6.0})
	records, err := db.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
}

func TestDB_Upsert_ReplacesExisting(t *testing.T) {
	db := openTestDB(t)
	db.Upsert("hash1", []float32{1.0, 2.0})
	db.Upsert("hash1", []float32{3.0, 4.0}) // replace
	records, _ := db.LoadAll()
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].Vector[0] != 3.0 {
		t.Errorf("vec[0] = %f, want 3.0", records[0].Vector[0])
	}
}

func TestDB_EmptyDB(t *testing.T) {
	db := openTestDB(t)
	records, err := db.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestDB_SyncFromDir(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	indexDir := filepath.Join(dir, "index")
	os.MkdirAll(indexDir, 0700)

	gh.WriteVecFile(filepath.Join(indexDir, "aaa111.vec"), []float32{1.0, 2.0})
	gh.WriteVecFile(filepath.Join(indexDir, "bbb222.vec"), []float32{3.0, 4.0})

	added, err := db.SyncFromDir(indexDir)
	if err != nil {
		t.Fatal(err)
	}
	if added != 2 {
		t.Errorf("added = %d, want 2", added)
	}

	records, _ := db.LoadAll()
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
}

func TestDB_SyncFromDir_Incremental(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	indexDir := filepath.Join(dir, "index")
	os.MkdirAll(indexDir, 0700)

	// First sync
	gh.WriteVecFile(filepath.Join(indexDir, "aaa111.vec"), []float32{1.0, 2.0})
	db.SyncFromDir(indexDir)

	// Add one more
	gh.WriteVecFile(filepath.Join(indexDir, "bbb222.vec"), []float32{3.0, 4.0})
	added, _ := db.SyncFromDir(indexDir)
	if added != 1 {
		t.Errorf("incremental added = %d, want 1", added)
	}

	records, _ := db.LoadAll()
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
}

func TestDB_SyncFromDir_EmptyDir(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	added, err := db.SyncFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if added != 0 {
		t.Errorf("added = %d, want 0", added)
	}
}
