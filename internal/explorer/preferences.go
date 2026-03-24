package explorer

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Preferences defines what the user is looking for.
// Used to auto-select the best bucket.
type Preferences struct {
	LookingFor map[string]any `yaml:"looking_for"`
}

// LoadPreferences reads preferences from a YAML file.
// Returns empty preferences (no filtering) if file doesn't exist.
func LoadPreferences(path string) Preferences {
	data, err := os.ReadFile(path)
	if err != nil {
		return Preferences{}
	}
	var p Preferences
	yaml.Unmarshal(data, &p)
	return p
}

// RankBuckets sorts buckets by how well they match preferences.
// Best-matching buckets come first. Within equal scores, larger buckets rank higher.
func RankBuckets(buckets []BucketInfo, prefs Preferences) []BucketInfo {
	if len(buckets) == 0 {
		return buckets
	}
	if len(prefs.LookingFor) == 0 {
		// No preferences — sort by profile count descending
		sorted := make([]BucketInfo, len(buckets))
		copy(sorted, buckets)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].ProfileCount > sorted[j].ProfileCount
		})
		return sorted
	}

	// Score each bucket
	type scored struct {
		bucket BucketInfo
		score  int
	}
	var items []scored

	for _, b := range buckets {
		score := 0
		for field, prefVal := range prefs.LookingFor {
			bucketVal, ok := b.Partitions[field]
			if !ok {
				continue
			}
			if matchesPref(bucketVal, prefVal) {
				score++
			}
		}
		items = append(items, scored{bucket: b, score: score})
	}

	// Sort: highest score first, then by profile count
	sort.Slice(items, func(i, j int) bool {
		if items[i].score != items[j].score {
			return items[i].score > items[j].score
		}
		return items[i].bucket.ProfileCount > items[j].bucket.ProfileCount
	})

	result := make([]BucketInfo, len(items))
	for i, item := range items {
		result[i] = item.bucket
	}
	return result
}

// matchesPref checks if a bucket partition value matches a preference value.
func matchesPref(bucketVal string, prefVal any) bool {
	switch v := prefVal.(type) {
	case string:
		return bucketVal == v

	case map[string]any:
		// Range preference: { min: 25, max: 35 }
		// Bucket value: "25-30"
		parts := strings.SplitN(bucketVal, "-", 2)
		if len(parts) != 2 {
			return false
		}
		bMin, err1 := strconv.Atoi(parts[0])
		bMax, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return false
		}

		pMin := toIntFromAny(v["min"])
		pMax := toIntFromAny(v["max"])

		// Bucket overlaps with preference range
		return bMax >= pMin && bMin <= pMax

	default:
		return fmt.Sprintf("%v", prefVal) == bucketVal
	}
}

func toIntFromAny(val any) int {
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
