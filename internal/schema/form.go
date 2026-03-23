package schema

import (
	"fmt"
	"sort"

	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
)

// BuildFormFields generates TUI form fields from schema attributes.
// existingProfile is used to pre-fill values (can be nil for new profiles).
// baseDir is needed for resolving file-based values.
func BuildFormFields(attrs map[string]Attribute, existingProfile map[string]any, baseDir string) ([]components.FormField, []components.Stepper, error) {
	// Sort attribute names for deterministic ordering.
	names := make([]string, 0, len(attrs))
	for name := range attrs {
		names = append(names, name)
	}
	sort.Strings(names)

	var fields []components.FormField
	var steppers []components.Stepper

	for _, name := range names {
		attr := attrs[name]

		switch attr.Type {
		case "enum":
			values, err := attr.ResolveValues(baseDir)
			if err != nil {
				return nil, nil, fmt.Errorf("resolving values for %s: %w", name, err)
			}
			selected := 0
			if existingProfile != nil {
				if v, ok := existingProfile[name].(string); ok {
					for i, opt := range values {
						if opt == v {
							selected = i
							break
						}
					}
				}
			}
			fields = append(fields, components.NewRadioField(name, name, values, selected))

		case "multi":
			values, err := attr.ResolveValues(baseDir)
			if err != nil {
				return nil, nil, fmt.Errorf("resolving values for %s: %w", name, err)
			}
			var selected []string
			if existingProfile != nil {
				selected = toStringList(existingProfile[name])
			}
			fields = append(fields, components.NewCheckboxField(name, name, values, selected))

		case "range":
			min := 0
			max := 100
			if attr.Min != nil {
				min = *attr.Min
			}
			if attr.Max != nil {
				max = *attr.Max
			}
			value := min
			if existingProfile != nil {
				if v, ok := toFloat64(existingProfile[name]); ok {
					value = int(v)
				}
			}
			steppers = append(steppers, components.NewStepper(name, name, value, min, max))

		case "text":
			value := ""
			if existingProfile != nil {
				if v, ok := existingProfile[name].(string); ok {
					value = v
				}
			}
			fields = append(fields, components.NewTextField(name, name, value))

		default:
			return nil, nil, fmt.Errorf("unknown attribute type %q for field %s", attr.Type, name)
		}
	}

	return fields, steppers, nil
}
