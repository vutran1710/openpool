package github

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteVecFile_Size(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.vec")
	vec := []float32{1.0, 2.0, 3.0}
	if err := WriteVecFile(path, vec); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Size() != 12 { // 3 × 4 bytes
		t.Errorf("size = %d, want 12", info.Size())
	}
}

func TestReadVecFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.vec")
	want := []float32{1.5, -2.0, 0.0, 3.14}
	WriteVecFile(path, want)
	got, err := ReadVecFile(path)
	if err != nil {
		t.Fatal(err)
	}
	assertVec(t, got, want)
}

func TestReadVecFile_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.vec")
	os.WriteFile(path, []byte{}, 0600)
	got, err := ReadVecFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %d", len(got))
	}
}

func TestReadVecFile_NotFound(t *testing.T) {
	_, err := ReadVecFile("/nonexistent/path.vec")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadVecDir(t *testing.T) {
	dir := t.TempDir()
	indexDir := filepath.Join(dir, "index")
	os.MkdirAll(indexDir, 0700)

	WriteVecFile(filepath.Join(indexDir, "abc123.vec"), []float32{1.0, 2.0})
	WriteVecFile(filepath.Join(indexDir, "def456.vec"), []float32{3.0, 4.0})

	// Non-.vec file should be ignored
	os.WriteFile(filepath.Join(indexDir, "readme.txt"), []byte("ignore"), 0600)

	records, err := ReadVecDir(indexDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}

	// Check match_hashes extracted from filenames
	hashes := map[string]bool{}
	for _, r := range records {
		hashes[r.MatchHash] = true
	}
	if !hashes["abc123"] || !hashes["def456"] {
		t.Errorf("unexpected hashes: %v", hashes)
	}
}

func TestReadVecDir_Empty(t *testing.T) {
	dir := t.TempDir()
	records, err := ReadVecDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestReadVecDir_NotFound(t *testing.T) {
	_, err := ReadVecDir("/nonexistent/dir")
	if err == nil {
		t.Fatal("expected error")
	}
}
