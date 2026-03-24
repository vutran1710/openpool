package schema

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadProfile_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pools", "test-pool", "profile.json")

	profile := map[string]any{
		"name":   "Alice",
		"age":    float64(25),
		"tags":   []any{"hiking", "coding"},
	}

	if err := SaveProfile(path, profile); err != nil {
		t.Fatalf("SaveProfile failed: %v", err)
	}

	loaded, err := LoadProfile(path)
	if err != nil {
		t.Fatalf("LoadProfile failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected profile, got nil")
	}
	if loaded["name"] != "Alice" {
		t.Errorf("expected name='Alice', got %v", loaded["name"])
	}
	if loaded["age"] != float64(25) {
		t.Errorf("expected age=25, got %v", loaded["age"])
	}
}

func TestLoadProfile_NotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "profile.json")

	profile, err := LoadProfile(path)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if profile != nil {
		t.Fatalf("expected nil profile, got %v", profile)
	}
}

func TestLoadProfile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")

	os.WriteFile(path, []byte("not json"), 0600)

	profile, err := LoadProfile(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if profile != nil {
		t.Fatal("expected nil profile on error")
	}
}

func TestProfilePath(t *testing.T) {
	path := ProfilePath("/home/user/.openpool", "love-pool")
	expected := filepath.Join("/home/user/.openpool", "pools", "love-pool", "profile.json")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestLoadAndValidate_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")

	profile := map[string]any{
		"gender": "male",
		"about":  "Hello",
	}
	SaveProfile(path, profile)

	schema := &PoolSchema{
		Profile: map[string]Attribute{
			"gender": {Type: "enum", Values: []any{"male", "female"}},
			"about":  {Type: "text"},
		},
	}

	loaded, err := LoadAndValidate(path, schema)
	if err != nil {
		t.Fatalf("expected valid, got error: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected profile, got nil")
	}
	if loaded["gender"] != "male" {
		t.Errorf("expected gender='male', got %v", loaded["gender"])
	}
}

func TestLoadAndValidate_Invalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")

	profile := map[string]any{
		"gender": "invalid_value",
	}
	SaveProfile(path, profile)

	schema := &PoolSchema{
		Profile: map[string]Attribute{
			"gender": {Type: "enum", Values: []any{"male", "female"}},
		},
	}

	loaded, err := LoadAndValidate(path, schema)
	if err == nil {
		t.Fatal("expected error for invalid profile")
	}
	if loaded != nil {
		t.Fatal("expected nil profile on validation failure")
	}
}

func TestLoadAndValidate_Missing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "profile.json")

	schema := &PoolSchema{
		Profile: map[string]Attribute{
			"gender": {Type: "enum", Values: []any{"male", "female"}},
		},
	}

	loaded, err := LoadAndValidate(path, schema)
	if err != nil {
		t.Fatalf("expected nil error for missing, got %v", err)
	}
	if loaded != nil {
		t.Fatal("expected nil profile for missing")
	}
}

func TestLoadAndValidate_CorruptedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")

	os.WriteFile(path, []byte("{broken json"), 0600)

	schema := &PoolSchema{
		Profile: map[string]Attribute{
			"gender": {Type: "enum", Values: []any{"male", "female"}},
		},
	}

	loaded, err := LoadAndValidate(path, schema)
	if err == nil {
		t.Fatal("expected error for corrupted file")
	}
	if loaded != nil {
		t.Fatal("expected nil profile on error")
	}
}

func TestSaveProfile_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "nested", "dir", "profile.json")

	profile := map[string]any{"key": "value"}
	if err := SaveProfile(path, profile); err != nil {
		t.Fatalf("SaveProfile failed: %v", err)
	}

	loaded, err := LoadProfile(path)
	if err != nil {
		t.Fatalf("LoadProfile failed: %v", err)
	}
	if loaded["key"] != "value" {
		t.Errorf("expected key='value', got %v", loaded["key"])
	}
}
