package explorer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPreferences_Missing(t *testing.T) {
	p := LoadPreferences("/nonexistent/path")
	if len(p.LookingFor) != 0 {
		t.Error("missing file should return empty preferences")
	}
}

func TestLoadPreferences_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "preferences.yaml")
	os.WriteFile(path, []byte(`looking_for:
  role: woman
  age:
    min: 25
    max: 35
`), 0600)

	p := LoadPreferences(path)
	if p.LookingFor["role"] != "woman" {
		t.Errorf("expected role=woman, got %v", p.LookingFor["role"])
	}
}

func TestRankBuckets_NoPrefs(t *testing.T) {
	buckets := []BucketInfo{
		{ID: "a", ProfileCount: 5},
		{ID: "b", ProfileCount: 10},
		{ID: "c", ProfileCount: 3},
	}

	ranked := RankBuckets(buckets, Preferences{})
	if ranked[0].ID != "b" {
		t.Errorf("largest bucket should be first, got %s", ranked[0].ID)
	}
}

func TestRankBuckets_WithPrefs(t *testing.T) {
	buckets := []BucketInfo{
		{ID: "role:man", Partitions: map[string]string{"role": "man"}, ProfileCount: 10},
		{ID: "role:woman", Partitions: map[string]string{"role": "woman"}, ProfileCount: 5},
	}

	prefs := Preferences{LookingFor: map[string]any{"role": "woman"}}
	ranked := RankBuckets(buckets, prefs)

	if ranked[0].ID != "role:woman" {
		t.Errorf("woman bucket should be first, got %s", ranked[0].ID)
	}
}

func TestRankBuckets_RangePrefs(t *testing.T) {
	buckets := []BucketInfo{
		{ID: "age:18-22", Partitions: map[string]string{"age": "18-22"}, ProfileCount: 5},
		{ID: "age:23-27", Partitions: map[string]string{"age": "23-27"}, ProfileCount: 5},
		{ID: "age:28-32", Partitions: map[string]string{"age": "28-32"}, ProfileCount: 5},
	}

	prefs := Preferences{LookingFor: map[string]any{
		"age": map[string]any{"min": 25, "max": 30},
	}}
	ranked := RankBuckets(buckets, prefs)

	// Both 23-27 and 28-32 overlap with 25-30, 18-22 doesn't
	if ranked[len(ranked)-1].ID != "age:18-22" {
		t.Errorf("18-22 should rank last, got %s", ranked[len(ranked)-1].ID)
	}
}

func TestMatchesPref_String(t *testing.T) {
	if !matchesPref("woman", "woman") {
		t.Error("exact string should match")
	}
	if matchesPref("man", "woman") {
		t.Error("different string should not match")
	}
}

func TestMatchesPref_Range(t *testing.T) {
	// Bucket 25-30, preference min=20 max=28 → overlaps
	if !matchesPref("25-30", map[string]any{"min": 20, "max": 28}) {
		t.Error("overlapping range should match")
	}
	// Bucket 25-30, preference min=31 max=40 → no overlap
	if matchesPref("25-30", map[string]any{"min": 31, "max": 40}) {
		t.Error("non-overlapping range should not match")
	}
}
