package github

import (
	"fmt"
	"math"
	"strings"
)

// EncodeFieldValue encodes a profile field value into vector dimensions per the schema field type.
func EncodeFieldValue(field SchemaField, value any) []float32 {
	switch field.Type {
	case "enum":
		return encodeEnum(field, value)
	case "multi":
		return encodeMulti(field, value)
	case "range":
		return encodeRange(field, value)
	}
	return make([]float32, field.Dimensions())
}

func encodeEnum(field SchemaField, value any) []float32 {
	vec := make([]float32, len(field.Values))
	s, ok := value.(string)
	if !ok {
		return vec
	}
	for i, v := range field.Values {
		if v == s {
			vec[i] = 1.0
			return vec
		}
	}
	return vec
}

func encodeMulti(field SchemaField, value any) []float32 {
	vec := make([]float32, len(field.Values))
	var selected []string
	switch v := value.(type) {
	case []string:
		selected = v
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				selected = append(selected, s)
			}
		}
	default:
		return vec
	}
	for _, s := range selected {
		for i, v := range field.Values {
			if v == s {
				vec[i] = 1.0
			}
		}
	}
	return vec
}

func encodeRange(field SchemaField, value any) []float32 {
	if field.Min == nil || field.Max == nil {
		return []float32{0}
	}
	min := float64(*field.Min)
	max := float64(*field.Max)
	if max == min {
		return []float32{0}
	}

	var val float64
	switch v := value.(type) {
	case float64:
		val = v
	case float32:
		val = float64(v)
	case int:
		val = float64(v)
	case int64:
		val = float64(v)
	default:
		return []float32{0}
	}

	normalized := (val - min) / (max - min)
	normalized = math.Max(0, math.Min(1, normalized))
	return []float32{float32(normalized)}
}

// EncodeProfile encodes similarity fields of a profile into a vector.
// Filter fields (complementary, approximate, exact) are excluded.
func EncodeProfile(schema *PoolSchema, profile map[string]any) []float32 {
	var vec []float32
	for _, field := range schema.SimilarityFields() {
		value := profile[field.Name]
		vec = append(vec, EncodeFieldValue(field, value)...)
	}
	return vec
}

// ApplyWeights multiplies each similarity field's vector segment by its weight.
func ApplyWeights(schema *PoolSchema, vec []float32, weights map[string]float64) []float32 {
	result := make([]float32, len(vec))
	copy(result, vec)
	offset := 0
	for _, field := range schema.SimilarityFields() {
		dims := field.Dimensions()
		w, ok := weights[field.Name]
		if !ok {
			w = 1.0
		}
		for i := 0; i < dims; i++ {
			if offset+i < len(result) {
				result[offset+i] *= float32(w)
			}
		}
		offset += dims
	}
	return result
}

// FilterValues holds encoded filter field values for matching.
type FilterValues struct {
	Fields map[string]int `msgpack:"f"` // field name → encoded value (enum index, bitmask, or raw int)
}

// ExtractFilters extracts filter field values from a profile per the schema.
func ExtractFilters(schema *PoolSchema, profile map[string]any) FilterValues {
	fv := FilterValues{Fields: make(map[string]int)}
	for _, field := range schema.Fields {
		val := profile[field.Name]
		if val == nil {
			continue
		}
		switch field.Type {
		case "enum":
			fv.Fields[field.Name] = enumIndex(field, val)
		case "multi":
			fv.Fields[field.Name] = multiBitmask(field, val)
		case "range":
			fv.Fields[field.Name] = toInt(val)
		}
	}
	return fv
}

// IsMatch checks if two users pass all bilateral filters defined in the schema.
func IsMatch(schema *PoolSchema, a, b FilterValues) bool {
	for _, field := range schema.FilterFields() {
		switch field.Match {
		case "complementary":
			targetField := field.Target
			myVal := a.Fields[field.Name]
			theirTarget := b.Fields[targetField]
			theirVal := b.Fields[field.Name]
			myTarget := a.Fields[targetField]
			if (theirTarget&(1<<myVal)) == 0 || (myTarget&(1<<theirVal)) == 0 {
				return false
			}
		case "approximate":
			myVal := a.Fields[field.Name]
			theirVal := b.Fields[field.Name]
			diff := myVal - theirVal
			if diff < 0 {
				diff = -diff
			}
			if diff > field.Tolerance {
				return false
			}
		case "exact":
			myVal := a.Fields[field.Name]
			theirVal := b.Fields[field.Name]
			if myVal&theirVal == 0 {
				return false
			}
		}
	}
	return true
}

func enumIndex(field SchemaField, val any) int {
	s, ok := val.(string)
	if !ok {
		return 0
	}
	for i, v := range field.Values {
		if v == s {
			return i
		}
	}
	return 0
}

func multiBitmask(field SchemaField, val any) int {
	var selected []string
	switch v := val.(type) {
	case []string:
		selected = v
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				selected = append(selected, s)
			}
		}
	}
	mask := 0
	for _, s := range selected {
		for i, v := range field.Values {
			if v == s {
				mask |= 1 << i
			}
		}
	}
	return mask
}

// DecodeFilterLabel returns a human-readable label for a filter field value.
func DecodeFilterLabel(field SchemaField, value int) string {
	switch field.Type {
	case "enum":
		if value >= 0 && value < len(field.Values) {
			return field.Values[value]
		}
	case "multi":
		var labels []string
		for i, v := range field.Values {
			if value&(1<<i) != 0 {
				labels = append(labels, v)
			}
		}
		return strings.Join(labels, ", ")
	case "range":
		return fmt.Sprintf("%d", value)
	}
	return ""
}

// DecodeFilters returns human-readable labels for all filter values.
func DecodeFilters(schema *PoolSchema, fv FilterValues) map[string]string {
	labels := make(map[string]string)
	for _, field := range schema.Fields {
		val, ok := fv.Fields[field.Name]
		if !ok {
			continue
		}
		label := DecodeFilterLabel(field, val)
		if label != "" {
			labels[field.Name] = label
		}
	}
	return labels
}

func toInt(val any) int {
	switch v := val.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return 0
}
