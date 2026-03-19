package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
	gh "github.com/vutran1710/dating-dev/internal/github"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
)

// ProfileFormMsg carries the loaded schema to the form.
type ProfileFormMsg struct {
	Schema   *gh.PoolSchema
	Profile  map[string]any // existing profile data (if editing)
	PoolName string
	Err      error
}

// ProfileFormSavedMsg signals the form was saved.
type ProfileFormSavedMsg struct {
	PoolName string
	Err      error
}

// ProfileFormScreen is an interactive form for creating/editing a pool profile.
type ProfileFormScreen struct {
	fields   []components.FormField
	cursor   int // which field is focused
	schema   *gh.PoolSchema
	poolName string
	Loading  bool
	saved    bool
	err      error
	Width    int
	scrollY  int
}

func NewProfileFormScreen() ProfileFormScreen {
	return ProfileFormScreen{Loading: true}
}

// LoadProfileFormCmd loads the pool schema and existing profile.
func LoadProfileFormCmd(poolName string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return ProfileFormMsg{Err: err}
		}

		var pool *config.PoolConfig
		for i := range cfg.Pools {
			if cfg.Pools[i].Name == poolName {
				pool = &cfg.Pools[i]
				break
			}
		}
		if pool == nil {
			return ProfileFormMsg{Err: fmt.Errorf("pool not found: %s", poolName)}
		}

		// Load schema from pool repo
		var schema *gh.PoolSchema
		repo, err := gitrepo.Clone(gitrepo.EnsureGitURL(pool.Repo))
		if err == nil {
			repo.Sync()
			data, err := os.ReadFile(filepath.Join(repo.LocalDir, "pool.json"))
			if err == nil {
				var manifest gh.PoolManifest
				if json.Unmarshal(data, &manifest) == nil && manifest.Schema != nil {
					schema = manifest.Schema
				}
			}
		}

		// Load existing profile
		var profile map[string]any
		profilePath := config.PoolProfilePath(poolName)
		if data, err := os.ReadFile(profilePath); err == nil {
			json.Unmarshal(data, &profile)
		}

		return ProfileFormMsg{
			Schema:   schema,
			Profile:  profile,
			PoolName: poolName,
		}
	}
}

func (s ProfileFormScreen) Update(msg tea.Msg) (ProfileFormScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case ProfileFormMsg:
		s.Loading = false
		if msg.Err != nil {
			s.err = msg.Err
			return s, nil
		}
		s.schema = msg.Schema
		s.poolName = msg.PoolName
		s.fields = buildFormFields(msg.Schema, msg.Profile)
		if len(s.fields) > 0 {
			s.fields[0].Focused = true
		}
		return s, nil

	case ProfileFormSavedMsg:
		if msg.Err != nil {
			s.err = msg.Err
		} else {
			s.saved = true
		}
		return s, nil

	case tea.KeyMsg:
		if s.saved {
			return s, nil
		}

		switch msg.String() {
		case "tab", "down":
			if !s.currentFieldHandlesKey("down") {
				s.focusNext()
				return s, nil
			}
		case "shift+tab", "up":
			if !s.currentFieldHandlesKey("up") {
				s.focusPrev()
				return s, nil
			}
		case "ctrl+s":
			return s, s.save()
		}

		// Forward to focused field
		if s.cursor >= 0 && s.cursor < len(s.fields) {
			s.fields[s.cursor].HandleKey(msg.String())
		}
	}
	return s, nil
}

func (s *ProfileFormScreen) currentFieldHandlesKey(key string) bool {
	if s.cursor < 0 || s.cursor >= len(s.fields) {
		return false
	}
	f := s.fields[s.cursor]
	return f.Type == components.FieldRadio || f.Type == components.FieldCheckbox
}

func (s *ProfileFormScreen) focusNext() {
	if s.cursor < len(s.fields)-1 {
		s.fields[s.cursor].Focused = false
		s.cursor++
		s.fields[s.cursor].Focused = true
	}
}

func (s *ProfileFormScreen) focusPrev() {
	if s.cursor > 0 {
		s.fields[s.cursor].Focused = false
		s.cursor--
		s.fields[s.cursor].Focused = true
	}
}

func (s ProfileFormScreen) save() tea.Cmd {
	poolName := s.poolName
	fields := s.fields
	return func() tea.Msg {
		profile := make(map[string]any)
		for _, f := range fields {
			profile[f.Name] = f.GetValue()
		}

		data, err := json.MarshalIndent(profile, "", "  ")
		if err != nil {
			return ProfileFormSavedMsg{Err: err}
		}

		path := config.PoolProfilePath(poolName)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return ProfileFormSavedMsg{Err: err}
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			return ProfileFormSavedMsg{Err: err}
		}

		return ProfileFormSavedMsg{PoolName: poolName}
	}
}

func (s ProfileFormScreen) View() string {
	if s.Loading {
		return components.ScreenLayout("Profile", components.DimHints("loading..."), "")
	}
	if s.err != nil {
		return components.ScreenLayout("Profile", "", theme.RedStyle.Render(s.err.Error()))
	}
	if s.saved {
		body := theme.GreenStyle.Render("✓ Profile saved!") + "\n\n" +
			theme.DimStyle.Render("  Run: dating pool join "+s.poolName+" to publish")
		return components.ScreenLayout("Profile", components.DimHints("saved"), body)
	}

	// Render all fields
	var lines []string
	for _, f := range s.fields {
		lines = append(lines, f.Render(s.Width-4))
		lines = append(lines, "") // spacing between fields
	}

	// Footer
	lines = append(lines, theme.DimStyle.Render("  tab/↓ next field  ·  shift+tab/↑ prev  ·  ctrl+s save"))

	body := strings.Join(lines, "\n")
	return components.ScreenLayout("Profile",
		components.DimHints("edit", "ctrl+s to save"),
		body)
}

func (s ProfileFormScreen) HelpBindings() []components.KeyBind {
	return []components.KeyBind{
		{Key: "tab", Desc: "next field"},
		{Key: "shift+tab", Desc: "prev field"},
		{Key: "ctrl+s", Desc: "save"},
		{Key: "esc", Desc: "back"},
	}
}

// buildFormFields creates form fields from the schema + existing profile data.
func buildFormFields(schema *gh.PoolSchema, profile map[string]any) []components.FormField {
	if profile == nil {
		profile = make(map[string]any)
	}

	var fields []components.FormField

	// Display name (always present)
	displayName, _ := profile["display_name"].(string)
	fields = append(fields, components.NewTextField("display_name", "Display Name", displayName))

	// Bio/slogan (one-liner)
	bio, _ := profile["bio"].(string)
	fields = append(fields, components.NewTextField("bio", "Bio (one-liner)", bio))

	// About (long text, 1000 char max)
	about, _ := profile["about"].(string)
	fields = append(fields, components.NewTextAreaField("about", "About", about, 1000))

	// Schema fields
	if schema != nil {
		for _, sf := range schema.Fields {
			// Skip fields we already handle above
			if sf.Name == "display_name" || sf.Name == "bio" || sf.Name == "about" {
				continue
			}

			switch sf.Type {
			case "enum":
				selected := 0
				if v, ok := profile[sf.Name].(string); ok {
					for i, opt := range sf.Values {
						if opt == v {
							selected = i
							break
						}
					}
				}
				fields = append(fields, components.NewRadioField(sf.Name, sf.Name, sf.Values, selected))

			case "multi":
				var selected []string
				switch v := profile[sf.Name].(type) {
				case []string:
					selected = v
				case []any:
					for _, item := range v {
						if s, ok := item.(string); ok {
							selected = append(selected, s)
						}
					}
				}
				fields = append(fields, components.NewCheckboxField(sf.Name, sf.Name, sf.Values, selected))

			case "range":
				val := 0
				switch v := profile[sf.Name].(type) {
				case float64:
					val = int(v)
				case int:
					val = v
				}
				min, max := 0, 100
				if sf.Min != nil {
					min = *sf.Min
				}
				if sf.Max != nil {
					max = *sf.Max
				}
				fields = append(fields, components.NewNumberField(sf.Name, sf.Name, val, min, max))
			}
		}
	}

	return fields
}
