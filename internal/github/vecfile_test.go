package github

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReadRecFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.rec")
	rec := IndexRecord{
		Filters: FilterValues{Fields: map[string]int{"gender": 1, "age": 28}},
		Vector:  []float32{1.5, -2.0, 3.14},
	}
	if err := WriteRecFile(path, rec); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRecFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Filters.Fields["gender"] != 1 {
		t.Errorf("gender = %d, want 1", got.Filters.Fields["gender"])
	}
	if got.Filters.Fields["age"] != 28 {
		t.Errorf("age = %d, want 28", got.Filters.Fields["age"])
	}
	if len(got.Vector) != 3 {
		t.Fatalf("vec len = %d, want 3", len(got.Vector))
	}
	if math.Abs(float64(got.Vector[2]-3.14)) > 0.01 {
		t.Errorf("vec[2] = %f, want 3.14", got.Vector[2])
	}
}

func TestWriteRecFile_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "nested", "test.rec")
	err := WriteRecFile(path, IndexRecord{Vector: []float32{1.0}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("file not created")
	}
}

func TestReadRecFile_NotFound(t *testing.T) {
	_, err := ReadRecFile("/nonexistent.rec")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadRecDir(t *testing.T) {
	dir := t.TempDir()
	indexDir := filepath.Join(dir, "index")
	os.MkdirAll(indexDir, 0700)

	WriteRecFile(filepath.Join(indexDir, "abc123.rec"), IndexRecord{
		Filters: FilterValues{Fields: map[string]int{"gender": 0}},
		Vector:  []float32{1.0, 2.0},
	})
	WriteRecFile(filepath.Join(indexDir, "def456.rec"), IndexRecord{
		Filters: FilterValues{Fields: map[string]int{"gender": 1}},
		Vector:  []float32{3.0, 4.0},
	})
	// Non-.rec file should be ignored
	os.WriteFile(filepath.Join(indexDir, "readme.txt"), []byte("ignore"), 0600)

	records, err := ReadRecDir(indexDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	hashes := map[string]bool{}
	for _, r := range records {
		hashes[r.MatchHash] = true
	}
	if !hashes["abc123"] || !hashes["def456"] {
		t.Errorf("unexpected hashes: %v", hashes)
	}
}

func TestReadRecDir_Empty(t *testing.T) {
	records, err := ReadRecDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0, got %d", len(records))
	}
}

func TestReadRecDir_NotFound(t *testing.T) {
	_, err := ReadRecDir("/nonexistent/dir")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadRecDir_FiltersPreserved(t *testing.T) {
	dir := t.TempDir()
	WriteRecFile(filepath.Join(dir, "hash1.rec"), IndexRecord{
		Filters: FilterValues{Fields: map[string]int{"gender": 1, "gender_target": 5, "age": 28, "intent": 1}},
		Vector:  []float32{1.0},
	})
	records, _ := ReadRecDir(dir)
	if len(records) != 1 {
		t.Fatal("expected 1 record")
	}
	f := records[0].Record.Filters.Fields
	if f["gender"] != 1 || f["gender_target"] != 5 || f["age"] != 28 || f["intent"] != 1 {
		t.Errorf("filters not preserved: %v", f)
	}
}
