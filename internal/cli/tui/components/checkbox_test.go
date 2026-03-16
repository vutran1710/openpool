package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newTestCheckbox() Checkbox {
	return NewCheckbox("Test", []CheckboxItem{
		{ID: "a", Label: "Alpha", Value: "val-a", Checked: true},
		{ID: "b", Label: "Beta", Value: "val-b", Checked: false},
		{ID: "c", Label: "Gamma", Value: "val-c", Checked: false},
	})
}

func TestCheckbox_InitialState(t *testing.T) {
	c := newTestCheckbox()
	if c.Cursor != 0 {
		t.Errorf("expected cursor 0, got %d", c.Cursor)
	}
	if !c.Items[0].Checked {
		t.Error("expected first item checked")
	}
}

func TestCheckbox_Navigation(t *testing.T) {
	c := newTestCheckbox()

	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyDown})
	if c.Cursor != 1 {
		t.Errorf("expected cursor 1, got %d", c.Cursor)
	}

	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyDown})
	if c.Cursor != 2 {
		t.Errorf("expected cursor 2, got %d", c.Cursor)
	}

	// Can't go past end
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyDown})
	if c.Cursor != 2 {
		t.Errorf("expected cursor still 2, got %d", c.Cursor)
	}

	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyUp})
	if c.Cursor != 1 {
		t.Errorf("expected cursor 1, got %d", c.Cursor)
	}
}

func TestCheckbox_Toggle(t *testing.T) {
	c := newTestCheckbox()

	// Toggle first item off
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if c.Items[0].Checked {
		t.Error("expected first item unchecked after toggle")
	}

	// Toggle it back on
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if !c.Items[0].Checked {
		t.Error("expected first item checked after second toggle")
	}
}

func TestCheckbox_SelectAll(t *testing.T) {
	c := newTestCheckbox()

	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	for i, item := range c.Items {
		if !item.Checked {
			t.Errorf("item %d should be checked after select all", i)
		}
	}
}

func TestCheckbox_SelectNone(t *testing.T) {
	c := newTestCheckbox()

	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	for i, item := range c.Items {
		if item.Checked {
			t.Errorf("item %d should be unchecked after select none", i)
		}
	}
}

func TestCheckbox_Submit(t *testing.T) {
	c := newTestCheckbox()
	// Items[0] is checked, rest are not

	_, cmd := c.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected submit command")
	}

	msg := cmd()
	submitMsg, ok := msg.(CheckboxSubmitMsg)
	if !ok {
		t.Fatalf("expected CheckboxSubmitMsg, got %T", msg)
	}

	if len(submitMsg.Selected) != 1 {
		t.Errorf("expected 1 selected, got %d", len(submitMsg.Selected))
	}
	if submitMsg.Selected[0].ID != "a" {
		t.Errorf("expected 'a' selected, got %s", submitMsg.Selected[0].ID)
	}
}

func TestCheckbox_SelectedIDs(t *testing.T) {
	c := newTestCheckbox()
	c.Items[1].Checked = true

	ids := c.SelectedIDs()
	if len(ids) != 2 {
		t.Errorf("expected 2 selected, got %d", len(ids))
	}
}

func TestCheckbox_SelectedValues(t *testing.T) {
	c := newTestCheckbox()
	vals := c.SelectedValues()
	if len(vals) != 1 || vals[0] != "val-a" {
		t.Errorf("unexpected values: %v", vals)
	}
}
