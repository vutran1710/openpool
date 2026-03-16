package components

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

// CheckboxItem represents a single toggleable item.
type CheckboxItem struct {
	ID      string
	Label   string
	Value   string // display value (truncated)
	Checked bool
	Muted   bool // true if value is empty/unavailable
}

// CheckboxSubmitMsg is emitted when the user presses enter on a checkbox list.
type CheckboxSubmitMsg struct {
	Selected []CheckboxItem
}

// Checkbox is a multi-select checkbox list component.
type Checkbox struct {
	Items    []CheckboxItem
	Cursor   int
	Title    string
	Subtitle string
	Width    int
}

func NewCheckbox(title string, items []CheckboxItem) Checkbox {
	return Checkbox{
		Title: title,
		Items: items,
	}
}

func (c Checkbox) Update(msg tea.Msg) (Checkbox, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if c.Cursor > 0 {
				c.Cursor--
			}
		case "down", "j":
			if c.Cursor < len(c.Items)-1 {
				c.Cursor++
			}
		case " ":
			if c.Cursor < len(c.Items) && !c.Items[c.Cursor].Muted {
				c.Items[c.Cursor].Checked = !c.Items[c.Cursor].Checked
			}
		case "enter":
			var selected []CheckboxItem
			for _, item := range c.Items {
				if item.Checked {
					selected = append(selected, item)
				}
			}
			return c, func() tea.Msg {
				return CheckboxSubmitMsg{Selected: selected}
			}
		case "a":
			// Select all (except muted/empty)
			for i := range c.Items {
				if !c.Items[i].Muted {
					c.Items[i].Checked = true
				}
			}
		case "n":
			// Select none
			for i := range c.Items {
				c.Items[i].Checked = false
			}
		}
	}
	return c, nil
}

func (c Checkbox) View() string {
	var out string

	if c.Title != "" {
		out += theme.BoldStyle.Render(c.Title) + "\n"
		if c.Subtitle != "" {
			out += theme.DimStyle.Render(c.Subtitle) + "\n"
		}
		out += "\n"
	}

	// Find max label width for alignment
	maxLabel := 0
	for _, item := range c.Items {
		if len(item.Label) > maxLabel {
			maxLabel = len(item.Label)
		}
	}
	labelCol := lipgloss.NewStyle().Width(maxLabel + 1)

	for i, item := range c.Items {
		cursor := "  "
		if i == c.Cursor {
			cursor = theme.Cursor()
		}

		check := theme.DimStyle.Render("[ ]")
		if item.Checked {
			check = theme.GreenStyle.Render("[✓]")
		}

		label := theme.TextStyle.Render(labelCol.Render(item.Label))
		if i == c.Cursor {
			label = theme.ActiveItem.Render(labelCol.Render(item.Label))
		}

		value := theme.TextStyle.Render(item.Value)
		if item.Muted {
			value = theme.DimStyle.Render(item.Value)
		}

		out += fmt.Sprintf("  %s%s %s %s\n", cursor, check, label, value)
	}

	out += "\n"
	out += theme.DimStyle.Render("  space toggle · a all · n none · enter submit")

	return out
}

// SelectedIDs returns the IDs of all checked items.
func (c Checkbox) SelectedIDs() []string {
	var ids []string
	for _, item := range c.Items {
		if item.Checked {
			ids = append(ids, item.ID)
		}
	}
	return ids
}

// SelectedValues returns the Values of all checked items. Falls back to ID if Value is empty.
func (c Checkbox) SelectedValues() []string {
	var vals []string
	for _, item := range c.Items {
		if item.Checked {
			v := item.Value
			if v == "" {
				v = item.ID
			}
			vals = append(vals, v)
		}
	}
	return vals
}
