package bucket

import (
	"testing"
)

func TestBucketID(t *testing.T) {
	// Single partition
	id := BucketID(map[string]string{"role": "woman"})
	if id != "role:woman" {
		t.Errorf("expected role:woman, got %s", id)
	}

	// Multiple partitions (sorted by key)
	id = BucketID(map[string]string{"role": "woman", "age": "25-30"})
	if id != "age:25-30|role:woman" {
		t.Errorf("expected age:25-30|role:woman, got %s", id)
	}

	// Deterministic
	id2 := BucketID(map[string]string{"age": "25-30", "role": "woman"})
	if id != id2 {
		t.Error("should be deterministic regardless of map order")
	}
}

func TestRangeBuckets(t *testing.T) {
	buckets := RangeBuckets(18, 100, 5, 2)

	if buckets[0].Min != 18 {
		t.Errorf("first bucket min should be 18, got %d", buckets[0].Min)
	}

	if len(buckets) < 2 {
		t.Fatal("should have multiple buckets")
	}

	// With overlap=2, adjacent buckets should share range
	if buckets[1].Min >= buckets[0].Max {
		t.Error("buckets should overlap")
	}

	t.Logf("generated %d buckets", len(buckets))
	for _, b := range buckets {
		t.Logf("  %d-%d", b.Min, b.Max)
	}
}

func TestRangeBuckets_NoOverlap(t *testing.T) {
	buckets := RangeBuckets(18, 100, 10, 0)
	for i := 1; i < len(buckets); i++ {
		if buckets[i].Min != buckets[i-1].Max+1 {
			t.Errorf("bucket %d should start at %d, got %d", i, buckets[i-1].Max+1, buckets[i].Min)
		}
	}
}

func TestRangeBucket_Contains(t *testing.T) {
	rb := RangeBucket{Min: 25, Max: 30}
	if !rb.Contains(25) {
		t.Error("should contain min")
	}
	if !rb.Contains(30) {
		t.Error("should contain max")
	}
	if !rb.Contains(27) {
		t.Error("should contain mid")
	}
	if rb.Contains(24) {
		t.Error("should not contain below min")
	}
	if rb.Contains(31) {
		t.Error("should not contain above max")
	}
}

func TestRangeBucket_Label(t *testing.T) {
	rb := RangeBucket{Min: 18, Max: 22}
	if rb.Label() != "18-22" {
		t.Errorf("expected 18-22, got %s", rb.Label())
	}
}

// === Assign tests ===

func TestAssign_SinglePartition(t *testing.T) {
	config := []PartitionConfig{{Field: "role"}}

	profiles := []Profile{
		{Tag: "alice", Attributes: map[string]any{"role": "woman", "age": 27}},
		{Tag: "bob", Attributes: map[string]any{"role": "man", "age": 28}},
		{Tag: "carol", Attributes: map[string]any{"role": "woman", "age": 26}},
	}

	buckets := Assign(profiles, config, nil)

	if len(buckets) != 2 {
		t.Fatalf("expected 2 buckets (man, woman), got %d", len(buckets))
	}

	var women *Bucket
	for i := range buckets {
		if buckets[i].PartitionValues["role"] == "woman" {
			women = &buckets[i]
			break
		}
	}
	if women == nil {
		t.Fatal("missing women bucket")
	}
	if len(women.Tags) != 2 {
		t.Errorf("expected 2 women, got %d", len(women.Tags))
	}
}

func TestAssign_RangePartition(t *testing.T) {
	config := []PartitionConfig{
		{Field: "age", Step: 5, Overlap: 2},
	}

	fieldRanges := map[string][2]int{
		"age": {18, 40},
	}

	profiles := []Profile{
		{Tag: "a", Attributes: map[string]any{"age": 20}},
		{Tag: "b", Attributes: map[string]any{"age": 24}},
		{Tag: "c", Attributes: map[string]any{"age": 30}},
	}

	buckets := Assign(profiles, config, fieldRanges)

	// Profile with age=24 should appear in multiple buckets due to overlap
	count := 0
	for _, b := range buckets {
		for _, tag := range b.Tags {
			if tag == "b" {
				count++
			}
		}
	}
	if count < 2 {
		t.Errorf("age=24 should appear in multiple overlapping buckets, appeared in %d", count)
	}
}

func TestAssign_MultiPartition(t *testing.T) {
	config := []PartitionConfig{
		{Field: "role"},
		{Field: "age", Step: 10, Overlap: 0},
	}

	fieldRanges := map[string][2]int{
		"age": {18, 40},
	}

	profiles := []Profile{
		{Tag: "a", Attributes: map[string]any{"role": "woman", "age": 25}},
		{Tag: "b", Attributes: map[string]any{"role": "man", "age": 25}},
		{Tag: "c", Attributes: map[string]any{"role": "woman", "age": 35}},
	}

	buckets := Assign(profiles, config, fieldRanges)

	for _, b := range buckets {
		t.Logf("bucket %s: %v", b.ID, b.Tags)
	}

	if len(buckets) < 3 {
		t.Errorf("expected at least 3 buckets, got %d", len(buckets))
	}
}

func TestAssign_MultiValueField(t *testing.T) {
	config := []PartitionConfig{
		{Field: "skills"},
	}

	profiles := []Profile{
		{Tag: "a", Attributes: map[string]any{"skills": []any{"go", "python"}}},
		{Tag: "b", Attributes: map[string]any{"skills": []any{"go", "rust"}}},
		{Tag: "c", Attributes: map[string]any{"skills": []any{"python"}}},
	}

	buckets := Assign(profiles, config, nil)

	goCount := 0
	for _, b := range buckets {
		if b.PartitionValues["skills"] == "go" {
			goCount = len(b.Tags)
		}
	}
	if goCount != 2 {
		t.Errorf("go bucket should have 2 profiles, got %d", goCount)
	}
}

func TestAssign_Empty(t *testing.T) {
	config := []PartitionConfig{{Field: "role"}}
	buckets := Assign(nil, config, nil)
	if len(buckets) != 0 {
		t.Error("empty profiles should produce no buckets")
	}
}

func TestPartitionConfig_IsRange(t *testing.T) {
	if (PartitionConfig{Field: "role"}).IsRange() {
		t.Error("role should not be a range partition")
	}
	if !(PartitionConfig{Field: "age", Step: 5}).IsRange() {
		t.Error("age with step should be a range partition")
	}
}
