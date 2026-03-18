package suggestions

import (
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
	alice := rec("alice", 1, 1, 28, 1, []float32{1, 0, 1}) // F→M, dating, hiking+music
	records := []Record{
		rec("bob", 0, 2, 32, 1, []float32{0, 1, 0}),      // M→F, dating, coding
		rec("charlie", 0, 6, 25, 2, []float32{1, 1, 0}),   // M→F+NB, friends
		rec("hank", 0, 2, 29, 1, []float32{1, 0, 1}),      // M→F, dating, hiking+music
	}
	ranked := RankSuggestions(schema, alice, records, 10)

	// Charlie filtered out (intent mismatch: dating & friends = 0)
	// Bob + Hank pass
	if len(ranked) != 2 {
		t.Fatalf("got %d results, want 2 (charlie filtered)", len(ranked))
	}
	// Hank should rank higher (identical interests)
	if ranked[0].MatchHash != "hank" {
		t.Errorf("first = %q, want hank", ranked[0].MatchHash)
	}
}

func TestRankSuggestions_NoSchema(t *testing.T) {
	me := Record{MatchHash: "me", Vector: []float32{1, 0}}
	records := []Record{
		{MatchHash: "other", Vector: []float32{1, 0}},
	}
	ranked := RankSuggestions(nil, me, records, 10)
	if len(ranked) != 1 {
		t.Fatalf("got %d, want 1 (no schema = no filter)", len(ranked))
	}
}

func TestRankSuggestions_ExcludesSelf(t *testing.T) {
	me := Record{MatchHash: "me", Vector: []float32{1, 0}}
	records := []Record{
		{MatchHash: "me", Vector: []float32{1, 0}},
		{MatchHash: "other", Vector: []float32{0, 1}},
	}
	ranked := RankSuggestions(nil, me, records, 10)
	if len(ranked) != 1 {
		t.Fatalf("got %d, want 1 (self excluded)", len(ranked))
	}
}

func TestRankSuggestions_Limit(t *testing.T) {
	me := Record{MatchHash: "me", Vector: []float32{1, 0}}
	records := []Record{
		{MatchHash: "a", Vector: []float32{1, 0}},
		{MatchHash: "b", Vector: []float32{0, 1}},
		{MatchHash: "c", Vector: []float32{1, 1}},
	}
	ranked := RankSuggestions(nil, me, records, 2)
	if len(ranked) != 2 {
		t.Fatalf("got %d, want 2", len(ranked))
	}
}

func TestRankSuggestions_Empty(t *testing.T) {
	me := Record{MatchHash: "me", Vector: []float32{1, 0}}
	ranked := RankSuggestions(nil, me, nil, 10)
	if len(ranked) != 0 {
		t.Errorf("expected 0, got %d", len(ranked))
	}
}

func TestRankSuggestions_AllFiltered(t *testing.T) {
	schema := datingSchema()
	alice := rec("alice", 1, 1, 28, 1, []float32{1, 0, 0}) // F→M, dating
	records := []Record{
		rec("dana", 1, 3, 30, 3, []float32{0, 0, 1}), // F→M+F — Alice doesn't target F
	}
	ranked := RankSuggestions(schema, alice, records, 10)
	if len(ranked) != 0 {
		t.Errorf("expected 0 (all filtered), got %d", len(ranked))
	}
}
