package components

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/openpool/internal/cli/tui/theme"
)

// Stepper is a range input with ◀ value ▶ display.
// Left/right arrows to decrement/increment. Bounds enforced.
type Stepper struct {
	Name    string
	Label   string
	Value   int
	Min     int
	Max     int
	Step    int // increment size, default 1
	Focused bool
}

// NewStepper creates a new stepper with the given bounds.
func NewStepper(name, label string, value, min, max int) Stepper {
	if value < min {
		value = min
	}
	if value > max {
		value = max
	}
	return Stepper{
		Name:  name,
		Label: label,
		Value: value,
		Min:   min,
		Max:   max,
		Step:  1,
	}
}

// Update handles key messages for the stepper.
func (s Stepper) Update(msg tea.Msg) (Stepper, tea.Cmd) {
	if !s.Focused {
		return s, nil
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	switch km.String() {
	case "left", "h":
		s.Value -= s.Step
		if s.Value < s.Min {
			s.Value = s.Min
		}
	case "right", "l":
		s.Value += s.Step
		if s.Value > s.Max {
			s.Value = s.Max
		}
	}
	return s, nil
}

// View renders the stepper.
func (s Stepper) View() string {
	valStr := fmt.Sprintf("%d", s.Value)
	if s.Focused {
		label := theme.BrandStyle.Render("▸ " + s.Label)
		arrows := theme.BrandStyle.Render("◀") + " " + theme.TextStyle.Render(valStr) + " " + theme.BrandStyle.Render("▶")
		return label + "\n  " + arrows
	}
	label := theme.BoldStyle.Render(s.Label)
	return label + "\n  " + theme.DimStyle.Render(valStr)
}

// GetValue returns the stepper's current value.
func (s Stepper) GetValue() int {
	return s.Value
}
