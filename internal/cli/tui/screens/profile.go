package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/openpool/internal/cli/config"
	"github.com/vutran1710/openpool/internal/cli/tui/components"
	"github.com/vutran1710/openpool/internal/cli/tui/theme"
	"github.com/vutran1710/openpool/internal/schema"
)

type profileLoadedMsg struct {
	profile map[string]any
	schema  *schema.PoolSchema
	err     error
}

type ProfileScreen struct {
	poolName string
	profile  map[string]any
	schema   *schema.PoolSchema
	loaded   bool
	err      string
	Width    int
	Height   int
}

func NewProfileScreen() ProfileScreen {
	return ProfileScreen{}
}

func (s ProfileScreen) IsLoaded() bool {
	return s.loaded
}

func (s ProfileScreen) Update(msg tea.Msg) (ProfileScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.Width = msg.Width
		s.Height = msg.Height
		return s, nil

	case profileLoadedMsg:
		s.loaded = true
		if msg.err != nil {
			s.err = msg.err.Error()
		} else {
			s.profile = msg.profile
			s.schema = msg.schema
		}
		return s, nil
	}
	return s, nil
}

// LoadProfileCmd loads the pool profile for the active pool.
func LoadProfileCmd(poolName string) tea.Cmd {
	return func() tea.Msg {
		if poolName == "" {
			return profileLoadedMsg{err: fmt.Errorf("no active pool")}
		}

		profilePath := schema.ProfilePath(config.Dir(), poolName)
		data, err := os.ReadFile(profilePath)
		if err != nil {
			return profileLoadedMsg{err: fmt.Errorf("no profile found — join a pool first")}
		}

		var profile map[string]any
		if err := json.Unmarshal(data, &profile); err != nil {
			return profileLoadedMsg{err: fmt.Errorf("invalid profile: %w", err)}
		}

		// Try loading schema for field metadata
		cfg, _ := config.Load()
		var poolSchema *schema.PoolSchema
		if cfg != nil {
			for _, p := range cfg.Pools {
				if p.Name == poolName {
					rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/pool.yaml", p.Repo)
					poolSchema, _ = schema.Load(rawURL)
					break
				}
			}
		}

		return profileLoadedMsg{profile: profile, schema: poolSchema}
	}
}

func (s ProfileScreen) View() string {
	if s.poolName == "" {
		return components.ScreenLayout("Profile", "",
			theme.DimStyle.Render("  No active pool. Join a pool first."))
	}

	if !s.loaded {
		return components.ScreenLayout("Profile", components.DimHints(s.poolName),
			theme.DimStyle.Render("  Loading..."))
	}

	if s.err != "" {
		return components.ScreenLayout("Profile", components.DimHints(s.poolName),
			theme.RedStyle.Render("  "+s.err))
	}

	if s.profile == nil {
		return components.ScreenLayout("Profile", components.DimHints(s.poolName),
			theme.DimStyle.Render("  No profile yet. Join a pool to create one."))
	}

	body := lipgloss.NewStyle().PaddingLeft(2).Render(s.renderProfile())
	return components.ScreenLayout("Profile", components.DimHints(s.poolName), body)
}

func (s ProfileScreen) renderProfile() string {
	var lines []string

	// Sort keys for stable ordering
	keys := make([]string, 0, len(s.profile))
	for key := range s.profile {
		if key != "_role" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	// Render each profile field
	for _, key := range keys {
		val := s.profile[key]
		label := titleCase(key)

		// Check schema for visibility
		if s.schema != nil {
			if attr, ok := s.schema.Profile[key]; ok {
				if !attr.IsPublic() {
					label += " \U0001F512"
				}
			}
		}

		var valStr string
		switch v := val.(type) {
		case string:
			if v == "" {
				valStr = theme.DimStyle.Render("(empty)")
			} else {
				valStr = theme.TextStyle.Render(v)
			}
		case float64:
			valStr = theme.TextStyle.Render(fmt.Sprintf("%g", v))
		case []any:
			var tags []string
			for _, item := range v {
				if str, ok := item.(string); ok {
					tags = append(tags, theme.AccentStyle.Render("#"+str))
				}
			}
			if len(tags) > 0 {
				valStr = strings.Join(tags, "  ")
			} else {
				valStr = theme.DimStyle.Render("(none)")
			}
		default:
			valStr = theme.DimStyle.Render(fmt.Sprintf("%v", val))
		}

		lines = append(lines, theme.BoldStyle.Render(label))
		lines = append(lines, valStr)
		lines = append(lines, "")
	}

	// Show role if set
	if role, ok := s.profile["_role"].(string); ok && role != "" {
		lines = append(lines, theme.DimStyle.Render("Role: ")+theme.TextStyle.Render(role))
	}

	return strings.Join(lines, "\n")
}

func (s *ProfileScreen) SetPool(poolName string) {
	s.poolName = poolName
	s.loaded = false
	s.profile = nil
	s.schema = nil
	s.err = ""
}

func (s ProfileScreen) HelpBindings() []components.KeyBind {
	return []components.KeyBind{
		{Key: "esc", Desc: "back"},
	}
}
