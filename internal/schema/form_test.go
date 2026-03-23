package schema

import (
	"testing"

	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
)

func formIntPtr(v int) *int { return &v }

func formBoolPtr(v bool) *bool { return &v }

func TestBuildFormFields_Enum(t *testing.T) {
	attrs := map[string]Attribute{
		"gender": {
			Type:   "enum",
			Values: []any{"male", "female", "other"},
		},
	}

	fields, steppers, err := BuildFormFields(attrs, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	if len(steppers) != 0 {
		t.Fatalf("expected 0 steppers, got %d", len(steppers))
	}
	f := fields[0]
	if f.Type != components.FieldRadio {
		t.Errorf("expected FieldRadio, got %d", f.Type)
	}
	if f.Name != "gender" {
		t.Errorf("expected name 'gender', got %q", f.Name)
	}
	if len(f.Options) != 3 {
		t.Errorf("expected 3 options, got %d", len(f.Options))
	}
}

func TestBuildFormFields_Multi(t *testing.T) {
	attrs := map[string]Attribute{
		"interests": {
			Type:   "multi",
			Values: []any{"hiking", "coding", "music", "travel"},
		},
	}

	fields, steppers, err := BuildFormFields(attrs, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	if len(steppers) != 0 {
		t.Fatalf("expected 0 steppers, got %d", len(steppers))
	}
	f := fields[0]
	if f.Type != components.FieldCheckbox {
		t.Errorf("expected FieldCheckbox, got %d", f.Type)
	}
	if len(f.Options) != 4 {
		t.Errorf("expected 4 options, got %d", len(f.Options))
	}
}

func TestBuildFormFields_Range(t *testing.T) {
	attrs := map[string]Attribute{
		"age": {
			Type: "range",
			Min:  formIntPtr(18),
			Max:  formIntPtr(99),
		},
	}

	fields, steppers, err := BuildFormFields(attrs, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 0 {
		t.Fatalf("expected 0 fields, got %d", len(fields))
	}
	if len(steppers) != 1 {
		t.Fatalf("expected 1 stepper, got %d", len(steppers))
	}
	s := steppers[0]
	if s.Name != "age" {
		t.Errorf("expected name 'age', got %q", s.Name)
	}
	if s.Min != 18 {
		t.Errorf("expected min 18, got %d", s.Min)
	}
	if s.Max != 99 {
		t.Errorf("expected max 99, got %d", s.Max)
	}
	if s.GetValue() != 18 {
		t.Errorf("expected initial value 18 (min), got %d", s.GetValue())
	}
}

func TestBuildFormFields_Text(t *testing.T) {
	attrs := map[string]Attribute{
		"about": {
			Type: "text",
		},
	}

	fields, steppers, err := BuildFormFields(attrs, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	if len(steppers) != 0 {
		t.Fatalf("expected 0 steppers, got %d", len(steppers))
	}
	f := fields[0]
	if f.Type != components.FieldTextArea {
		t.Errorf("expected FieldTextArea, got %d", f.Type)
	}
	if f.Value != "" {
		t.Errorf("expected empty value, got %q", f.Value)
	}
}

func TestBuildFormFields_PrefillFromExistingProfile(t *testing.T) {
	attrs := map[string]Attribute{
		"about": {Type: "text"},
		"age":   {Type: "range", Min: formIntPtr(18), Max: formIntPtr(99)},
		"gender": {
			Type:   "enum",
			Values: []any{"male", "female", "other"},
		},
		"interests": {
			Type:   "multi",
			Values: []any{"hiking", "coding", "music"},
		},
	}

	existing := map[string]any{
		"about":     "Hello world",
		"age":       float64(25), // JSON numbers are float64
		"gender":    "female",
		"interests": []any{"hiking", "music"},
	}

	fields, steppers, err := BuildFormFields(attrs, existing, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check text field pre-filled
	var aboutField *components.FormField
	for i := range fields {
		if fields[i].Name == "about" {
			aboutField = &fields[i]
			break
		}
	}
	if aboutField == nil {
		t.Fatal("expected 'about' field")
	}
	if aboutField.Value != "Hello world" {
		t.Errorf("expected about='Hello world', got %q", aboutField.Value)
	}

	// Check enum field pre-filled
	var genderField *components.FormField
	for i := range fields {
		if fields[i].Name == "gender" {
			genderField = &fields[i]
			break
		}
	}
	if genderField == nil {
		t.Fatal("expected 'gender' field")
	}
	if genderField.RadioIndex != 1 { // "female" is index 1
		t.Errorf("expected gender radio index 1, got %d", genderField.RadioIndex)
	}

	// Check multi field pre-filled
	var interestsField *components.FormField
	for i := range fields {
		if fields[i].Name == "interests" {
			interestsField = &fields[i]
			break
		}
	}
	if interestsField == nil {
		t.Fatal("expected 'interests' field")
	}
	if !interestsField.Selected[0] { // hiking
		t.Error("expected 'hiking' selected")
	}
	if interestsField.Selected[1] { // coding
		t.Error("expected 'coding' NOT selected")
	}
	if !interestsField.Selected[2] { // music
		t.Error("expected 'music' selected")
	}

	// Check stepper pre-filled
	if len(steppers) != 1 {
		t.Fatalf("expected 1 stepper, got %d", len(steppers))
	}
	if steppers[0].GetValue() != 25 {
		t.Errorf("expected age=25, got %d", steppers[0].GetValue())
	}
}

func TestBuildFormFields_MissingOptionalFields(t *testing.T) {
	attrs := map[string]Attribute{
		"about": {
			Type:     "text",
			Required: formBoolPtr(false),
		},
		"phone": {
			Type:     "text",
			Required: formBoolPtr(false),
		},
	}

	// Profile with only 'about' set, 'phone' missing
	existing := map[string]any{
		"about": "Hi there",
	}

	fields, _, err := BuildFormFields(attrs, existing, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(fields))
	}

	var aboutField, phoneField *components.FormField
	for i := range fields {
		switch fields[i].Name {
		case "about":
			aboutField = &fields[i]
		case "phone":
			phoneField = &fields[i]
		}
	}

	if aboutField.Value != "Hi there" {
		t.Errorf("expected about='Hi there', got %q", aboutField.Value)
	}
	if phoneField.Value != "" {
		t.Errorf("expected phone='', got %q", phoneField.Value)
	}
}

func TestBuildFormFields_UnknownType(t *testing.T) {
	attrs := map[string]Attribute{
		"foo": {Type: "unknown_type"},
	}

	_, _, err := BuildFormFields(attrs, nil, "")
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestBuildFormFields_CommaSeparatedValues(t *testing.T) {
	attrs := map[string]Attribute{
		"color": {
			Type:   "enum",
			Values: "red, green, blue",
		},
	}

	fields, _, err := BuildFormFields(attrs, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	if len(fields[0].Options) != 3 {
		t.Errorf("expected 3 options, got %d", len(fields[0].Options))
	}
}

func TestBuildFormFields_DeterministicOrder(t *testing.T) {
	attrs := map[string]Attribute{
		"zebra": {Type: "text"},
		"apple": {Type: "text"},
		"mango": {Type: "text"},
	}

	fields, _, err := BuildFormFields(attrs, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}
	if fields[0].Name != "apple" || fields[1].Name != "mango" || fields[2].Name != "zebra" {
		t.Errorf("expected sorted order [apple, mango, zebra], got [%s, %s, %s]",
			fields[0].Name, fields[1].Name, fields[2].Name)
	}
}
