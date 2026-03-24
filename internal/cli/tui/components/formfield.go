package components

import (
	"fmt"
	"strings"

	"github.com/vutran1710/openpool/internal/cli/tui/theme"
)

// FormFieldType defines the kind of form input.
type FormFieldType int

const (
	FieldText FormFieldType = iota
	FieldTextArea
	FieldRadio
	FieldCheckbox
	FieldNumber
)

// FormField represents a single field in a profile form.
type FormField struct {
	Name        string
	Label       string
	Type        FormFieldType
	Options     []string      // for Radio/Checkbox
	Value       string        // for Text/TextArea/Number
	Selected    map[int]bool  // for Checkbox (indices)
	RadioIndex  int           // for Radio (selected index)
	MaxLen      int           // for TextArea
	Focused     bool
	CursorPos   int           // for text editing
}

// NewTextField creates a text input field.
func NewTextField(name, label, value string) FormField {
	return FormField{Name: name, Label: label, Type: FieldText, Value: value, CursorPos: len(value)}
}

// NewTextAreaField creates a text area field with max length.
func NewTextAreaField(name, label, value string, maxLen int) FormField {
	return FormField{Name: name, Label: label, Type: FieldTextArea, Value: value, MaxLen: maxLen, CursorPos: len(value)}
}

// NewRadioField creates a radio select field.
func NewRadioField(name, label string, options []string, selected int) FormField {
	return FormField{Name: name, Label: label, Type: FieldRadio, Options: options, RadioIndex: selected}
}

// NewCheckboxField creates a checkbox multi-select field.
func NewCheckboxField(name, label string, options []string, selected []string) FormField {
	sel := make(map[int]bool)
	for _, s := range selected {
		for i, o := range options {
			if o == s {
				sel[i] = true
			}
		}
	}
	return FormField{Name: name, Label: label, Type: FieldCheckbox, Options: options, Selected: sel}
}

// NewNumberField creates a number input field.
func NewNumberField(name, label string, value int, min, max int) FormField {
	return FormField{Name: name, Label: label, Type: FieldNumber, Value: fmt.Sprintf("%d", value)}
}

// GetValue returns the field's current value as any (for JSON marshaling).
func (f FormField) GetValue() any {
	switch f.Type {
	case FieldText, FieldTextArea:
		return f.Value
	case FieldNumber:
		var n int
		fmt.Sscanf(f.Value, "%d", &n)
		return n
	case FieldRadio:
		if f.RadioIndex >= 0 && f.RadioIndex < len(f.Options) {
			return f.Options[f.RadioIndex]
		}
		return ""
	case FieldCheckbox:
		var selected []string
		for i, o := range f.Options {
			if f.Selected[i] {
				selected = append(selected, o)
			}
		}
		return selected
	}
	return nil
}

// Render renders the field for display.
func (f FormField) Render(width int) string {
	label := theme.BoldStyle.Render(f.Label)
	if f.Focused {
		label = theme.BrandStyle.Render("▸ " + f.Label)
	}

	var content string
	switch f.Type {
	case FieldText:
		content = f.renderText()
	case FieldTextArea:
		content = f.renderTextArea(width)
	case FieldRadio:
		content = f.renderRadio()
	case FieldCheckbox:
		content = f.renderCheckbox()
	case FieldNumber:
		content = f.renderText()
	}

	return label + "\n" + content
}

func (f FormField) renderText() string {
	val := f.Value
	if val == "" {
		val = theme.DimStyle.Render("(empty)")
	} else {
		val = theme.TextStyle.Render(val)
	}
	if f.Focused {
		val += theme.BrandStyle.Render("▏")
	}
	return "  " + val
}

func (f FormField) renderTextArea(width int) string {
	val := f.Value
	if val == "" {
		val = theme.DimStyle.Render("(empty)")
	} else {
		val = theme.TextStyle.Render(val)
	}
	if f.Focused {
		val += theme.BrandStyle.Render("▏")
	}
	charCount := ""
	if f.MaxLen > 0 {
		charCount = theme.DimStyle.Render(fmt.Sprintf("  %d/%d", len(f.Value), f.MaxLen))
	}
	return "  " + val + charCount
}

func (f FormField) renderRadio() string {
	var lines []string
	for i, opt := range f.Options {
		marker := "  ○ "
		style := theme.DimStyle
		if i == f.RadioIndex {
			marker = "  ● "
			style = theme.TextStyle
		}
		if f.Focused && i == f.RadioIndex {
			style = theme.BrandStyle
		}
		lines = append(lines, marker+style.Render(opt))
	}
	return strings.Join(lines, "\n")
}

func (f FormField) renderCheckbox() string {
	var tags []string
	for i, opt := range f.Options {
		if f.Selected[i] {
			if f.Focused && i == f.RadioIndex {
				tags = append(tags, theme.BrandStyle.Render("["+opt+"]"))
			} else {
				tags = append(tags, theme.AccentStyle.Render("["+opt+"]"))
			}
		} else {
			if f.Focused && i == f.RadioIndex {
				tags = append(tags, theme.BrandStyle.Render(" "+opt+" "))
			} else {
				tags = append(tags, theme.DimStyle.Render(" "+opt+" "))
			}
		}
	}
	return "  " + strings.Join(tags, " ")
}

// HandleKey processes a key press for the focused field. Returns true if handled.
func (f *FormField) HandleKey(key string) bool {
	switch f.Type {
	case FieldText, FieldNumber:
		return f.handleTextKey(key)
	case FieldTextArea:
		return f.handleTextAreaKey(key)
	case FieldRadio:
		return f.handleRadioKey(key)
	case FieldCheckbox:
		return f.handleCheckboxKey(key)
	}
	return false
}

func (f *FormField) handleTextKey(key string) bool {
	switch key {
	case "backspace":
		if len(f.Value) > 0 {
			f.Value = f.Value[:len(f.Value)-1]
		}
		return true
	default:
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			if f.Type == FieldNumber {
				if key[0] >= '0' && key[0] <= '9' {
					f.Value += key
					return true
				}
				return false
			}
			f.Value += key
			return true
		}
	}
	return false
}

func (f *FormField) handleTextAreaKey(key string) bool {
	switch key {
	case "backspace":
		if len(f.Value) > 0 {
			f.Value = f.Value[:len(f.Value)-1]
		}
		return true
	case "enter":
		if f.MaxLen == 0 || len(f.Value) < f.MaxLen {
			f.Value += "\n"
		}
		return true
	default:
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			if f.MaxLen > 0 && len(f.Value) >= f.MaxLen {
				return false
			}
			f.Value += key
			return true
		}
	}
	return false
}

func (f *FormField) handleRadioKey(key string) bool {
	switch key {
	case "up", "k":
		if f.RadioIndex > 0 {
			f.RadioIndex--
		}
		return true
	case "down", "j":
		if f.RadioIndex < len(f.Options)-1 {
			f.RadioIndex++
		}
		return true
	case " ", "enter":
		return true // already selected by index
	}
	return false
}

func (f *FormField) handleCheckboxKey(key string) bool {
	switch key {
	case " ", "enter":
		// Toggle current (use RadioIndex as cursor for checkboxes)
		if f.RadioIndex >= 0 && f.RadioIndex < len(f.Options) {
			f.Selected[f.RadioIndex] = !f.Selected[f.RadioIndex]
		}
		return true
	case "left":
		if f.RadioIndex > 0 {
			f.RadioIndex--
		}
		return true
	case "right":
		if f.RadioIndex < len(f.Options)-1 {
			f.RadioIndex++
		}
		return true
	}
	return false
}
