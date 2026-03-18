package github

import "math"

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

// EncodeProfile encodes an entire profile into a vector per the schema.
func EncodeProfile(schema *PoolSchema, profile map[string]any) []float32 {
	var vec []float32
	for _, field := range schema.Fields {
		value := profile[field.Name]
		vec = append(vec, EncodeFieldValue(field, value)...)
	}
	return vec
}

// ApplyWeights multiplies each field's vector segment by its weight.
func ApplyWeights(schema *PoolSchema, vec []float32, weights map[string]float64) []float32 {
	result := make([]float32, len(vec))
	copy(result, vec)
	offset := 0
	for _, field := range schema.Fields {
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
