package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
	"github.com/vutran1710/dating-dev/internal/schema"
)

// PoolOnboardStep represents the current step in the onboarding wizard.
type PoolOnboardStep int

const (
	PoolOnboardRole PoolOnboardStep = iota // role selection (if multiple)
	PoolOnboardForm                        // fill profile fields
	PoolOnboardDone                        // complete
)

// PoolOnboardDoneMsg is emitted when onboarding completes.
type PoolOnboardDoneMsg struct {
	PoolName string
	Role     string
	Profile  map[string]any
}

// PoolOnboardScreen implements a step-by-step wizard for pool registration.
type PoolOnboardScreen struct {
	poolName string
	schema   *schema.PoolSchema
	step     PoolOnboardStep
	editMode bool           // true when editing existing profile
	existing map[string]any // existing profile data for pre-fill

	// Role selection
	roles      []string
	roleCursor int

	// Form
	formFields []components.FormField
	steppers   []components.Stepper
	fieldIdx   int
	totalCount int // total number of navigable fields (formFields + steppers)

	// Result
	selectedRole string

	width, height int
	err           error
}

// NewPoolOnboardScreen creates a new onboarding screen for a pool.
func NewPoolOnboardScreen(poolName string, s *schema.PoolSchema, width, height int) PoolOnboardScreen {
	return newPoolOnboardScreen(poolName, s, width, height, false, nil)
}

// NewPoolEditScreen creates a screen for editing an existing pool profile.
func NewPoolEditScreen(poolName string, s *schema.PoolSchema, width, height int, existing map[string]any) PoolOnboardScreen {
	return newPoolOnboardScreen(poolName, s, width, height, true, existing)
}

func newPoolOnboardScreen(poolName string, s *schema.PoolSchema, width, height int, editMode bool, existing map[string]any) PoolOnboardScreen {
	roles, _ := s.ParsedRoles()

	screen := PoolOnboardScreen{
		poolName: poolName,
		schema:   s,
		width:    width,
		height:   height,
		roles:    roles,
		editMode: editMode,
		existing: existing,
	}

	// In edit mode, skip role selection — go straight to form
	if editMode {
		if role, ok := existing["_role"].(string); ok {
			screen.selectedRole = role
		}
		screen.step = PoolOnboardForm
		screen.initForm()
		return screen
	}

	if len(roles) <= 1 {
		if len(roles) == 1 {
			screen.selectedRole = roles[0]
		}
		screen.step = PoolOnboardForm
		screen.initForm()
	} else {
		screen.step = PoolOnboardRole
	}

	return screen
}

func (s *PoolOnboardScreen) initForm() {
	fields, steppers, err := schema.BuildFormFields(s.schema.Profile, s.existing, "")
	if err != nil {
		s.err = err
		return
	}
	s.formFields = fields
	s.steppers = steppers
	s.totalCount = len(fields) + len(steppers)
	s.fieldIdx = 0
	s.updateFieldFocus()
}

// updateFieldFocus sets the Focused flag on the currently selected field.
func (s *PoolOnboardScreen) updateFieldFocus() {
	for i := range s.formFields {
		s.formFields[i].Focused = false
	}
	for i := range s.steppers {
		s.steppers[i].Focused = false
	}

	if s.fieldIdx < len(s.formFields) {
		s.formFields[s.fieldIdx].Focused = true
	} else {
		stepperIdx := s.fieldIdx - len(s.formFields)
		if stepperIdx >= 0 && stepperIdx < len(s.steppers) {
			s.steppers[stepperIdx].Focused = true
		}
	}
}

// Update handles messages for the onboarding screen.
func (s PoolOnboardScreen) Update(msg tea.Msg) (PoolOnboardScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		return s, nil

	case tea.KeyMsg:
		switch s.step {
		case PoolOnboardRole:
			return s.updateRole(msg)
		case PoolOnboardForm:
			return s.updateForm(msg)
		case PoolOnboardDone:
			if msg.String() == "enter" {
				return s, func() tea.Msg {
					return PoolOnboardDoneMsg{
						PoolName: s.poolName,
						Role:     s.selectedRole,
						Profile:  s.collectProfile(),
					}
				}
			}
		}
	}

	return s, nil
}

func (s PoolOnboardScreen) updateRole(msg tea.KeyMsg) (PoolOnboardScreen, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if s.roleCursor > 0 {
			s.roleCursor--
		}
	case "down", "j":
		if s.roleCursor < len(s.roles)-1 {
			s.roleCursor++
		}
	case "enter":
		s.selectedRole = s.roles[s.roleCursor]
		s.step = PoolOnboardForm
		s.initForm()
	}
	return s, nil
}

func (s PoolOnboardScreen) updateForm(msg tea.KeyMsg) (PoolOnboardScreen, tea.Cmd) {
	key := msg.String()

	// Only use tab/shift+tab for navigation in form (j/k conflict with text input)
	switch key {
	case "tab", "down":
		if s.totalCount > 0 {
			s.fieldIdx = (s.fieldIdx + 1) % s.totalCount
			s.updateFieldFocus()
		}
		return s, nil
	case "shift+tab", "up":
		if s.totalCount > 0 {
			s.fieldIdx = (s.fieldIdx - 1 + s.totalCount) % s.totalCount
			s.updateFieldFocus()
		}
		return s, nil
	case "enter":
		// If current field handles enter (radio/checkbox/textarea), delegate
		if s.fieldIdx < len(s.formFields) {
			f := &s.formFields[s.fieldIdx]
			if f.Type == components.FieldRadio || f.Type == components.FieldCheckbox || f.Type == components.FieldTextArea {
				f.HandleKey(key)
				return s, nil
			}
		}
		// Otherwise submit the form
		s.step = PoolOnboardDone
		return s, nil
	case "ctrl+d":
		// Submit form
		s.step = PoolOnboardDone
		return s, nil
	}

	// Delegate to the focused field
	if s.fieldIdx < len(s.formFields) {
		s.formFields[s.fieldIdx].HandleKey(key)
	} else {
		stepperIdx := s.fieldIdx - len(s.formFields)
		if stepperIdx >= 0 && stepperIdx < len(s.steppers) {
			s.steppers[stepperIdx], _ = s.steppers[stepperIdx].Update(msg)
		}
	}

	return s, nil
}

func (s PoolOnboardScreen) collectProfile() map[string]any {
	profile := make(map[string]any)
	for _, f := range s.formFields {
		profile[f.Name] = f.GetValue()
	}
	for _, st := range s.steppers {
		profile[st.Name] = st.GetValue()
	}
	return profile
}

// View renders the onboarding screen.
func (s PoolOnboardScreen) View() string {
	var content string
	switch s.step {
	case PoolOnboardRole:
		content = s.roleView()
	case PoolOnboardForm:
		content = s.formView()
	case PoolOnboardDone:
		content = s.doneView()
	}

	return content
}

func (s PoolOnboardScreen) roleView() string {
	subtitle := theme.DimStyle.Render("What are you?")

	var options []string
	for i, role := range s.roles {
		marker := "  ○ "
		style := theme.DimStyle
		if i == s.roleCursor {
			marker = "  ● "
			style = theme.BrandStyle
		}
		options = append(options, marker+style.Render(role))
	}

	content := "  " + subtitle + "\n\n" + strings.Join(options, "\n") + "\n\n" +
		theme.DimStyle.Render("    enter to continue")

	return components.ScreenLayout(s.screenTitle(), components.DimHints("select role"), content)
}

func (s PoolOnboardScreen) formView() string {
	if s.err != nil {
		return components.ScreenLayout(s.screenTitle(), "", theme.RedStyle.Render("Error: "+s.err.Error()))
	}

	var lines []string

	for _, f := range s.formFields {
		attr, ok := s.schema.Profile[f.Name]
		label := titleCase(f.Name)
		if ok && !attr.IsPublic() {
			label += " \U0001F512" // lock emoji
		}
		if ok && !attr.IsRequired() {
			label += " " + theme.DimStyle.Render("(optional)")
		}

		// Override the label for rendering
		field := f
		field.Label = label
		lines = append(lines, field.Render(s.width))
		lines = append(lines, "")
	}

	for _, st := range s.steppers {
		attr, ok := s.schema.Profile[st.Name]
		label := titleCase(st.Name)
		if ok && !attr.IsPublic() {
			label += " \U0001F512"
		}
		if ok && !attr.IsRequired() {
			label += " " + theme.DimStyle.Render("(optional)")
		}

		stepper := st
		stepper.Label = label
		lines = append(lines, stepper.View())
		lines = append(lines, "")
	}

	lines = append(lines, theme.DimStyle.Render("  ctrl+d to submit"))

	body := strings.Join(lines, "\n")
	padded := lipgloss.NewStyle().PaddingLeft(2).Render(body)
	return components.ScreenLayout(s.screenTitle(), components.DimHints("fill profile"), padded)
}

func (s PoolOnboardScreen) doneView() string {
	var lines []string
	lines = append(lines, theme.GreenStyle.Render("✓ ")+"Profile complete!")
	lines = append(lines, "")
	if s.selectedRole != "" {
		lines = append(lines, theme.DimStyle.Render("  Role: ")+theme.TextStyle.Render(s.selectedRole))
	}
	lines = append(lines, theme.DimStyle.Render(fmt.Sprintf("  %d fields filled", len(s.formFields)+len(s.steppers))))
	lines = append(lines, "")
	lines = append(lines, theme.DimStyle.Render("  Press enter to continue"))

	return components.ScreenLayout(s.screenTitle(), components.DimHints("complete"), strings.Join(lines, "\n"))
}

// HelpBindings returns contextual key bindings for the help bar.
func (s PoolOnboardScreen) HelpBindings() []components.KeyBind {
	switch s.step {
	case PoolOnboardRole:
		return []components.KeyBind{
			{Key: "↑↓", Desc: "navigate"},
			{Key: "enter", Desc: "select"},
		}
	case PoolOnboardForm:
		return []components.KeyBind{
			{Key: "↑↓", Desc: "navigate fields"},
			{Key: "←→", Desc: "stepper"},
			{Key: "space", Desc: "toggle"},
			{Key: "ctrl+d", Desc: "submit"},
		}
	default:
		return []components.KeyBind{
			{Key: "enter", Desc: "continue"},
		}
	}
}

func (s PoolOnboardScreen) screenTitle() string {
	if s.editMode {
		return "Edit Profile · " + s.poolName
	}
	return "Join " + s.poolName
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	// Replace underscores with spaces, capitalize each word
	words := strings.Split(strings.ReplaceAll(s, "_", " "), " ")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
