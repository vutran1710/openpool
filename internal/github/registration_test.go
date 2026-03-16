package github

import (
	"strings"
	"testing"
)

func TestParseRegistrationTemplate_Valid(t *testing.T) {
	yml := `
title: "Join the Pool"
labels:
  - registration
  - new-member
fields:
  - id: display_name
    label: "Display Name"
    type: text
    required: true
  - id: age
    label: "Age Range"
    type: select
    required: true
    options:
      - "18-25"
      - "26-35"
      - "36+"
  - id: vibes
    label: "Your Vibes"
    type: multiselect
    options:
      - chill
      - nerdy
      - adventurous
  - id: rules
    label: "Accept rules"
    type: checkbox
    required: true
  - id: intro
    label: "Quick intro"
    type: textarea
`

	tmpl, err := ParseRegistrationTemplate([]byte(yml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if tmpl.Title != "Join the Pool" {
		t.Errorf("expected title 'Join the Pool', got %q", tmpl.Title)
	}
	if len(tmpl.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(tmpl.Labels))
	}
	if len(tmpl.Fields) != 5 {
		t.Errorf("expected 5 fields, got %d", len(tmpl.Fields))
	}

	// Check field types
	types := map[string]string{
		"display_name": "text",
		"age":          "select",
		"vibes":        "multiselect",
		"rules":        "checkbox",
		"intro":        "textarea",
	}
	for _, f := range tmpl.Fields {
		expected, ok := types[f.ID]
		if !ok {
			t.Errorf("unexpected field: %s", f.ID)
			continue
		}
		if f.Type != expected {
			t.Errorf("field %s: expected type %s, got %s", f.ID, expected, f.Type)
		}
	}

	// Check select options
	for _, f := range tmpl.Fields {
		if f.ID == "age" && len(f.Options) != 3 {
			t.Errorf("age field: expected 3 options, got %d", len(f.Options))
		}
	}
}

func TestParseRegistrationTemplate_Defaults(t *testing.T) {
	yml := `
fields:
  - id: name
`
	tmpl, err := ParseRegistrationTemplate([]byte(yml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if tmpl.Title != "Registration Request" {
		t.Errorf("expected default title, got %q", tmpl.Title)
	}
	if len(tmpl.Labels) != 1 || tmpl.Labels[0] != "registration" {
		t.Errorf("expected default label, got %v", tmpl.Labels)
	}
	if tmpl.Fields[0].Type != "text" {
		t.Errorf("expected default type 'text', got %q", tmpl.Fields[0].Type)
	}
	if tmpl.Fields[0].Label != "name" {
		t.Errorf("expected label to default to id, got %q", tmpl.Fields[0].Label)
	}
}

func TestParseRegistrationTemplate_InvalidType(t *testing.T) {
	yml := `
fields:
  - id: bad
    type: dropdown
`
	_, err := ParseRegistrationTemplate([]byte(yml))
	if err == nil {
		t.Error("expected error for invalid field type")
	}
}

func TestParseRegistrationTemplate_SelectWithoutOptions(t *testing.T) {
	yml := `
fields:
  - id: choice
    type: select
`
	_, err := ParseRegistrationTemplate([]byte(yml))
	if err == nil {
		t.Error("expected error for select without options")
	}
}

func TestParseRegistrationTemplate_MissingID(t *testing.T) {
	yml := `
fields:
  - label: "No ID"
    type: text
`
	_, err := ParseRegistrationTemplate([]byte(yml))
	if err == nil {
		t.Error("expected error for field without id")
	}
}

func TestDefaultTemplate(t *testing.T) {
	tmpl := DefaultTemplate()
	if tmpl.Title != "Registration Request" {
		t.Errorf("unexpected title: %q", tmpl.Title)
	}
	if len(tmpl.Fields) != 0 {
		t.Errorf("expected no fields, got %d", len(tmpl.Fields))
	}
}

func TestRenderIssueBody(t *testing.T) {
	tmpl := &RegistrationTemplate{
		Fields: []RegistrationField{
			{ID: "name", Label: "Name"},
			{ID: "age", Label: "Age"},
			{ID: "empty", Label: "Empty"},
		},
	}
	values := map[string]string{
		"name": "Alice",
		"age":  "26-35",
	}

	body := RenderIssueBody(tmpl, values, "deadbeef")

	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) < 1 {
		t.Fatal("body is empty")
	}

	// First line = blob
	if lines[0] != "deadbeef" {
		t.Errorf("first line should be blob, got %q", lines[0])
	}

	// key:value pairs
	if !strings.Contains(body, "name:Alice") {
		t.Error("missing name:Alice")
	}
	if !strings.Contains(body, "age:26-35") {
		t.Error("missing age:26-35")
	}

	// Empty fields should not appear
	if strings.Contains(body, "empty:") {
		t.Error("empty field should not appear")
	}
}

func TestRenderIssueBody_NoFields(t *testing.T) {
	tmpl := DefaultTemplate()
	body := RenderIssueBody(tmpl, nil, "aabbccdd")

	lines := strings.Split(strings.TrimSpace(body), "\n")
	if lines[0] != "aabbccdd" {
		t.Errorf("expected blob on first line, got %q", lines[0])
	}
	if len(lines) != 1 {
		t.Errorf("expected only 1 line (blob), got %d", len(lines))
	}
}
