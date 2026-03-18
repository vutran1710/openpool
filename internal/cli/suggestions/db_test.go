package suggestions

import (
	"os"
	"path/filepath"
	"testing"

	gh "github.com/vutran1710/dating-dev/internal/github"
)

func TestPack_UpsertAndFind(t *testing.T) {
	p := &Pack{}
	p.Upsert(Record{MatchHash: "hash1", Vector: []float32{1.0, 2.0}})
	p.Upsert(Record{MatchHash: "hash2", Vector: []float32{3.0, 4.0}})
	if len(p.Records) != 2 {
		t.Fatalf("got %d records, want 2", len(p.Records))
	}
	r := p.Find("hash1")
	if r == nil || r.Vector[0] != 1.0 {
		t.Error("Find failed")
	}
}

func TestPack_Upsert_Replaces(t *testing.T) {
	p := &Pack{}
	p.Upsert(Record{MatchHash: "hash1", Vector: []float32{1.0}})
	p.Upsert(Record{MatchHash: "hash1", Vector: []float32{9.0}})
	if len(p.Records) != 1 {
		t.Fatalf("got %d, want 1", len(p.Records))
	}
	if p.Records[0].Vector[0] != 9.0 {
		t.Errorf("vec = %f, want 9.0", p.Records[0].Vector[0])
	}
}

func TestPack_Find_NotFound(t *testing.T) {
	p := &Pack{}
	if p.Find("nope") != nil {
		t.Error("expected nil")
	}
}

func TestPack_SaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pack")

	p := &Pack{}
	p.Upsert(Record{
		MatchHash: "hash1",
		Filters:   gh.FilterValues{Fields: map[string]int{"gender": 1, "age": 28}},
		Vector:    []float32{1.0, 2.0, 3.0},
	})
	p.Upsert(Record{
		MatchHash: "hash2",
		Vector:    []float32{4.0, 5.0},
	})

	if err := p.Save(path); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Records) != 2 {
		t.Fatalf("loaded %d records, want 2", len(loaded.Records))
	}

	r := loaded.Find("hash1")
	if r == nil {
		t.Fatal("hash1 not found")
	}
	if r.Filters.Fields["gender"] != 1 {
		t.Errorf("gender = %d, want 1", r.Filters.Fields["gender"])
	}
	if r.Vector[0] != 1.0 {
		t.Errorf("vec[0] = %f, want 1.0", r.Vector[0])
	}
}

func TestPack_Load_NotFound(t *testing.T) {
	p, err := Load("/nonexistent/path.pack")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Records) != 0 {
		t.Error("expected empty pack")
	}
}

func TestPack_SyncFromRecDir(t *testing.T) {
	dir := t.TempDir()
	indexDir := filepath.Join(dir, "index")
	os.MkdirAll(indexDir, 0700)

	gh.WriteRecFile(filepath.Join(indexDir, "aaa111.rec"), gh.IndexRecord{
		Filters: gh.FilterValues{Fields: map[string]int{"gender": 0}},
		Vector:  []float32{1.0, 2.0},
	})
	gh.WriteRecFile(filepath.Join(indexDir, "bbb222.rec"), gh.IndexRecord{
		Filters: gh.FilterValues{Fields: map[string]int{"gender": 1}},
		Vector:  []float32{3.0, 4.0},
	})

	p := &Pack{}
	added, err := p.SyncFromRecDir(indexDir)
	if err != nil {
		t.Fatal(err)
	}
	if added != 2 {
		t.Errorf("added = %d, want 2", added)
	}
	if len(p.Records) != 2 {
		t.Errorf("records = %d, want 2", len(p.Records))
	}
	// Check filters came through
	r := p.Find("aaa111")
	if r == nil || r.Filters.Fields["gender"] != 0 {
		t.Error("filters not preserved")
	}
}

func TestPack_SyncFromRecDir_Incremental(t *testing.T) {
	dir := t.TempDir()
	indexDir := filepath.Join(dir, "index")
	os.MkdirAll(indexDir, 0700)

	p := &Pack{}
	gh.WriteRecFile(filepath.Join(indexDir, "aaa111.rec"), gh.IndexRecord{Vector: []float32{1.0}})
	p.SyncFromRecDir(indexDir)

	gh.WriteRecFile(filepath.Join(indexDir, "bbb222.rec"), gh.IndexRecord{Vector: []float32{2.0}})
	added, _ := p.SyncFromRecDir(indexDir)
	if added != 1 {
		t.Errorf("incremental added = %d, want 1", added)
	}
}

func TestPack_SyncFromRecDir_EmptyDir(t *testing.T) {
	p := &Pack{}
	added, err := p.SyncFromRecDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if added != 0 {
		t.Errorf("added = %d, want 0", added)
	}
}
