# Profile Bucketing — Implementation Plan (2 of 5)

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement profile bucketing — assign profiles to buckets based on pool.yaml partition config.

**Architecture:** New `internal/bucket` package. Pure logic — takes profiles + partition config, returns bucket assignments. No crypto, no database, no IO. Testable in isolation.

**Tech Stack:** Go, `internal/schema` (for pool.yaml parsing)

**Spec:** `docs/superpowers/specs/2026-03-23-matching-engine-design.md` (Pool Configuration + Indexer sections)

---

## File Structure

```
internal/bucket/
  bucket.go       — types, partition config parsing, bucket ID generation
  assign.go       — assign profiles to buckets
  bucket_test.go  — all tests
```

---

### Task 1: Types and bucket ID generation

**Files:**
- Create: `internal/bucket/bucket.go`
- Create: `internal/bucket/bucket_test.go`

- [ ] **Step 1: Write tests**

```go
// internal/bucket/bucket_test.go
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
}

func TestRangeBuckets(t *testing.T) {
	// age step=5, overlap=2, range 18-100
	buckets := RangeBuckets(18, 100, 5, 2)

	// First bucket should start at 18
	if buckets[0].Min != 18 {
		t.Errorf("first bucket min should be 18, got %d", buckets[0].Min)
	}

	// Buckets should overlap
	if len(buckets) < 2 {
		t.Fatal("should have multiple buckets")
	}
	// With overlap=2, bucket 0 and bucket 1 should share some range
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

func TestParsePartitions(t *testing.T) {
	config := []PartitionConfig{
		{Field: "role"},
		{Field: "age", Step: 5, Overlap: 2},
	}

	// role partition: values come from profile data
	if config[0].IsRange() {
		t.Error("role should not be a range partition")
	}

	// age partition: has step → range
	if !config[1].IsRange() {
		t.Error("age should be a range partition")
	}
}
```

- [ ] **Step 2: Run tests — should fail (package doesn't exist)**

- [ ] **Step 3: Implement bucket.go**

```go
// internal/bucket/bucket.go
package bucket

import (
	"fmt"
	"sort"
	"strings"
)

// PartitionConfig defines how to partition profiles on one attribute.
type PartitionConfig struct {
	Field   string `yaml:"field"`
	Step    int    `yaml:"step,omitempty"`    // for range fields: bucket width
	Overlap int    `yaml:"overlap,omitempty"` // for range fields: overlap with adjacent buckets
}

// IsRange returns true if this partition uses range bucketing.
func (p PartitionConfig) IsRange() bool {
	return p.Step > 0
}

// RangeBucket represents a single range bucket (e.g., age 25-30).
type RangeBucket struct {
	Min int
	Max int
}

// Label returns the bucket label (e.g., "25-30").
func (r RangeBucket) Label() string {
	return fmt.Sprintf("%d-%d", r.Min, r.Max)
}

// Contains returns true if value falls within this bucket.
func (r RangeBucket) Contains(value int) bool {
	return value >= r.Min && value <= r.Max
}

// Bucket holds the profiles assigned to one bucket.
type Bucket struct {
	ID              string            // e.g., "role:woman|age:25-30"
	PartitionValues map[string]string // e.g., {"role": "woman", "age": "25-30"}
	Tags            []string          // match_hashes of profiles in this bucket
}

// BucketID generates a deterministic ID from partition values.
// Keys are sorted alphabetically for consistency.
func BucketID(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + ":" + values[k]
	}
	return strings.Join(parts, "|")
}

// RangeBuckets generates range buckets for a field with given min, max, step, and overlap.
func RangeBuckets(fieldMin, fieldMax, step, overlap int) []RangeBucket {
	var buckets []RangeBucket
	start := fieldMin

	for start <= fieldMax {
		end := start + step - 1
		if end > fieldMax {
			end = fieldMax
		}
		buckets = append(buckets, RangeBucket{Min: start, Max: end})

		// Next bucket starts at (step - overlap) from current start
		advance := step - overlap
		if advance < 1 {
			advance = 1
		}
		start += advance
	}

	return buckets
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/bucket/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/bucket/bucket.go internal/bucket/bucket_test.go
git commit -m "feat(bucket): types, bucket ID generation, range bucketing"
```

---

### Task 2: Profile assignment to buckets

**Files:**
- Create: `internal/bucket/assign.go`
- Modify: `internal/bucket/bucket_test.go`

- [ ] **Step 1: Write tests**

```go
// Add to bucket_test.go

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

	// Find women bucket
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

	// Schema needed for range field min/max
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

	// Should have: woman|18-27, woman|28-37, man|18-27
	// (man|28-37 would be empty → not created)
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

	// "go" bucket should have a and b
	// "python" bucket should have a and c
	// "rust" bucket should have b only
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

func TestAssign_EmptyProfiles(t *testing.T) {
	config := []PartitionConfig{{Field: "role"}}
	buckets := Assign(nil, config, nil)
	if len(buckets) != 0 {
		t.Error("empty profiles should produce no buckets")
	}
}
```

- [ ] **Step 2: Run tests — should fail**

- [ ] **Step 3: Implement assign.go**

```go
// internal/bucket/assign.go
package bucket

import (
	"fmt"
)

// Profile is the input to bucket assignment.
type Profile struct {
	Tag        string         // match_hash
	Attributes map[string]any // profile field values
}

// Assign distributes profiles into buckets based on partition config.
// fieldRanges provides min/max for range fields (from schema): map[fieldName][2]int{min, max}.
func Assign(profiles []Profile, partitions []PartitionConfig, fieldRanges map[string][2]int) []Bucket {
	if len(profiles) == 0 || len(partitions) == 0 {
		return nil
	}

	// Pre-compute range buckets for range partitions
	rangeBuckets := make(map[string][]RangeBucket)
	for _, p := range partitions {
		if p.IsRange() {
			r, ok := fieldRanges[p.Field]
			if !ok {
				continue
			}
			rangeBuckets[p.Field] = RangeBuckets(r[0], r[1], p.Step, p.Overlap)
		}
	}

	// Assign each profile to buckets
	bucketMap := make(map[string]*Bucket) // bucketID → bucket

	for _, profile := range profiles {
		// Generate all partition value combinations for this profile
		combos := partitionCombos(profile, partitions, rangeBuckets)

		for _, combo := range combos {
			id := BucketID(combo)
			b, ok := bucketMap[id]
			if !ok {
				b = &Bucket{
					ID:              id,
					PartitionValues: combo,
				}
				bucketMap[id] = b
			}
			b.Tags = append(b.Tags, profile.Tag)
		}
	}

	// Convert map to slice
	result := make([]Bucket, 0, len(bucketMap))
	for _, b := range bucketMap {
		result = append(result, *b)
	}
	return result
}

// partitionCombos generates all bucket assignments for a profile.
// A profile may appear in multiple buckets due to range overlap or multi-value fields.
func partitionCombos(profile Profile, partitions []PartitionConfig, rangeBuckets map[string][]RangeBucket) []map[string]string {
	// Start with one empty combination
	combos := []map[string]string{{}}

	for _, part := range partitions {
		val, ok := profile.Attributes[part.Field]
		if !ok {
			continue
		}

		var fieldValues []string

		if part.IsRange() {
			// Range field: find which range buckets this value falls into
			intVal := toInt(val)
			for _, rb := range rangeBuckets[part.Field] {
				if rb.Contains(intVal) {
					fieldValues = append(fieldValues, rb.Label())
				}
			}
		} else {
			// Discrete field: extract values
			fieldValues = extractValues(val)
		}

		if len(fieldValues) == 0 {
			continue
		}

		// Cross-product: each existing combo × each field value
		var newCombos []map[string]string
		for _, combo := range combos {
			for _, fv := range fieldValues {
				newCombo := make(map[string]string, len(combo)+1)
				for k, v := range combo {
					newCombo[k] = v
				}
				newCombo[part.Field] = fv
				newCombos = append(newCombos, newCombo)
			}
		}
		combos = newCombos
	}

	return combos
}

// extractValues gets string values from a field value.
// Handles: string, []string, []any (with string elements).
func extractValues(val any) []string {
	switch v := val.(type) {
	case string:
		return []string{v}
	case []string:
		return v
	case []any:
		var result []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return []string{fmt.Sprintf("%v", val)}
	}
}

func toInt(val any) int {
	switch v := val.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case int64:
		return int(v)
	}
	return 0
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/bucket/ -v`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `make test`
Expected: all pass

- [ ] **Step 6: Commit**

```bash
git add internal/bucket/assign.go internal/bucket/bucket_test.go
git commit -m "feat(bucket): profile assignment with range overlap and multi-value fields"
git push
```
