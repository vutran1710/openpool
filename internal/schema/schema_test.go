package schema

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_FromFile(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
name: test-pool
description: A test pool
relay_url: wss://relay.example.com
operator_public_key: abc123
profile:
  gender:
    type: enum
    values: [male, female, non-binary]
  age:
    type: range
    min: 18
    max: 99
  bio:
    type: text
    required: false
    visibility: private
`
	path := filepath.Join(dir, "pool.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if s.Name != "test-pool" {
		t.Errorf("Name = %q, want %q", s.Name, "test-pool")
	}
	if s.Description != "A test pool" {
		t.Errorf("Description = %q, want %q", s.Description, "A test pool")
	}
	if s.RelayURL != "wss://relay.example.com" {
		t.Errorf("RelayURL = %q, want %q", s.RelayURL, "wss://relay.example.com")
	}
	if s.OperatorPublicKey != "abc123" {
		t.Errorf("OperatorPublicKey = %q, want %q", s.OperatorPublicKey, "abc123")
	}
	if len(s.Profile) != 3 {
		t.Errorf("Profile has %d fields, want 3", len(s.Profile))
	}
	if s.Profile["gender"].Type != "enum" {
		t.Errorf("gender type = %q, want %q", s.Profile["gender"].Type, "enum")
	}
	if s.Profile["age"].Type != "range" {
		t.Errorf("age type = %q, want %q", s.Profile["age"].Type, "range")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestResolveValues_InlineList(t *testing.T) {
	a := Attribute{
		Type:   "enum",
		Values: []any{"red", "green", "blue"},
	}
	vals, err := a.ResolveValues("")
	if err != nil {
		t.Fatal(err)
	}
	if len(vals) != 3 {
		t.Fatalf("got %d values, want 3", len(vals))
	}
	if vals[0] != "red" || vals[1] != "green" || vals[2] != "blue" {
		t.Errorf("values = %v, want [red green blue]", vals)
	}
}

func TestResolveValues_CommaSeparated(t *testing.T) {
	a := Attribute{
		Type:   "enum",
		Values: "a, b, c",
	}
	vals, err := a.ResolveValues("")
	if err != nil {
		t.Fatal(err)
	}
	if len(vals) != 3 {
		t.Fatalf("got %d values, want 3", len(vals))
	}
	if vals[0] != "a" || vals[1] != "b" || vals[2] != "c" {
		t.Errorf("values = %v, want [a b c]", vals)
	}
}

func TestResolveValues_FileReference(t *testing.T) {
	dir := t.TempDir()
	valuesDir := filepath.Join(dir, "values")
	if err := os.MkdirAll(valuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(valuesDir, "test.txt"), []byte("alpha\nbeta\ngamma"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := Attribute{
		Type:   "enum",
		Values: "./values/test.txt",
	}
	vals, err := a.ResolveValues(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(vals) != 3 {
		t.Fatalf("got %d values, want 3", len(vals))
	}
	if vals[0] != "alpha" || vals[1] != "beta" || vals[2] != "gamma" {
		t.Errorf("values = %v, want [alpha beta gamma]", vals)
	}
}

func TestValidateProfile_Valid(t *testing.T) {
	s := &PoolSchema{
		Profile: map[string]Attribute{
			"gender": {Type: "enum", Values: []any{"male", "female"}},
			"age":    {Type: "range", Min: intPtr(18), Max: intPtr(99)},
			"bio":    {Type: "text"},
		},
	}

	profile := map[string]any{
		"gender": "male",
		"age":    25,
		"bio":    "hello world",
	}

	errs := s.ValidateProfile(profile)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateProfile_MissingRequired(t *testing.T) {
	s := &PoolSchema{
		Profile: map[string]Attribute{
			"gender": {Type: "enum", Values: []any{"male", "female"}},
		},
	}

	profile := map[string]any{}

	errs := s.ValidateProfile(profile)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Error() != "missing required field: gender" {
		t.Errorf("error = %q, want %q", errs[0].Error(), "missing required field: gender")
	}
}

func TestValidateProfile_InvalidEnum(t *testing.T) {
	s := &PoolSchema{
		Profile: map[string]Attribute{
			"gender": {Type: "enum", Values: []any{"male", "female"}},
		},
	}

	profile := map[string]any{
		"gender": "unknown",
	}

	errs := s.ValidateProfile(profile)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
}

func TestValidateProfile_RangeOutOfBounds(t *testing.T) {
	s := &PoolSchema{
		Profile: map[string]Attribute{
			"age": {Type: "range", Min: intPtr(18), Max: intPtr(99)},
		},
	}

	// Below minimum
	errs := s.ValidateProfile(map[string]any{"age": 10})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for below min, got %d: %v", len(errs), errs)
	}

	// Above maximum
	errs = s.ValidateProfile(map[string]any{"age": 100})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for above max, got %d: %v", len(errs), errs)
	}
}

func TestValidateProfile_OptionalMissing(t *testing.T) {
	s := &PoolSchema{
		Profile: map[string]Attribute{
			"bio": {Type: "text", Required: boolPtr(false)},
		},
	}

	profile := map[string]any{}

	errs := s.ValidateProfile(profile)
	if len(errs) != 0 {
		t.Errorf("expected no errors for missing optional, got %v", errs)
	}
}

func TestIsRequired_Default(t *testing.T) {
	a := Attribute{}
	if !a.IsRequired() {
		t.Error("expected IsRequired() to be true by default")
	}

	a2 := Attribute{Required: boolPtr(false)}
	if a2.IsRequired() {
		t.Error("expected IsRequired() to be false when set to false")
	}

	a3 := Attribute{Required: boolPtr(true)}
	if !a3.IsRequired() {
		t.Error("expected IsRequired() to be true when set to true")
	}
}

func TestIsPublic_Default(t *testing.T) {
	a := Attribute{}
	if !a.IsPublic() {
		t.Error("expected IsPublic() to be true by default")
	}

	a2 := Attribute{Visibility: "private"}
	if a2.IsPublic() {
		t.Error("expected IsPublic() to be false when visibility=private")
	}

	a3 := Attribute{Visibility: "public"}
	if !a3.IsPublic() {
		t.Error("expected IsPublic() to be true when visibility=public")
	}
}

func intPtr(v int) *int {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}
