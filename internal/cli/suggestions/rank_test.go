package suggestions

import (
	"math"
	"testing"
)

func TestCosine_Identical(t *testing.T) {
	a := []float32{1, 0, 1}
	s := cosine(a, a)
	if math.Abs(s-1.0) > 0.001 {
		t.Errorf("cosine = %f, want 1.0", s)
	}
}

func TestCosine_Orthogonal(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{0, 1}
	s := cosine(a, b)
	if math.Abs(s) > 0.001 {
		t.Errorf("cosine = %f, want 0.0", s)
	}
}

func TestCosine_Opposite(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{-1, 0}
	s := cosine(a, b)
	if math.Abs(s+1.0) > 0.001 {
		t.Errorf("cosine = %f, want -1.0", s)
	}
}

func TestCosine_ZeroVector(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{1, 2, 3}
	s := cosine(a, b)
	if s != 0 {
		t.Errorf("cosine = %f, want 0", s)
	}
}

func TestCosine_DifferentLengths(t *testing.T) {
	a := []float32{1, 2}
	b := []float32{1, 2, 3}
	s := cosine(a, b)
	// Should handle gracefully — only compare up to shorter length
	_ = s // no crash = pass
}

func TestRankSuggestions_Order(t *testing.T) {
	me := []float32{1, 0, 1, 0}
	records := []Record{
		{MatchHash: "far", Vector: []float32{0, 1, 0, 1}},
		{MatchHash: "close", Vector: []float32{1, 0, 1, 0}},
		{MatchHash: "mid", Vector: []float32{1, 0, 0, 1}},
	}
	ranked := RankSuggestions(me, "my_hash", records, 10)
	if len(ranked) != 3 {
		t.Fatalf("got %d results, want 3", len(ranked))
	}
	// "close" should be first (identical vector)
	if ranked[0].MatchHash != "close" {
		t.Errorf("first = %q, want 'close'", ranked[0].MatchHash)
	}
	if ranked[0].Score < 0.99 {
		t.Errorf("close score = %f, want ~1.0", ranked[0].Score)
	}
}

func TestRankSuggestions_ExcludesSelf(t *testing.T) {
	me := []float32{1, 0}
	records := []Record{
		{MatchHash: "me", Vector: []float32{1, 0}},
		{MatchHash: "other", Vector: []float32{0, 1}},
	}
	ranked := RankSuggestions(me, "me", records, 10)
	if len(ranked) != 1 {
		t.Fatalf("got %d results, want 1 (self excluded)", len(ranked))
	}
	if ranked[0].MatchHash != "other" {
		t.Errorf("expected 'other', got %q", ranked[0].MatchHash)
	}
}

func TestRankSuggestions_Limit(t *testing.T) {
	me := []float32{1, 0}
	records := []Record{
		{MatchHash: "a", Vector: []float32{1, 0}},
		{MatchHash: "b", Vector: []float32{0, 1}},
		{MatchHash: "c", Vector: []float32{1, 1}},
	}
	ranked := RankSuggestions(me, "none", records, 2)
	if len(ranked) != 2 {
		t.Fatalf("got %d results, want 2", len(ranked))
	}
}

func TestRankSuggestions_Empty(t *testing.T) {
	me := []float32{1, 0}
	ranked := RankSuggestions(me, "me", nil, 10)
	if len(ranked) != 0 {
		t.Errorf("expected 0, got %d", len(ranked))
	}
}

func TestRankSuggestions_AllSelf(t *testing.T) {
	me := []float32{1, 0}
	records := []Record{
		{MatchHash: "me", Vector: []float32{1, 0}},
	}
	ranked := RankSuggestions(me, "me", records, 10)
	if len(ranked) != 0 {
		t.Errorf("expected 0 (only self), got %d", len(ranked))
	}
}
