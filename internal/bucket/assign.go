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
	bucketMap := make(map[string]*Bucket)

	for _, profile := range profiles {
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

	result := make([]Bucket, 0, len(bucketMap))
	for _, b := range bucketMap {
		result = append(result, *b)
	}
	return result
}

// partitionCombos generates all bucket assignments for a profile.
// A profile may appear in multiple buckets due to range overlap or multi-value fields.
func partitionCombos(profile Profile, partitions []PartitionConfig, rangeBuckets map[string][]RangeBucket) []map[string]string {
	combos := []map[string]string{{}}

	for _, part := range partitions {
		val, ok := profile.Attributes[part.Field]
		if !ok {
			continue
		}

		var fieldValues []string

		if part.IsRange() {
			intVal := toInt(val)
			for _, rb := range rangeBuckets[part.Field] {
				if rb.Contains(intVal) {
					fieldValues = append(fieldValues, rb.Label())
				}
			}
		} else {
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
