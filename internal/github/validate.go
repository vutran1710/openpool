package github

import "fmt"

// ValidateProfile checks a profile against the pool schema.
func ValidateProfile(schema *PoolSchema, profile map[string]any) error {
	for _, field := range schema.Fields {
		value, exists := profile[field.Name]
		if !exists {
			return fmt.Errorf("missing field: %s", field.Name)
		}
		if err := validateField(field, value); err != nil {
			return fmt.Errorf("field %s: %w", field.Name, err)
		}
	}
	return nil
}

func validateField(field SchemaField, value any) error {
	switch field.Type {
	case "enum":
		return validateEnum(field, value)
	case "multi":
		return validateMulti(field, value)
	case "range":
		return validateRange(field, value)
	}
	return fmt.Errorf("unknown field type: %s", field.Type)
}

func validateEnum(field SchemaField, value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string, got %T", value)
	}
	if s == "" {
		return fmt.Errorf("empty value")
	}
	for _, v := range field.Values {
		if v == s {
			return nil
		}
	}
	return fmt.Errorf("invalid value %q (valid: %v)", s, field.Values)
}

func validateMulti(field SchemaField, value any) error {
	var items []string
	switch v := value.(type) {
	case []string:
		items = v
	case []any:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return fmt.Errorf("expected string in array, got %T", item)
			}
			items = append(items, s)
		}
	default:
		return fmt.Errorf("expected array, got %T", value)
	}
	if len(items) == 0 {
		return fmt.Errorf("empty array")
	}
	valid := make(map[string]bool)
	for _, v := range field.Values {
		valid[v] = true
	}
	for _, item := range items {
		if !valid[item] {
			return fmt.Errorf("invalid value %q (valid: %v)", item, field.Values)
		}
	}
	return nil
}

func validateRange(field SchemaField, value any) error {
	var val float64
	switch v := value.(type) {
	case float64:
		val = v
	case int:
		val = float64(v)
	case int64:
		val = float64(v)
	default:
		return fmt.Errorf("expected number, got %T", value)
	}
	if field.Min != nil && val < float64(*field.Min) {
		return fmt.Errorf("value %v below minimum %d", val, *field.Min)
	}
	if field.Max != nil && val > float64(*field.Max) {
		return fmt.Errorf("value %v above maximum %d", val, *field.Max)
	}
	return nil
}
