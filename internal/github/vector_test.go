package github

import (
	"math"
	"testing"
)

func intPtr(v int) *int { return &v }

func assertVec(t *testing.T, got, want []float32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("vec length = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if math.Abs(float64(got[i]-want[i])) > 0.001 {
			t.Errorf("vec[%d] = %f, want %f", i, got[i], want[i])
		}
	}
}

// --- Schema Dimensions ---

func TestSchemaField_Dimensions_Enum(t *testing.T) {
	f := SchemaField{Type: "enum", Values: []string{"a", "b", "c"}}
	if f.Dimensions() != 3 {
		t.Errorf("dims = %d, want 3", f.Dimensions())
	}
}

func TestSchemaField_Dimensions_Multi(t *testing.T) {
	f := SchemaField{Type: "multi", Values: []string{"a", "b"}}
	if f.Dimensions() != 2 {
		t.Errorf("dims = %d, want 2", f.Dimensions())
	}
}

func TestSchemaField_Dimensions_Range(t *testing.T) {
	f := SchemaField{Type: "range", Min: intPtr(0), Max: intPtr(100)}
	if f.Dimensions() != 1 {
		t.Errorf("dims = %d, want 1", f.Dimensions())
	}
}

func TestSchemaField_Dimensions_Unknown(t *testing.T) {
	f := SchemaField{Type: "unknown"}
	if f.Dimensions() != 0 {
		t.Errorf("dims = %d, want 0", f.Dimensions())
	}
}

func TestPoolSchema_Dimensions(t *testing.T) {
	s := &PoolSchema{
		Fields: []SchemaField{
			{Name: "gender", Type: "enum", Values: []string{"m", "f", "nb"}},
			{Name: "age", Type: "range", Min: intPtr(18), Max: intPtr(80)},
			{Name: "interests", Type: "multi", Values: []string{"a", "b", "c", "d"}},
		},
	}
	if s.Dimensions() != 8 { // 3 + 1 + 4
		t.Errorf("dims = %d, want 8", s.Dimensions())
	}
}

// --- Enum Encoding ---

func TestEncodeFieldValue_Enum(t *testing.T) {
	f := SchemaField{Name: "gender", Type: "enum", Values: []string{"male", "female", "nb"}}
	assertVec(t, EncodeFieldValue(f, "female"), []float32{0, 1, 0})
}

func TestEncodeFieldValue_Enum_First(t *testing.T) {
	f := SchemaField{Name: "gender", Type: "enum", Values: []string{"male", "female"}}
	assertVec(t, EncodeFieldValue(f, "male"), []float32{1, 0})
}

func TestEncodeFieldValue_Enum_Invalid(t *testing.T) {
	f := SchemaField{Name: "gender", Type: "enum", Values: []string{"male", "female"}}
	assertVec(t, EncodeFieldValue(f, "invalid"), []float32{0, 0})
}

func TestEncodeFieldValue_Enum_WrongType(t *testing.T) {
	f := SchemaField{Name: "gender", Type: "enum", Values: []string{"male", "female"}}
	assertVec(t, EncodeFieldValue(f, 42), []float32{0, 0})
}

// --- Multi Encoding ---

func TestEncodeFieldValue_Multi(t *testing.T) {
	f := SchemaField{Name: "interests", Type: "multi", Values: []string{"hiking", "coding", "music"}}
	assertVec(t, EncodeFieldValue(f, []string{"hiking", "music"}), []float32{1, 0, 1})
}

func TestEncodeFieldValue_Multi_All(t *testing.T) {
	f := SchemaField{Name: "interests", Type: "multi", Values: []string{"a", "b"}}
	assertVec(t, EncodeFieldValue(f, []string{"a", "b"}), []float32{1, 1})
}

func TestEncodeFieldValue_Multi_None(t *testing.T) {
	f := SchemaField{Name: "interests", Type: "multi", Values: []string{"a", "b"}}
	assertVec(t, EncodeFieldValue(f, []string{}), []float32{0, 0})
}

func TestEncodeFieldValue_Multi_AnySlice(t *testing.T) {
	f := SchemaField{Name: "interests", Type: "multi", Values: []string{"hiking", "coding"}}
	assertVec(t, EncodeFieldValue(f, []any{"hiking"}), []float32{1, 0})
}

func TestEncodeFieldValue_Multi_InvalidInSlice(t *testing.T) {
	f := SchemaField{Name: "interests", Type: "multi", Values: []string{"hiking", "coding"}}
	assertVec(t, EncodeFieldValue(f, []any{"unknown"}), []float32{0, 0})
}

func TestEncodeFieldValue_Multi_WrongType(t *testing.T) {
	f := SchemaField{Name: "interests", Type: "multi", Values: []string{"a", "b"}}
	assertVec(t, EncodeFieldValue(f, "not an array"), []float32{0, 0})
}

// --- Range Encoding ---

func TestEncodeFieldValue_Range_Mid(t *testing.T) {
	f := SchemaField{Name: "age", Type: "range", Min: intPtr(18), Max: intPtr(80)}
	assertVec(t, EncodeFieldValue(f, float64(49)), []float32{0.5})
}

func TestEncodeFieldValue_Range_Min(t *testing.T) {
	f := SchemaField{Name: "age", Type: "range", Min: intPtr(18), Max: intPtr(80)}
	assertVec(t, EncodeFieldValue(f, float64(18)), []float32{0})
}

func TestEncodeFieldValue_Range_Max(t *testing.T) {
	f := SchemaField{Name: "age", Type: "range", Min: intPtr(18), Max: intPtr(80)}
	assertVec(t, EncodeFieldValue(f, float64(80)), []float32{1})
}

func TestEncodeFieldValue_Range_BelowMin(t *testing.T) {
	f := SchemaField{Name: "age", Type: "range", Min: intPtr(18), Max: intPtr(80)}
	assertVec(t, EncodeFieldValue(f, float64(10)), []float32{0}) // clamped
}

func TestEncodeFieldValue_Range_AboveMax(t *testing.T) {
	f := SchemaField{Name: "age", Type: "range", Min: intPtr(18), Max: intPtr(80)}
	assertVec(t, EncodeFieldValue(f, float64(100)), []float32{1}) // clamped
}

func TestEncodeFieldValue_Range_Int(t *testing.T) {
	f := SchemaField{Name: "age", Type: "range", Min: intPtr(0), Max: intPtr(100)}
	assertVec(t, EncodeFieldValue(f, 50), []float32{0.5})
}

func TestEncodeFieldValue_Range_NoMinMax(t *testing.T) {
	f := SchemaField{Name: "age", Type: "range"}
	assertVec(t, EncodeFieldValue(f, float64(50)), []float32{0})
}

func TestEncodeFieldValue_Range_WrongType(t *testing.T) {
	f := SchemaField{Name: "age", Type: "range", Min: intPtr(0), Max: intPtr(100)}
	assertVec(t, EncodeFieldValue(f, "not a number"), []float32{0})
}

// --- Full Profile Encoding ---

func TestEncodeProfile_FullSchema(t *testing.T) {
	schema := &PoolSchema{
		Fields: []SchemaField{
			{Name: "gender", Type: "enum", Values: []string{"male", "female"}},
			{Name: "age", Type: "range", Min: intPtr(18), Max: intPtr(80)},
			{Name: "interests", Type: "multi", Values: []string{"a", "b", "c"}},
		},
	}
	profile := map[string]any{
		"gender":    "female",
		"age":       float64(49),
		"interests": []any{"a", "c"},
	}
	vec := EncodeProfile(schema, profile)
	// gender: [0,1] + age: [0.5] + interests: [1,0,1]
	assertVec(t, vec, []float32{0, 1, 0.5, 1, 0, 1})
}

func TestEncodeProfile_MissingField(t *testing.T) {
	schema := &PoolSchema{
		Fields: []SchemaField{
			{Name: "gender", Type: "enum", Values: []string{"male", "female"}},
		},
	}
	profile := map[string]any{}
	vec := EncodeProfile(schema, profile)
	assertVec(t, vec, []float32{0, 0})
}

func TestEncodeProfile_EmptySchema(t *testing.T) {
	schema := &PoolSchema{Fields: []SchemaField{}}
	profile := map[string]any{"gender": "male"}
	vec := EncodeProfile(schema, profile)
	if len(vec) != 0 {
		t.Errorf("expected empty vector, got %d dims", len(vec))
	}
}

// --- Weights ---

func TestApplyWeights_Basic(t *testing.T) {
	schema := &PoolSchema{Fields: []SchemaField{
		{Name: "gender", Type: "enum", Values: []string{"m", "f"}},
		{Name: "age", Type: "range", Min: intPtr(18), Max: intPtr(80)},
	}}
	vec := []float32{0, 1, 0.5}
	weights := map[string]float64{"gender": 10.0, "age": 2.0}
	result := ApplyWeights(schema, vec, weights)
	assertVec(t, result, []float32{0, 10, 1.0})
}

func TestApplyWeights_DefaultWeight(t *testing.T) {
	schema := &PoolSchema{Fields: []SchemaField{
		{Name: "gender", Type: "enum", Values: []string{"m", "f"}},
	}}
	vec := []float32{1, 0}
	weights := map[string]float64{} // no weight for gender → default 1.0
	result := ApplyWeights(schema, vec, weights)
	assertVec(t, result, []float32{1, 0})
}

func TestApplyWeights_ZeroWeight(t *testing.T) {
	schema := &PoolSchema{Fields: []SchemaField{
		{Name: "gender", Type: "enum", Values: []string{"m", "f"}},
	}}
	vec := []float32{1, 0}
	weights := map[string]float64{"gender": 0.0}
	result := ApplyWeights(schema, vec, weights)
	assertVec(t, result, []float32{0, 0})
}
