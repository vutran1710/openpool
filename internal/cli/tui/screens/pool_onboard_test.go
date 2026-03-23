package screens

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/dating-dev/internal/schema"
	"gopkg.in/yaml.v3"
)

func makeRolesNode(roles []string) yaml.Node {
	var node yaml.Node
	node.Kind = yaml.SequenceNode
	for _, r := range roles {
		node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: r})
	}
	return node
}

func TestPoolOnboard_SingleRole_SkipsRoleSelection(t *testing.T) {
	s := &schema.PoolSchema{
		Name: "test-pool",
		Profile: map[string]schema.Attribute{
			"about": {Type: "text"},
		},
		Roles: makeRolesNode([]string{"user"}),
	}

	screen := NewPoolOnboardScreen("test-pool", s, 80, 40)
	if screen.step != PoolOnboardForm {
		t.Errorf("expected PoolOnboardForm (skipped role), got %d", screen.step)
	}
	if screen.selectedRole != "user" {
		t.Errorf("expected selectedRole='user', got %q", screen.selectedRole)
	}
}

func TestPoolOnboard_MultipleRoles_ShowsRoleSelection(t *testing.T) {
	s := &schema.PoolSchema{
		Name: "test-pool",
		Profile: map[string]schema.Attribute{
			"about": {Type: "text"},
		},
		Roles: makeRolesNode([]string{"man", "woman"}),
	}

	screen := NewPoolOnboardScreen("test-pool", s, 80, 40)
	if screen.step != PoolOnboardRole {
		t.Errorf("expected PoolOnboardRole, got %d", screen.step)
	}
	if len(screen.roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(screen.roles))
	}
}

func TestPoolOnboard_RoleSelection_NavigateAndSelect(t *testing.T) {
	s := &schema.PoolSchema{
		Name: "test-pool",
		Profile: map[string]schema.Attribute{
			"about": {Type: "text"},
		},
		Roles: makeRolesNode([]string{"man", "woman"}),
	}

	screen := NewPoolOnboardScreen("test-pool", s, 80, 40)

	// Navigate down
	screen, _ = screen.Update(tea.KeyMsg{Type: tea.KeyDown})
	if screen.roleCursor != 1 {
		t.Errorf("expected cursor=1, got %d", screen.roleCursor)
	}

	// Select "woman"
	screen, _ = screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if screen.step != PoolOnboardForm {
		t.Errorf("expected PoolOnboardForm after selection, got %d", screen.step)
	}
	if screen.selectedRole != "woman" {
		t.Errorf("expected selectedRole='woman', got %q", screen.selectedRole)
	}
}

func TestPoolOnboard_FormRendersAllFields(t *testing.T) {
	min18, max99 := 18, 99
	s := &schema.PoolSchema{
		Name: "test-pool",
		Profile: map[string]schema.Attribute{
			"about": {Type: "text"},
			"age":   {Type: "range", Min: &min18, Max: &max99},
			"gender": {
				Type:   "enum",
				Values: []any{"male", "female"},
			},
			"interests": {
				Type:   "multi",
				Values: []any{"hiking", "coding"},
			},
		},
		Roles: makeRolesNode([]string{"user"}),
	}

	screen := NewPoolOnboardScreen("test-pool", s, 80, 40)
	if screen.step != PoolOnboardForm {
		t.Fatalf("expected PoolOnboardForm, got %d", screen.step)
	}

	// Should have 3 form fields (about, gender, interests) and 1 stepper (age)
	if len(screen.formFields) != 3 {
		t.Errorf("expected 3 form fields, got %d", len(screen.formFields))
	}
	if len(screen.steppers) != 1 {
		t.Errorf("expected 1 stepper, got %d", len(screen.steppers))
	}

	// View should render without panic
	view := screen.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestPoolOnboard_SubmitEmitsDoneMsg(t *testing.T) {
	s := &schema.PoolSchema{
		Name: "test-pool",
		Profile: map[string]schema.Attribute{
			"about": {Type: "text"},
		},
		Roles: makeRolesNode([]string{"user"}),
	}

	screen := NewPoolOnboardScreen("test-pool", s, 80, 40)

	// Type something into the text field
	screen.formFields[0].HandleKey("H")
	screen.formFields[0].HandleKey("i")

	// Submit with ctrl+d
	screen, _ = screen.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	if screen.step != PoolOnboardDone {
		t.Fatalf("expected PoolOnboardDone, got %d", screen.step)
	}

	// Press enter to emit done message
	screen, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command")
	}

	msg := cmd()
	doneMsg, ok := msg.(PoolOnboardDoneMsg)
	if !ok {
		t.Fatalf("expected PoolOnboardDoneMsg, got %T", msg)
	}
	if doneMsg.PoolName != "test-pool" {
		t.Errorf("expected poolName='test-pool', got %q", doneMsg.PoolName)
	}
	if doneMsg.Role != "user" {
		t.Errorf("expected role='user', got %q", doneMsg.Role)
	}
	if doneMsg.Profile["about"] != "Hi" {
		t.Errorf("expected about='Hi', got %v", doneMsg.Profile["about"])
	}
}

func TestPoolOnboard_NoRoles(t *testing.T) {
	s := &schema.PoolSchema{
		Name: "test-pool",
		Profile: map[string]schema.Attribute{
			"about": {Type: "text"},
		},
	}

	screen := NewPoolOnboardScreen("test-pool", s, 80, 40)
	// No roles → skip role selection
	if screen.step != PoolOnboardForm {
		t.Errorf("expected PoolOnboardForm with no roles, got %d", screen.step)
	}
	if screen.selectedRole != "" {
		t.Errorf("expected empty selectedRole, got %q", screen.selectedRole)
	}
}

func TestPoolOnboard_FieldNavigation(t *testing.T) {
	s := &schema.PoolSchema{
		Name: "test-pool",
		Profile: map[string]schema.Attribute{
			"about": {Type: "text"},
			"name":  {Type: "text"},
		},
		Roles: makeRolesNode([]string{"user"}),
	}

	screen := NewPoolOnboardScreen("test-pool", s, 80, 40)

	// First field should be focused
	if screen.fieldIdx != 0 {
		t.Errorf("expected fieldIdx=0, got %d", screen.fieldIdx)
	}
	if !screen.formFields[0].Focused {
		t.Error("expected first field to be focused")
	}

	// Navigate down
	screen, _ = screen.Update(tea.KeyMsg{Type: tea.KeyTab})
	if screen.fieldIdx != 1 {
		t.Errorf("expected fieldIdx=1, got %d", screen.fieldIdx)
	}
	if !screen.formFields[1].Focused {
		t.Error("expected second field to be focused")
	}
	if screen.formFields[0].Focused {
		t.Error("expected first field to be unfocused")
	}
}

func TestPoolOnboard_HelpBindings(t *testing.T) {
	s := &schema.PoolSchema{
		Name: "test-pool",
		Profile: map[string]schema.Attribute{
			"about": {Type: "text"},
		},
		Roles: makeRolesNode([]string{"man", "woman"}),
	}

	screen := NewPoolOnboardScreen("test-pool", s, 80, 40)

	// Role step
	bindings := screen.HelpBindings()
	if len(bindings) == 0 {
		t.Error("expected help bindings for role step")
	}

	// Move to form step
	screen, _ = screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
	bindings = screen.HelpBindings()
	if len(bindings) == 0 {
		t.Error("expected help bindings for form step")
	}
}

func TestPoolOnboard_WindowSizeMsg(t *testing.T) {
	s := &schema.PoolSchema{
		Name: "test-pool",
		Profile: map[string]schema.Attribute{
			"about": {Type: "text"},
		},
		Roles: makeRolesNode([]string{"user"}),
	}

	screen := NewPoolOnboardScreen("test-pool", s, 80, 40)
	screen, _ = screen.Update(tea.WindowSizeMsg{Width: 120, Height: 60})
	if screen.width != 120 {
		t.Errorf("expected width=120, got %d", screen.width)
	}
	if screen.height != 60 {
		t.Errorf("expected height=60, got %d", screen.height)
	}
}
