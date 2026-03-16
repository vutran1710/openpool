package github

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// RegistrationTemplate defines the schema for pool registration forms.
// Pool operators place this at .github/registration.yml in their pool repo.
type RegistrationTemplate struct {
	Title  string              `yaml:"title"`
	Labels []string            `yaml:"labels"`
	Fields []RegistrationField `yaml:"fields"`
}

// RegistrationField is a single form field in the registration template.
type RegistrationField struct {
	ID          string   `yaml:"id"`
	Label       string   `yaml:"label"`
	Type        string   `yaml:"type"` // text, textarea, select, multiselect, checkbox
	Required    bool     `yaml:"required"`
	Description string   `yaml:"description"`
	Options     []string `yaml:"options"` // for select, multiselect
}

// ValidFieldTypes lists all supported field types.
var ValidFieldTypes = map[string]bool{
	"text":        true,
	"textarea":    true,
	"select":      true,
	"multiselect": true,
	"checkbox":    true,
}

// ParseRegistrationTemplate parses a registration.yml file.
func ParseRegistrationTemplate(data []byte) (*RegistrationTemplate, error) {
	var tmpl RegistrationTemplate

	// Try YAML first (via toml parser — both use similar syntax for simple cases)
	// Actually, use a simple line-based parser for YAML since we don't want a YAML dependency
	if err := parseYAML(data, &tmpl); err != nil {
		return nil, fmt.Errorf("parsing registration template: %w", err)
	}

	if tmpl.Title == "" {
		tmpl.Title = "Registration Request"
	}
	if len(tmpl.Labels) == 0 {
		tmpl.Labels = []string{"registration"}
	}

	for i, f := range tmpl.Fields {
		if f.ID == "" {
			return nil, fmt.Errorf("field %d missing id", i)
		}
		if f.Label == "" {
			tmpl.Fields[i].Label = f.ID
		}
		if f.Type == "" {
			tmpl.Fields[i].Type = "text"
		}
		if !ValidFieldTypes[tmpl.Fields[i].Type] {
			return nil, fmt.Errorf("field %q has invalid type %q", f.ID, f.Type)
		}
		if (f.Type == "select" || f.Type == "multiselect") && len(f.Options) == 0 {
			return nil, fmt.Errorf("field %q of type %q requires options", f.ID, f.Type)
		}
	}

	return &tmpl, nil
}

// DefaultTemplate returns the default registration template when none is configured.
func DefaultTemplate() *RegistrationTemplate {
	return &RegistrationTemplate{
		Title:  "Registration Request",
		Labels: []string{"registration"},
	}
}

// RenderIssueBody builds the GitHub Issue body.
// Format: first line = hex blob, remaining lines = key:value pairs.
func RenderIssueBody(tmpl *RegistrationTemplate, values map[string]string, blobHex string) string {
	var body strings.Builder

	// Line 1: blob
	body.WriteString(blobHex + "\n")

	// Remaining lines: template field values
	for _, f := range tmpl.Fields {
		val := values[f.ID]
		if val == "" {
			continue
		}
		body.WriteString(fmt.Sprintf("%s:%s\n", f.ID, val))
	}

	return body.String()
}

func parseYAML(data []byte, v any) error {
	return yaml.Unmarshal(data, v)
}
