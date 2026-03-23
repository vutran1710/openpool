package schema

import "fmt"

// ValidateProfile checks a profile (map of field name to value) against the schema.
// Returns a list of errors (empty = valid).
func (s *PoolSchema) ValidateProfile(profile map[string]any) []error {
	var errs []error

	for name, attr := range s.Profile {
		val, exists := profile[name]

		// Required check
		if attr.IsRequired() && !exists {
			errs = append(errs, fmt.Errorf("missing required field: %s", name))
			continue
		}
		if !exists {
			continue
		}

		// Type-specific validation
		switch attr.Type {
		case "enum":
			strVal, ok := val.(string)
			if !ok {
				errs = append(errs, fmt.Errorf("%s: expected string, got %T", name, val))
				continue
			}
			values, _ := attr.ResolveValues("")
			if !contains(values, strVal) {
				errs = append(errs, fmt.Errorf("%s: invalid value %q (allowed: %v)", name, strVal, values))
			}

		case "multi":
			list := toStringList(val)
			if list == nil {
				errs = append(errs, fmt.Errorf("%s: expected list of strings", name))
				continue
			}
			values, _ := attr.ResolveValues("")
			for _, item := range list {
				if !contains(values, item) {
					errs = append(errs, fmt.Errorf("%s: invalid value %q", name, item))
				}
			}

		case "range":
			num, ok := toFloat64(val)
			if !ok {
				errs = append(errs, fmt.Errorf("%s: expected number, got %T", name, val))
				continue
			}
			if attr.Min != nil && num < float64(*attr.Min) {
				errs = append(errs, fmt.Errorf("%s: %v below minimum %d", name, num, *attr.Min))
			}
			if attr.Max != nil && num > float64(*attr.Max) {
				errs = append(errs, fmt.Errorf("%s: %v above maximum %d", name, num, *attr.Max))
			}

		case "text":
			if _, ok := val.(string); !ok {
				errs = append(errs, fmt.Errorf("%s: expected string, got %T", name, val))
			}
		}
	}

	return errs
}

func contains(list []string, val string) bool {
	for _, v := range list {
		if v == val {
			return true
		}
	}
	return false
}

func toStringList(val any) []string {
	switch v := val.(type) {
	case []string:
		return v
	case []any:
		var list []string
		for _, item := range v {
			list = append(list, fmt.Sprint(item))
		}
		return list
	default:
		return nil
	}
}

func toFloat64(val any) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}
