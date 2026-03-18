package github

import "testing"

func testSchema() *PoolSchema {
	return &PoolSchema{
		Version: 1,
		Fields: []SchemaField{
			{Name: "gender", Type: "enum", Values: []string{"male", "female", "nb"}},
			{Name: "age", Type: "range", Min: intPtr(18), Max: intPtr(80)},
			{Name: "interests", Type: "multi", Values: []string{"hiking", "coding", "music"}},
		},
	}
}

func TestValidateProfile_Valid(t *testing.T) {
	profile := map[string]any{
		"gender":    "male",
		"age":       float64(25),
		"interests": []any{"hiking", "coding"},
	}
	if err := ValidateProfile(testSchema(), profile); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateProfile_MissingField(t *testing.T) {
	profile := map[string]any{
		"gender": "male",
		"age":    float64(25),
		// missing interests
	}
	err := ValidateProfile(testSchema(), profile)
	if err == nil {
		t.Fatal("expected error for missing field")
	}
}

func TestValidateProfile_InvalidEnum(t *testing.T) {
	profile := map[string]any{
		"gender":    "invalid",
		"age":       float64(25),
		"interests": []any{"hiking"},
	}
	err := ValidateProfile(testSchema(), profile)
	if err == nil {
		t.Fatal("expected error for invalid enum")
	}
}

func TestValidateProfile_EmptyEnum(t *testing.T) {
	profile := map[string]any{
		"gender":    "",
		"age":       float64(25),
		"interests": []any{"hiking"},
	}
	err := ValidateProfile(testSchema(), profile)
	if err == nil {
		t.Fatal("expected error for empty enum")
	}
}

func TestValidateProfile_RangeBelowMin(t *testing.T) {
	profile := map[string]any{
		"gender":    "male",
		"age":       float64(5),
		"interests": []any{"hiking"},
	}
	err := ValidateProfile(testSchema(), profile)
	if err == nil {
		t.Fatal("expected error for age below min")
	}
}

func TestValidateProfile_RangeAboveMax(t *testing.T) {
	profile := map[string]any{
		"gender":    "male",
		"age":       float64(200),
		"interests": []any{"hiking"},
	}
	err := ValidateProfile(testSchema(), profile)
	if err == nil {
		t.Fatal("expected error for age above max")
	}
}

func TestValidateProfile_InvalidMultiValue(t *testing.T) {
	profile := map[string]any{
		"gender":    "male",
		"age":       float64(25),
		"interests": []any{"nonexistent"},
	}
	err := ValidateProfile(testSchema(), profile)
	if err == nil {
		t.Fatal("expected error for invalid multi value")
	}
}

func TestValidateProfile_EmptyMulti(t *testing.T) {
	profile := map[string]any{
		"gender":    "male",
		"age":       float64(25),
		"interests": []any{},
	}
	err := ValidateProfile(testSchema(), profile)
	if err == nil {
		t.Fatal("expected error for empty multi")
	}
}

func TestValidateProfile_WrongTypeEnum(t *testing.T) {
	profile := map[string]any{
		"gender":    42,
		"age":       float64(25),
		"interests": []any{"hiking"},
	}
	err := ValidateProfile(testSchema(), profile)
	if err == nil {
		t.Fatal("expected error for wrong type")
	}
}

func TestValidateProfile_WrongTypeRange(t *testing.T) {
	profile := map[string]any{
		"gender":    "male",
		"age":       "twenty five",
		"interests": []any{"hiking"},
	}
	err := ValidateProfile(testSchema(), profile)
	if err == nil {
		t.Fatal("expected error for wrong type")
	}
}
