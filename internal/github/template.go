package github

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var fieldPattern = regexp.MustCompile(`\{\{\s*(\w+)\s*\}\}`)

type PRTemplate struct {
	Raw    string
	Fields []TemplateField
}

type TemplateField struct {
	Name     string
	Label    string
	Required bool
}

func (c *Client) GetPRTemplate(ctx context.Context, templateName string) (*PRTemplate, error) {
	paths := []string{
		fmt.Sprintf(".github/PULL_REQUEST_TEMPLATE/%s.md", templateName),
		".github/PULL_REQUEST_TEMPLATE.md",
	}

	for _, path := range paths {
		data, err := c.GetFile(ctx, path)
		if err != nil {
			continue
		}
		return parseTemplate(string(data)), nil
	}

	return nil, nil
}

func parseTemplate(raw string) *PRTemplate {
	tmpl := &PRTemplate{Raw: raw}

	matches := fieldPattern.FindAllStringSubmatch(raw, -1)
	seen := make(map[string]bool)

	for _, match := range matches {
		name := match[1]
		if seen[name] {
			continue
		}
		seen[name] = true

		tmpl.Fields = append(tmpl.Fields, TemplateField{
			Name:     name,
			Label:    fieldNameToLabel(name),
			Required: true,
		})
	}

	return tmpl
}

func (t *PRTemplate) Render(values map[string]string) string {
	result := t.Raw
	for name, value := range values {
		placeholder := fmt.Sprintf("{{ %s }}", name)
		result = strings.ReplaceAll(result, placeholder, value)
		placeholder = fmt.Sprintf("{{%s}}", name)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

func (t *PRTemplate) HasFields() bool {
	return len(t.Fields) > 0
}

func IsFieldPlaceholder(line string) bool {
	trimmed := strings.TrimSpace(line)
	return fieldPattern.MatchString(trimmed) && fieldPattern.ReplaceAllString(trimmed, "") == ""
}

func fieldNameToLabel(name string) string {
	result := strings.ReplaceAll(name, "_", " ")
	if len(result) > 0 {
		result = strings.ToUpper(result[:1]) + result[1:]
	}
	return result
}
