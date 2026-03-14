package components

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

type ToastLevel int

const (
	ToastInfo ToastLevel = iota
	ToastSuccess
	ToastWarning
	ToastError
)

type ToastMsg struct {
	Text  string
	Level ToastLevel
}

type ToastClearMsg struct{}

type Toast struct {
	Text    string
	Level   ToastLevel
	Visible bool
	Width   int
}

func NewToast() Toast {
	return Toast{}
}

func (t Toast) Update(msg tea.Msg) (Toast, tea.Cmd) {
	switch msg := msg.(type) {
	case ToastMsg:
		t.Text = msg.Text
		t.Level = msg.Level
		t.Visible = true
		return t, tea.Tick(3*time.Second, func(time.Time) tea.Msg {
			return ToastClearMsg{}
		})
	case ToastClearMsg:
		t.Visible = false
	}
	return t, nil
}

func (t Toast) View() string {
	if !t.Visible || t.Text == "" {
		return ""
	}

	var style = theme.DimStyle
	prefix := "  "

	switch t.Level {
	case ToastSuccess:
		style = theme.GreenStyle
		prefix = "✓ "
	case ToastWarning:
		style = theme.AmberStyle
		prefix = "⚠ "
	case ToastError:
		style = theme.RedStyle
		prefix = "✗ "
	case ToastInfo:
		style = theme.AccentStyle
		prefix = "● "
	}

	return "\n  " + style.Render(prefix+t.Text)
}
