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

func TestPoolSchema_Dimensions_SimilarityOnly(t *testing.T) {
	s := &PoolSchema{Fields: []SchemaField{
		{Name: "gender", Type: "enum", Match: "complementary", Values: []string{"m", "f", "nb"}},
		{Name: "gender_target", Type: "multi", Values: []string{"m", "f", "nb"}},
		{Name: "age", Type: "range", Match: "approximate", Tolerance: 5},
		{Name: "intent", Type: "multi", Match: "exact", Values: []string{"dating", "friends"}},
		{Name: "interests", Type: "multi", Match: "similarity", Values: []string{"a", "b", "c"}},
	}}
	// Only similarity: interests(3) = 3
	if s.Dimensions() != 3 {
		t.Errorf("dims = %d, want 3", s.Dimensions())
	}
}

func TestSchemaField_IsSimilarity(t *testing.T) {
	if !(SchemaField{Match: "similarity"}).IsSimilarity() {
		t.Error("similarity should be true")
	}
	if (SchemaField{Match: "complementary"}).IsSimilarity() {
		t.Error("complementary should not be similarity")
	}
	if (SchemaField{Match: ""}).IsSimilarity() {
		t.Error("empty match should not be similarity")
	}
}

func TestSchemaField_IsFilter(t *testing.T) {
	for _, m := range []string{"complementary", "approximate", "exact"} {
		if !(SchemaField{Match: m}).IsFilter() {
			t.Errorf("%s should be filter", m)
		}
	}
	if (SchemaField{Match: "similarity"}).IsFilter() {
		t.Error("similarity should not be filter")
	}
	if (SchemaField{Match: ""}).IsFilter() {
		t.Error("empty should not be filter")
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

func TestEncodeFieldValue_Range_Clamped(t *testing.T) {
	f := SchemaField{Name: "age", Type: "range", Min: intPtr(18), Max: intPtr(80)}
	assertVec(t, EncodeFieldValue(f, float64(10)), []float32{0})
	assertVec(t, EncodeFieldValue(f, float64(100)), []float32{1})
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

// --- Profile Encoding (similarity fields only) ---

func TestEncodeProfile_SimilarityOnly(t *testing.T) {
	schema := &PoolSchema{Fields: []SchemaField{
		{Name: "gender", Type: "enum", Match: "complementary", Target: "gender_target", Values: []string{"male", "female"}},
		{Name: "gender_target", Type: "multi", Values: []string{"male", "female"}},
		{Name: "intent", Type: "multi", Match: "similarity", Values: []string{"dating", "friends"}},
		{Name: "interests", Type: "multi", Match: "similarity", Values: []string{"a", "b", "c"}},
	}}
	profile := map[string]any{
		"gender": "female", "gender_target": []any{"male"},
		"intent": []any{"dating"}, "interests": []any{"a", "c"},
	}
	vec := EncodeProfile(schema, profile)
	// Only similarity: intent [1,0] + interests [1,0,1] = 5 dims
	assertVec(t, vec, []float32{1, 0, 1, 0, 1})
}

func TestEncodeProfile_MissingField(t *testing.T) {
	schema := &PoolSchema{Fields: []SchemaField{
		{Name: "interests", Type: "multi", Match: "similarity", Values: []string{"a", "b"}},
	}}
	vec := EncodeProfile(schema, map[string]any{})
	assertVec(t, vec, []float32{0, 0})
}

func TestEncodeProfile_EmptySchema(t *testing.T) {
	schema := &PoolSchema{Fields: []SchemaField{}}
	vec := EncodeProfile(schema, map[string]any{"x": "y"})
	if len(vec) != 0 {
		t.Errorf("expected empty vector, got %d dims", len(vec))
	}
}

// --- Weights (similarity fields only) ---

func TestApplyWeights_Basic(t *testing.T) {
	schema := &PoolSchema{Fields: []SchemaField{
		{Name: "intent", Type: "multi", Match: "similarity", Values: []string{"dating", "friends"}},
		{Name: "interests", Type: "multi", Match: "similarity", Values: []string{"a", "b"}},
	}}
	vec := []float32{1, 0, 1, 1}
	weights := map[string]float64{"intent": 3.0, "interests": 1.0}
	result := ApplyWeights(schema, vec, weights)
	assertVec(t, result, []float32{3, 0, 1, 1})
}

func TestApplyWeights_DefaultWeight(t *testing.T) {
	schema := &PoolSchema{Fields: []SchemaField{
		{Name: "interests", Type: "multi", Match: "similarity", Values: []string{"a", "b"}},
	}}
	result := ApplyWeights(schema, []float32{1, 0}, map[string]float64{})
	assertVec(t, result, []float32{1, 0})
}

func TestApplyWeights_ZeroWeight(t *testing.T) {
	schema := &PoolSchema{Fields: []SchemaField{
		{Name: "interests", Type: "multi", Match: "similarity", Values: []string{"a", "b"}},
	}}
	result := ApplyWeights(schema, []float32{1, 0}, map[string]float64{"interests": 0.0})
	assertVec(t, result, []float32{0, 0})
}

func TestApplyWeights_SkipsFilterFields(t *testing.T) {
	schema := &PoolSchema{Fields: []SchemaField{
		{Name: "gender", Type: "enum", Match: "complementary", Values: []string{"m", "f"}},
		{Name: "interests", Type: "multi", Match: "similarity", Values: []string{"a", "b"}},
	}}
	result := ApplyWeights(schema, []float32{1, 0}, map[string]float64{"interests": 2.0})
	assertVec(t, result, []float32{2, 0})
}

// --- Filter Extraction ---

func TestExtractFilters(t *testing.T) {
	schema := &PoolSchema{Fields: []SchemaField{
		{Name: "gender", Type: "enum", Match: "complementary", Target: "gender_target", Values: []string{"male", "female", "nb"}},
		{Name: "gender_target", Type: "multi", Values: []string{"male", "female", "nb"}},
		{Name: "age", Type: "range", Match: "approximate", Tolerance: 5},
		{Name: "intent", Type: "multi", Match: "exact", Values: []string{"dating", "friends"}},
	}}
	profile := map[string]any{
		"gender": "female", "gender_target": []any{"male", "nb"},
		"age": float64(28), "intent": []any{"dating"},
	}
	fv := ExtractFilters(schema, profile)
	if fv.Fields["gender"] != 1 { t.Errorf("gender = %d, want 1 (female)", fv.Fields["gender"]) }
	if fv.Fields["gender_target"] != 5 { t.Errorf("gender_target = %d, want 5 (male+nb bitmask 101)", fv.Fields["gender_target"]) }
	if fv.Fields["age"] != 28 { t.Errorf("age = %d", fv.Fields["age"]) }
	if fv.Fields["intent"] != 1 { t.Errorf("intent = %d, want 1 (dating bitmask)", fv.Fields["intent"]) }
}

// --- IsMatch (Bilateral Filtering) ---

func datingSchema() *PoolSchema {
	return &PoolSchema{Fields: []SchemaField{
		{Name: "gender", Type: "enum", Match: "complementary", Target: "gender_target", Values: []string{"male", "female", "nb"}},
		{Name: "gender_target", Type: "multi", Values: []string{"male", "female", "nb"}},
		{Name: "age", Type: "range", Match: "approximate", Tolerance: 5},
		{Name: "intent", Type: "multi", Match: "exact", Values: []string{"dating", "friends"}},
	}}
}

func TestIsMatch_Compatible(t *testing.T) {
	s := datingSchema()
	alice := FilterValues{Fields: map[string]int{"gender": 1, "gender_target": 1, "age": 28, "intent": 1}} // F→M, dating
	bob := FilterValues{Fields: map[string]int{"gender": 0, "gender_target": 2, "age": 32, "intent": 1}}   // M→F, dating
	if !IsMatch(s, alice, bob) {
		t.Error("Alice and Bob should match")
	}
}

func TestIsMatch_GenderMismatch(t *testing.T) {
	s := datingSchema()
	alice := FilterValues{Fields: map[string]int{"gender": 1, "gender_target": 1, "age": 28, "intent": 1}} // F→M
	dana := FilterValues{Fields: map[string]int{"gender": 1, "gender_target": 3, "age": 30, "intent": 3}}  // F→M+F
	if IsMatch(s, alice, dana) {
		t.Error("Alice (F→M) and Dana (F→M+F): Alice doesn't target F")
	}
}

func TestIsMatch_AgeMismatch(t *testing.T) {
	s := datingSchema()
	alice := FilterValues{Fields: map[string]int{"gender": 1, "gender_target": 1, "age": 28, "intent": 1}}
	frank := FilterValues{Fields: map[string]int{"gender": 0, "gender_target": 2, "age": 45, "intent": 1}}
	if IsMatch(s, alice, frank) {
		t.Error("|28-45|=17 > tolerance 5")
	}
}

func TestIsMatch_IntentMismatch(t *testing.T) {
	s := datingSchema()
	alice := FilterValues{Fields: map[string]int{"gender": 1, "gender_target": 1, "age": 28, "intent": 1}} // dating
	charlie := FilterValues{Fields: map[string]int{"gender": 0, "gender_target": 6, "age": 25, "intent": 2}} // friends
	if IsMatch(s, alice, charlie) {
		t.Error("dating & friends = 0, no overlap")
	}
}

func TestIsMatch_IntentOverlap(t *testing.T) {
	s := datingSchema()
	alice := FilterValues{Fields: map[string]int{"gender": 1, "gender_target": 1, "age": 28, "intent": 1}}  // dating
	dana := FilterValues{Fields: map[string]int{"gender": 0, "gender_target": 2, "age": 30, "intent": 3}}   // dating+friends
	if !IsMatch(s, alice, dana) {
		t.Error("dating & (dating+friends) should overlap")
	}
}

func TestIsMatch_NBTargetsAll(t *testing.T) {
	s := datingSchema()
	eve := FilterValues{Fields: map[string]int{"gender": 2, "gender_target": 7, "age": 25, "intent": 1}} // NB→all
	bob := FilterValues{Fields: map[string]int{"gender": 0, "gender_target": 4, "age": 25, "intent": 1}} // M→NB
	if !IsMatch(s, eve, bob) {
		t.Error("Eve (NB→all) and Bob (M→NB) should match")
	}
}

func TestIsMatch_NoFilterSchema(t *testing.T) {
	s := &PoolSchema{Fields: []SchemaField{
		{Name: "interests", Type: "multi", Match: "similarity", Values: []string{"a", "b"}},
	}}
	a := FilterValues{Fields: map[string]int{}}
	b := FilterValues{Fields: map[string]int{}}
	if !IsMatch(s, a, b) {
		t.Error("no filter fields = everyone passes")
	}
}

func TestIsMatch_AgeBorderline(t *testing.T) {
	s := datingSchema()
	a := FilterValues{Fields: map[string]int{"gender": 0, "gender_target": 2, "age": 25, "intent": 1}}
	b := FilterValues{Fields: map[string]int{"gender": 1, "gender_target": 1, "age": 30, "intent": 1}}
	if !IsMatch(s, a, b) {
		t.Error("|25-30|=5 == tolerance 5, should pass")
	}
}

func TestIsMatch_AgeJustOver(t *testing.T) {
	s := datingSchema()
	a := FilterValues{Fields: map[string]int{"gender": 0, "gender_target": 2, "age": 24, "intent": 1}}
	b := FilterValues{Fields: map[string]int{"gender": 1, "gender_target": 1, "age": 30, "intent": 1}}
	if IsMatch(s, a, b) {
		t.Error("|24-30|=6 > tolerance 5, should fail")
	}
}
