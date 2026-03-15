package components

import (
	"fmt"
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

type commandEntry struct {
	Cmd  string
	Desc string
}

var commands = []commandEntry{
	{Cmd: "/home", Desc: "go to home screen"},
	{Cmd: "/discover", Desc: "find new people"},
	{Cmd: "/matches", Desc: "view your matches"},
	{Cmd: "/pools", Desc: "browse & manage pools"},
	{Cmd: "/quit", Desc: "exit the app"},
}

type Input struct {
	textInput    textinput.Model
	Width        int
	showPalette  bool
	paletteCursor int
	filtered     []commandEntry
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
	}
}

func (i Input) Update(msg tea.Msg) (Input, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if i.showPalette {
			switch msg.String() {
			case "up", "shift+tab":
				if i.paletteCursor > 0 {
					i.paletteCursor--
				}
				return i, nil
			case "down", "tab":
				if i.paletteCursor < len(i.filtered)-1 {
					i.paletteCursor++
				}
				return i, nil
			case "enter":
				if len(i.filtered) > 0 && i.paletteCursor < len(i.filtered) {
					selected := i.filtered[i.paletteCursor].Cmd
					i.textInput.SetValue("")
					i.showPalette = false
					i.paletteCursor = 0
					return i, func() tea.Msg {
						return SubmitMsg{Value: selected, IsCommand: true}
					}
				}
				return i, nil
			case "esc":
				i.textInput.SetValue("")
				i.showPalette = false
				i.paletteCursor = 0
				return i, nil
			}
		}

		if msg.Type == tea.KeyEnter {
			val := strings.TrimSpace(i.textInput.Value())
			if val != "" {
				i.textInput.SetValue("")
				i.showPalette = false
				i.paletteCursor = 0
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

	// Update palette visibility and filtering
	val := i.textInput.Value()
	if strings.HasPrefix(val, "/") {
		i.showPalette = true
		i.filtered = filterCommands(val)
		if i.paletteCursor >= len(i.filtered) {
			i.paletteCursor = 0
		}
	} else {
		i.showPalette = false
		i.paletteCursor = 0
	}

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

	return separator + "\n" + input
}

// PaletteView renders the command palette (shown above the input area).
func (i Input) PaletteView() string {
	if !i.showPalette || len(i.filtered) == 0 {
		return ""
	}

	paletteStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Padding(0, 1).
		Width(40)

	var items string
	for idx, c := range i.filtered {
		cursor := "  "
		cmdStyle := theme.DimStyle
		descStyle := theme.DimStyle
		if idx == i.paletteCursor {
			cursor = theme.Cursor()
			cmdStyle = theme.ActiveItem
			descStyle = theme.TextStyle
		}
		items += fmt.Sprintf("%s%s  %s\n", cursor, cmdStyle.Render(c.Cmd), descStyle.Render(c.Desc))
	}

	return lipgloss.NewStyle().
		Padding(0, 2).
		Render(paletteStyle.Render(strings.TrimRight(items, "\n")))
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

func filterCommands(query string) []commandEntry {
	query = strings.ToLower(query)
	var results []commandEntry
	for _, c := range commands {
		if strings.HasPrefix(c.Cmd, query) || query == "/" {
			results = append(results, c)
		}
	}
	return results
}
