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

func TestPartitionConfig_IsRange(t *testing.T) {
	if (PartitionConfig{Field: "role"}).IsRange() {
		t.Error("role should not be a range partition")
	}
	if !(PartitionConfig{Field: "age", Step: 5}).IsRange() {
		t.Error("age with step should be a range partition")
	}
}
