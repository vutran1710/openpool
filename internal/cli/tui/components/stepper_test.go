package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestStepper_InitialValue(t *testing.T) {
	s := NewStepper("age", "Age", 28, 18, 99)
	if s.GetValue() != 28 {
		t.Errorf("expected 28, got %d", s.GetValue())
	}
	if s.Name != "age" {
		t.Errorf("expected name 'age', got %q", s.Name)
	}
	if s.Label != "Age" {
		t.Errorf("expected label 'Age', got %q", s.Label)
	}
	if s.Step != 1 {
		t.Errorf("expected default step 1, got %d", s.Step)
	}
}

func TestStepper_InitialValueClamped(t *testing.T) {
	s := NewStepper("age", "Age", 5, 18, 99)
	if s.GetValue() != 18 {
		t.Errorf("expected clamped to 18, got %d", s.GetValue())
	}

	s2 := NewStepper("age", "Age", 200, 18, 99)
	if s2.GetValue() != 99 {
		t.Errorf("expected clamped to 99, got %d", s2.GetValue())
	}
}

func TestStepper_Increment(t *testing.T) {
	s := NewStepper("age", "Age", 28, 18, 99)
	s.Focused = true

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRight})
	if s.GetValue() != 29 {
		t.Errorf("expected 29, got %d", s.GetValue())
	}

	// Also test 'l' key
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if s.GetValue() != 30 {
		t.Errorf("expected 30, got %d", s.GetValue())
	}
}

func TestStepper_Decrement(t *testing.T) {
	s := NewStepper("age", "Age", 28, 18, 99)
	s.Focused = true

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if s.GetValue() != 27 {
		t.Errorf("expected 27, got %d", s.GetValue())
	}

	// Also test 'h' key
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if s.GetValue() != 26 {
		t.Errorf("expected 26, got %d", s.GetValue())
	}
}

func TestStepper_ClampAtMax(t *testing.T) {
	s := NewStepper("age", "Age", 99, 18, 99)
	s.Focused = true

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRight})
	if s.GetValue() != 99 {
		t.Errorf("expected clamped at 99, got %d", s.GetValue())
	}
}

func TestStepper_ClampAtMin(t *testing.T) {
	s := NewStepper("age", "Age", 18, 18, 99)
	s.Focused = true

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if s.GetValue() != 18 {
		t.Errorf("expected clamped at 18, got %d", s.GetValue())
	}
}

func TestStepper_CustomStep(t *testing.T) {
	s := NewStepper("score", "Score", 50, 0, 100)
	s.Step = 5
	s.Focused = true

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRight})
	if s.GetValue() != 55 {
		t.Errorf("expected 55 with step=5, got %d", s.GetValue())
	}

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if s.GetValue() != 50 {
		t.Errorf("expected 50 with step=5, got %d", s.GetValue())
	}
}

func TestStepper_CustomStepClamp(t *testing.T) {
	s := NewStepper("score", "Score", 98, 0, 100)
	s.Step = 5
	s.Focused = true

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRight})
	if s.GetValue() != 100 {
		t.Errorf("expected clamped at 100, got %d", s.GetValue())
	}
}

func TestStepper_UnfocusedIgnoresKeys(t *testing.T) {
	s := NewStepper("age", "Age", 28, 18, 99)
	s.Focused = false

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRight})
	if s.GetValue() != 28 {
		t.Errorf("expected unchanged 28 when unfocused, got %d", s.GetValue())
	}
}

func TestStepper_ViewFocused(t *testing.T) {
	s := NewStepper("age", "Age", 28, 18, 99)
	s.Focused = true

	view := s.View()
	if !strings.Contains(view, "Age") {
		t.Error("expected view to contain label 'Age'")
	}
	if !strings.Contains(view, "28") {
		t.Error("expected view to contain value '28'")
	}
	if !strings.Contains(view, "◀") || !strings.Contains(view, "▶") {
		t.Error("expected focused view to contain arrows ◀ ▶")
	}
}

func TestStepper_ViewUnfocused(t *testing.T) {
	s := NewStepper("age", "Age", 28, 18, 99)
	s.Focused = false

	view := s.View()
	if !strings.Contains(view, "Age") {
		t.Error("expected view to contain label 'Age'")
	}
	if !strings.Contains(view, "28") {
		t.Error("expected view to contain value '28'")
	}
	if strings.Contains(view, "◀") || strings.Contains(view, "▶") {
		t.Error("expected unfocused view to NOT contain arrows")
	}
}
