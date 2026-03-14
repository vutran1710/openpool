package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

type SubmitMsg struct {
	Value     string
	IsCommand bool
}

type Input struct {
	textInput textinput.Model
	Width     int
	Hint      string
}

func NewInput(placeholder string) Input {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Prompt = theme.Cursor()
	ti.TextStyle = lipgloss.NewStyle().Foreground(theme.Text)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(theme.Dim)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(theme.Pink)
	ti.Focus()
	ti.CharLimit = 256

	return Input{
		textInput: ti,
		Hint:      "Type / for commands",
	}
}

func (i Input) Update(msg tea.Msg) (Input, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEnter {
			val := strings.TrimSpace(i.textInput.Value())
			if val != "" {
				i.textInput.SetValue("")
				return i, func() tea.Msg {
					return SubmitMsg{
						Value:     val,
						IsCommand: strings.HasPrefix(val, "/"),
					}
				}
			}
			return i, nil
		}
	}

	var cmd tea.Cmd
	i.textInput, cmd = i.textInput.Update(msg)
	return i, cmd
}

func (i Input) View() string {
	separator := lipgloss.NewStyle().
		Width(i.Width).
		Foreground(theme.Border).
		Render(Repeat("─", i.Width))

	input := lipgloss.NewStyle().
		Padding(0, 2).
		Width(i.Width).
		Render(i.textInput.View())

	hint := lipgloss.NewStyle().
		Padding(0, 2).
		Width(i.Width).
		Render(theme.DimStyle.Render(i.Hint))

	return separator + "\n" + input + "\n" + hint
}

func (i *Input) Focus() {
	i.textInput.Focus()
}

func (i *Input) Blur() {
	i.textInput.Blur()
}

func (i *Input) SetPlaceholder(s string) {
	i.textInput.Placeholder = s
}
