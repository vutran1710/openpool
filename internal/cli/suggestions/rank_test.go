package suggestions

import (
	"fmt"
	"math"
	"testing"

	gh "github.com/vutran1710/dating-dev/internal/github"
)

func TestCosine_Identical(t *testing.T) {
	a := []float32{1, 0, 1}
	s := cosine(a, a)
	if math.Abs(s-1.0) > 0.001 {
		t.Errorf("cosine = %f, want 1.0", s)
	}
}

func TestCosine_Orthogonal(t *testing.T) {
	s := cosine([]float32{1, 0}, []float32{0, 1})
	if math.Abs(s) > 0.001 {
		t.Errorf("cosine = %f, want 0.0", s)
	}
}

func TestCosine_Opposite(t *testing.T) {
	s := cosine([]float32{1, 0}, []float32{-1, 0})
	if math.Abs(s+1.0) > 0.001 {
		t.Errorf("cosine = %f, want -1.0", s)
	}
}

func TestCosine_ZeroVector(t *testing.T) {
	if cosine([]float32{0, 0}, []float32{1, 2}) != 0 {
		t.Error("expected 0 for zero vector")
	}
}

func datingSchema() *gh.PoolSchema {
	return &gh.PoolSchema{Fields: []gh.SchemaField{
		{Name: "gender", Type: "enum", Match: "complementary", Target: "gender_target", Values: []string{"male", "female", "nb"}},
		{Name: "gender_target", Type: "multi", Values: []string{"male", "female", "nb"}},
		{Name: "age", Type: "range", Match: "approximate", Tolerance: 5},
		{Name: "intent", Type: "multi", Match: "exact", Values: []string{"dating", "friends"}},
		{Name: "interests", Type: "multi", Match: "similarity", Values: []string{"hiking", "coding", "music"}},
	}}
}

func rec(hash string, gender, genderTarget, age, intent int, vec []float32) Record {
	return Record{
		MatchHash: hash,
		Filters:   gh.FilterValues{Fields: map[string]int{"gender": gender, "gender_target": genderTarget, "age": age, "intent": intent}},
		Vector:    vec,
	}
}

func TestRankSuggestions_WithFilters(t *testing.T) {
	schema := datingSchema()
	alice := rec("alice", 1, 1, 28, 1, []float32{1, 0, 1})
	records := []Record{
		rec("bob", 0, 2, 32, 1, []float32{0, 1, 0}),
		rec("charlie", 0, 6, 25, 2, []float32{1, 1, 0}),
		rec("hank", 0, 2, 29, 1, []float32{1, 0, 1}),
	}
	seen := map[string]bool{}
	ranked := RankSuggestions(schema, alice, records, seen, 10)

	if len(ranked) != 2 {
		t.Fatalf("got %d results, want 2 (charlie filtered)", len(ranked))
	}
	if ranked[0].MatchHash != "hank" {
		t.Errorf("first = %q, want hank", ranked[0].MatchHash)
	}
}

func TestRankSuggestions_NoSchema(t *testing.T) {
	me := Record{MatchHash: "me", Vector: []float32{1, 0}}
	records := []Record{
		{MatchHash: "other", Vector: []float32{1, 0}},
	}
	ranked := RankSuggestions(nil, me, records, nil, 10)
	if len(ranked) != 1 {
		t.Fatalf("got %d, want 1", len(ranked))
	}
}

func TestRankSuggestions_ExcludesSelf(t *testing.T) {
	me := Record{MatchHash: "me", Vector: []float32{1, 0}}
	records := []Record{
		{MatchHash: "me", Vector: []float32{1, 0}},
		{MatchHash: "other", Vector: []float32{0, 1}},
	}
	ranked := RankSuggestions(nil, me, records, nil, 10)
	if len(ranked) != 1 {
		t.Fatalf("got %d, want 1", len(ranked))
	}
}

func TestRankSuggestions_Limit(t *testing.T) {
	me := Record{MatchHash: "me", Vector: []float32{1, 0}}
	records := []Record{
		{MatchHash: "a", Vector: []float32{1, 0}},
		{MatchHash: "b", Vector: []float32{0, 1}},
		{MatchHash: "c", Vector: []float32{1, 1}},
	}
	ranked := RankSuggestions(nil, me, records, nil, 2)
	if len(ranked) != 2 {
		t.Fatalf("got %d, want 2", len(ranked))
	}
}

func TestRankSuggestions_Empty(t *testing.T) {
	me := Record{MatchHash: "me", Vector: []float32{1, 0}}
	ranked := RankSuggestions(nil, me, nil, nil, 10)
	if len(ranked) != 0 {
		t.Errorf("expected 0, got %d", len(ranked))
	}
}

func TestRankSuggestions_AllFiltered(t *testing.T) {
	schema := datingSchema()
	alice := rec("alice", 1, 1, 28, 1, []float32{1, 0, 0})
	records := []Record{
		rec("dana", 1, 3, 30, 3, []float32{0, 0, 1}),
	}
	ranked := RankSuggestions(schema, alice, records, nil, 10)
	if len(ranked) != 0 {
		t.Errorf("expected 0, got %d", len(ranked))
	}
}

// --- Seen tracking ---

func TestRankSuggestions_ExcludesSeen(t *testing.T) {
	me := Record{MatchHash: "me", Vector: []float32{1, 0}}
	records := []Record{
		{MatchHash: "a", Vector: []float32{1, 0}},
		{MatchHash: "b", Vector: []float32{0, 1}},
		{MatchHash: "c", Vector: []float32{1, 1}},
	}
	seen := map[string]bool{"a": true}
	ranked := RankSuggestions(nil, me, records, seen, 10)
	if len(ranked) != 2 {
		t.Fatalf("got %d, want 2 (a excluded)", len(ranked))
	}
	for _, s := range ranked {
		if s.MatchHash == "a" {
			t.Error("seen 'a' should be excluded")
		}
	}
}

func TestRankSuggestions_AllSeen(t *testing.T) {
	me := Record{MatchHash: "me", Vector: []float32{1, 0}}
	records := []Record{
		{MatchHash: "a", Vector: []float32{1, 0}},
		{MatchHash: "b", Vector: []float32{0, 1}},
	}
	seen := map[string]bool{"a": true, "b": true}
	ranked := RankSuggestions(nil, me, records, seen, 10)
	if len(ranked) != 0 {
		t.Errorf("expected 0, got %d", len(ranked))
	}
}

// --- Tier ordering ---

func TestRankSuggestions_TierOrdering(t *testing.T) {
	// Create 100 records with varying similarity
	me := Record{MatchHash: "me", Vector: []float32{1, 0, 0, 0, 0}}
	var records []Record
	for i := 0; i < 100; i++ {
		vec := make([]float32, 5)
		// Higher index = less similar
		if i < 5 {
			vec[0] = 1 // top 5% — identical to me
		} else if i < 20 {
			vec[0] = 0.8
			vec[1] = 0.2
		} else if i < 50 {
			vec[0] = 0.5
			vec[2] = 0.5
		} else {
			vec[3] = 1 // bottom 50% — orthogonal
		}
		records = append(records, Record{
			MatchHash: fmt.Sprintf("user_%03d", i),
			Vector:    vec,
		})
	}

	ranked := RankSuggestions(nil, me, records, nil, 100)

	// Top results should come from tier 1 (top 5%)
	// They should have high scores
	if ranked[0].Score < 0.5 {
		t.Errorf("first suggestion score = %f, expected high (from top tier)", ranked[0].Score)
	}

	// Last results should have low scores
	last := ranked[len(ranked)-1]
	if last.Score > 0.5 {
		t.Errorf("last suggestion score = %f, expected low", last.Score)
	}
}
