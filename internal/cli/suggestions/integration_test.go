package suggestions

import (
	"os"
	"path/filepath"
	"testing"

	gh "github.com/vutran1710/dating-dev/internal/github"
)

// TestFullDiscoverFlow tests: write .rec files → sync into pack → rank with filters.
func TestFullDiscoverFlow(t *testing.T) {
	dir := t.TempDir()
	indexDir := filepath.Join(dir, "index")
	os.MkdirAll(indexDir, 0700)

	schema := &gh.PoolSchema{Fields: []gh.SchemaField{
		{Name: "gender", Type: "enum", Match: "complementary", Target: "gender_target", Values: []string{"male", "female", "nb"}},
		{Name: "gender_target", Type: "multi", Values: []string{"male", "female", "nb"}},
		{Name: "age", Type: "range", Match: "approximate", Tolerance: 5},
		{Name: "intent", Type: "multi", Match: "exact", Values: []string{"dating", "friends"}},
		{Name: "interests", Type: "multi", Match: "similarity", Values: []string{"hiking", "coding", "music"}},
	}}

	// Alice: F→M, 28, dating, hiking+music
	aliceProfile := map[string]any{
		"gender": "female", "gender_target": []any{"male"},
		"age": float64(28), "intent": []any{"dating"},
		"interests": []any{"hiking", "music"},
	}
	aliceFilters := gh.ExtractFilters(schema, aliceProfile)
	aliceVec := gh.EncodeProfile(schema, aliceProfile)
	gh.WriteRecFile(filepath.Join(indexDir, "alice_hash.rec"), gh.IndexRecord{
		Filters: aliceFilters, Vector: aliceVec,
	})

	// Bob: M→F, 30, dating, coding
	bobProfile := map[string]any{
		"gender": "male", "gender_target": []any{"female"},
		"age": float64(30), "intent": []any{"dating"},
		"interests": []any{"coding"},
	}
	bobFilters := gh.ExtractFilters(schema, bobProfile)
	bobVec := gh.EncodeProfile(schema, bobProfile)
	gh.WriteRecFile(filepath.Join(indexDir, "bob_hash.rec"), gh.IndexRecord{
		Filters: bobFilters, Vector: bobVec,
	})

	// Charlie: M→F, 25, friends, hiking+coding
	charlieProfile := map[string]any{
		"gender": "male", "gender_target": []any{"female", "nb"},
		"age": float64(25), "intent": []any{"friends"},
		"interests": []any{"hiking", "coding"},
	}
	charlieFilters := gh.ExtractFilters(schema, charlieProfile)
	charlieVec := gh.EncodeProfile(schema, charlieProfile)
	gh.WriteRecFile(filepath.Join(indexDir, "charlie_hash.rec"), gh.IndexRecord{
		Filters: charlieFilters, Vector: charlieVec,
	})

	// Dana: F→M+F, 30, dating+friends, music
	danaProfile := map[string]any{
		"gender": "female", "gender_target": []any{"male", "female"},
		"age": float64(30), "intent": []any{"dating", "friends"},
		"interests": []any{"music"},
	}
	danaFilters := gh.ExtractFilters(schema, danaProfile)
	danaVec := gh.EncodeProfile(schema, danaProfile)
	gh.WriteRecFile(filepath.Join(indexDir, "dana_hash.rec"), gh.IndexRecord{
		Filters: danaFilters, Vector: danaVec,
	})

	// Hank: M→F, 29, dating, hiking+music
	hankProfile := map[string]any{
		"gender": "male", "gender_target": []any{"female"},
		"age": float64(29), "intent": []any{"dating"},
		"interests": []any{"hiking", "music"},
	}
	hankFilters := gh.ExtractFilters(schema, hankProfile)
	hankVec := gh.EncodeProfile(schema, hankProfile)
	gh.WriteRecFile(filepath.Join(indexDir, "hank_hash.rec"), gh.IndexRecord{
		Filters: hankFilters, Vector: hankVec,
	})

	// Step 1: Sync .rec files into pack
	packPath := filepath.Join(dir, "suggestions.pack")
	pack, err := Load(packPath)
	if err != nil {
		t.Fatal(err)
	}
	added, err := pack.SyncFromRecDir(indexDir)
	if err != nil {
		t.Fatal(err)
	}
	if added != 5 {
		t.Fatalf("added = %d, want 5", added)
	}
	if err := pack.Save(packPath); err != nil {
		t.Fatal(err)
	}

	// Step 2: Reload and verify
	pack, err = Load(packPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Records) != 5 {
		t.Fatalf("records = %d, want 5", len(pack.Records))
	}

	// Step 3: Rank from Alice's perspective
	me := pack.Find("alice_hash")
	if me == nil {
		t.Fatal("alice not found")
	}

	ranked := RankSuggestions(schema, *me, pack.Records, nil, 10)

	// Alice (F→M, dating, 28):
	// - Bob: M→F, dating, 30 → PASS (gender OK, age |28-30|=2, intent exact match)
	// - Charlie: M→F+NB, friends, 25 → FAIL (intent: dating & friends = 0)
	// - Dana: F→M+F, dating+friends, 30 → FAIL (Alice doesn't target F)
	// - Hank: M→F, dating, 29 → PASS (gender OK, age |28-29|=1, intent match)
	if len(ranked) != 2 {
		t.Fatalf("ranked = %d, want 2 (Bob + Hank pass filter)", len(ranked))
	}

	// Hank should rank higher (identical interests: hiking+music)
	// Bob has no interest overlap with Alice
	hankFound := false
	bobFound := false
	for _, s := range ranked {
		if s.MatchHash == "hank_hash" {
			hankFound = true
			if s.Score < 0.9 {
				t.Errorf("Hank score = %f, expected ~1.0 (identical interests)", s.Score)
			}
		}
		if s.MatchHash == "bob_hash" {
			bobFound = true
		}
	}
	if !hankFound {
		t.Error("Hank not in results")
	}
	if !bobFound {
		t.Error("Bob not in results")
	}

	// Verify Hank ranks above Bob
	if ranked[0].MatchHash != "hank_hash" {
		t.Errorf("first = %q, want hank_hash", ranked[0].MatchHash)
	}
}

// TestFullDiscoverFlow_NoSchema tests discover without schema (no filters).
func TestFullDiscoverFlow_NoSchema(t *testing.T) {
	dir := t.TempDir()
	indexDir := filepath.Join(dir, "index")
	os.MkdirAll(indexDir, 0700)

	gh.WriteRecFile(filepath.Join(indexDir, "a.rec"), gh.IndexRecord{Vector: []float32{1, 0, 1}})
	gh.WriteRecFile(filepath.Join(indexDir, "b.rec"), gh.IndexRecord{Vector: []float32{1, 0, 0}})
	gh.WriteRecFile(filepath.Join(indexDir, "c.rec"), gh.IndexRecord{Vector: []float32{0, 1, 0}})

	pack := &Pack{}
	pack.SyncFromRecDir(indexDir)

	me := pack.Find("a")
	ranked := RankSuggestions(nil, *me, pack.Records, nil, 10)
	// No filter → all 2 others pass
	if len(ranked) != 2 {
		t.Fatalf("ranked = %d, want 2", len(ranked))
	}
	// b is closer to a than c
	if ranked[0].MatchHash != "b" {
		t.Errorf("first = %q, want b", ranked[0].MatchHash)
	}
}

// TestFullDiscoverFlow_AllFiltered tests when nobody passes the filter.
func TestFullDiscoverFlow_AllFiltered(t *testing.T) {
	schema := &gh.PoolSchema{Fields: []gh.SchemaField{
		{Name: "gender", Type: "enum", Match: "complementary", Target: "gender_target", Values: []string{"male", "female"}},
		{Name: "gender_target", Type: "multi", Values: []string{"male", "female"}},
		{Name: "interests", Type: "multi", Match: "similarity", Values: []string{"a", "b"}},
	}}

	// All female, all targeting male → nobody matches each other
	pack := &Pack{Records: []Record{
		{MatchHash: "a", Filters: gh.FilterValues{Fields: map[string]int{"gender": 1, "gender_target": 1}}, Vector: []float32{1, 0}},
		{MatchHash: "b", Filters: gh.FilterValues{Fields: map[string]int{"gender": 1, "gender_target": 1}}, Vector: []float32{0, 1}},
		{MatchHash: "c", Filters: gh.FilterValues{Fields: map[string]int{"gender": 1, "gender_target": 1}}, Vector: []float32{1, 1}},
	}}

	me := pack.Find("a")
	ranked := RankSuggestions(schema, *me, pack.Records, nil, 10)
	if len(ranked) != 0 {
		t.Errorf("ranked = %d, want 0 (all filtered — same gender, targeting opposite)", len(ranked))
	}
}

// TestFullDiscoverFlow_IncrementalSync tests adding users after initial sync.
func TestFullDiscoverFlow_IncrementalSync(t *testing.T) {
	dir := t.TempDir()
	indexDir := filepath.Join(dir, "index")
	os.MkdirAll(indexDir, 0700)
	packPath := filepath.Join(dir, "suggestions.pack")

	// Initial: 2 users
	gh.WriteRecFile(filepath.Join(indexDir, "a.rec"), gh.IndexRecord{Vector: []float32{1, 0}})
	gh.WriteRecFile(filepath.Join(indexDir, "b.rec"), gh.IndexRecord{Vector: []float32{0, 1}})

	pack, _ := Load(packPath)
	pack.SyncFromRecDir(indexDir)
	pack.Save(packPath)

	// Add 1 more user
	gh.WriteRecFile(filepath.Join(indexDir, "c.rec"), gh.IndexRecord{Vector: []float32{1, 1}})

	pack, _ = Load(packPath)
	added, _ := pack.SyncFromRecDir(indexDir)
	pack.Save(packPath)

	if added != 1 {
		t.Errorf("added = %d, want 1", added)
	}
	if len(pack.Records) != 3 {
		t.Errorf("total = %d, want 3", len(pack.Records))
	}
}
