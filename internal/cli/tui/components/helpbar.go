package components

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/openpool/internal/cli/tui/theme"
)

// KeyBind wraps key.Binding for convenience.
type KeyBind struct {
	Key  string
	Desc string
}

// toBinding converts a KeyBind to a bubbles key.Binding.
func (k KeyBind) toBinding() key.Binding {
	return key.NewBinding(
		key.WithKeys(k.Key),
		key.WithHelp(k.Key, k.Desc),
	)
}

// helpKeyMap implements help.KeyMap for our bindings.
type helpKeyMap struct {
	bindings []key.Binding
}

func (h helpKeyMap) ShortHelp() []key.Binding { return h.bindings }
func (h helpKeyMap) FullHelp() [][]key.Binding { return [][]key.Binding{h.bindings} }

// HelpBar renders contextual keybinding help using bubbles/help.
type HelpBar struct {
	model help.Model
	keys  helpKeyMap
	Width int
}

func NewHelpBar(bindings ...KeyBind) HelpBar {
	m := help.New()
	m.Styles.ShortKey = lipgloss.NewStyle().Foreground(theme.Violet).Bold(true)
	m.Styles.ShortDesc = lipgloss.NewStyle().Foreground(theme.Dim)
	m.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(theme.Dim)
	m.ShortSeparator = "  ·  "

	var keys []key.Binding
	for _, b := range bindings {
		keys = append(keys, b.toBinding())
	}

	return HelpBar{
		model: m,
		keys:  helpKeyMap{bindings: keys},
	}
}

func (h HelpBar) View() string {
	h.model.Width = h.Width
	return lipgloss.NewStyle().
		Padding(0, 2).
		Width(h.Width).
		Render(h.model.ShortHelpView(h.keys.bindings))
}
