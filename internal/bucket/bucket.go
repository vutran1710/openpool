// Package bucket assigns profiles to buckets based on pool.yaml partition config.
// Pure logic — no crypto, no database, no IO.
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

		advance := step - overlap
		if advance < 1 {
			advance = 1
		}
		start += advance
	}

	return buckets
}
