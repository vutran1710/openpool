package screens

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

func newTestJoinScreen() JoinScreen {
	return NewJoinScreen("test-pool", "owner/test-pool", "aabbcc", "ws://localhost:8081", "alice", "12345")
}

func TestJoin_InitialState(t *testing.T) {
	s := newTestJoinScreen()
	if s.step != joinConfigSources {
		t.Errorf("expected joinConfigSources, got %d", s.step)
	}
	if s.poolName != "test-pool" {
		t.Errorf("expected test-pool, got %s", s.poolName)
	}
}

func TestJoin_ConfigSources_Continue(t *testing.T) {
	s := newTestJoinScreen()

	// Navigate to Continue and press enter
	s.configCursor = 2
	s, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if s.step != joinFetchingSources {
		t.Errorf("expected joinFetchingSources, got %d", s.step)
	}
	if cmd == nil {
		t.Error("expected fetch command")
	}
}

func TestJoin_ConfigSources_ToggleShowcase(t *testing.T) {
	s := newTestJoinScreen()
	if !s.includeShowcase {
		t.Error("expected showcase enabled by default")
	}

	// Toggle showcase off
	s.configCursor = 0
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if s.includeShowcase {
		t.Error("expected showcase disabled after toggle")
	}
}

func TestJoin_SourcesFetched_MovesToToggle(t *testing.T) {
	s := newTestJoinScreen()
	s.step = joinFetchingSources

	profile := &gh.DatingProfile{
		DisplayName: "Alice",
		Bio:         "engineer",
		Location:    "Berlin",
	}

	s, _ = s.Update(sourcesFetchedMsg{profile: profile})
	if s.step != joinToggleFields {
		t.Errorf("expected joinToggleFields, got %d", s.step)
	}
	if s.profile.DisplayName != "Alice" {
		t.Errorf("expected Alice, got %s", s.profile.DisplayName)
	}
}

func TestJoin_SourcesFetchError(t *testing.T) {
	s := newTestJoinScreen()
	s.step = joinFetchingSources

	s, _ = s.Update(sourcesFetchedMsg{err: fmt.Errorf("network error")})
	if s.step != joinError {
		t.Errorf("expected joinError, got %d", s.step)
	}
}

func TestJoin_FieldToggle_SubmitMovesToTemplate(t *testing.T) {
	s := newTestJoinScreen()
	s.step = joinToggleFields
	s.profile = &gh.DatingProfile{DisplayName: "Alice", Bio: "dev"}

	s, cmd := s.Update(components.CheckboxSubmitMsg{
		Selected: []components.CheckboxItem{
			{ID: "display_name"},
			{ID: "bio"},
		},
	})
	if s.step != joinFetchTemplate {
		t.Errorf("expected joinFetchTemplate, got %d", s.step)
	}
	if cmd == nil {
		t.Error("expected command")
	}
}

func TestJoin_TemplateFetched_NoFields_GoesToEncrypt(t *testing.T) {
	s := newTestJoinScreen()
	s.step = joinFetchTemplate
	s.profile = &gh.DatingProfile{DisplayName: "Alice"}

	s, _ = s.Update(templateFetchedMsg{template: gh.DefaultTemplate()})
	if s.step != joinEncrypting {
		t.Errorf("expected joinEncrypting (no fields), got %d", s.step)
	}
}

func TestJoin_TemplateFetched_WithFields_GoesToFill(t *testing.T) {
	s := newTestJoinScreen()
	s.step = joinFetchTemplate
	s.profile = &gh.DatingProfile{DisplayName: "Alice"}

	tmpl := &gh.RegistrationTemplate{
		Title:  "Join",
		Labels: []string{"reg"},
		Fields: []gh.RegistrationField{
			{ID: "name", Label: "Name", Type: "text", Required: true},
		},
	}

	s, _ = s.Update(templateFetchedMsg{template: tmpl})
	if s.step != joinFillTemplate {
		t.Errorf("expected joinFillTemplate, got %d", s.step)
	}
}

func TestJoin_TemplateError_UsesDefault(t *testing.T) {
	s := newTestJoinScreen()
	s.step = joinFetchTemplate
	s.profile = &gh.DatingProfile{DisplayName: "Alice"}

	s, _ = s.Update(templateFetchedMsg{err: fmt.Errorf("not found")})
	if s.template == nil {
		t.Error("expected default template")
	}
	// Default has no fields, so should go to encrypt
	if s.step != joinEncrypting {
		t.Errorf("expected joinEncrypting after default template, got %d", s.step)
	}
}

func TestJoin_IssueCreated_SavesPendingAndDone(t *testing.T) {
	s := newTestJoinScreen()
	s.step = joinSubmitting

	s, _ = s.Update(issueCreatedMsg{number: 42})
	if s.step != joinDone {
		t.Errorf("expected joinDone (saves pending, returns immediately), got %d", s.step)
	}
	if s.issueNumber != 42 {
		t.Errorf("expected issue 42, got %d", s.issueNumber)
	}
}

func TestJoin_IssueCreateError(t *testing.T) {
	s := newTestJoinScreen()
	s.step = joinSubmitting

	s, _ = s.Update(issueCreatedMsg{err: fmt.Errorf("403 forbidden")})
	if s.step != joinError {
		t.Errorf("expected joinError, got %d", s.step)
	}
}

func TestJoin_Done_EmitsMsg(t *testing.T) {
	s := newTestJoinScreen()
	s.step = joinDone
	s.poolName = "test-pool"
	s.userHash = "abc123"

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	doneMsg, ok := msg.(JoinDoneMsg)
	if !ok {
		t.Fatalf("expected JoinDoneMsg, got %T", msg)
	}
	if doneMsg.PoolName != "test-pool" {
		t.Errorf("expected test-pool, got %s", doneMsg.PoolName)
	}
	if doneMsg.UserHash != "abc123" {
		t.Errorf("expected abc123, got %s", doneMsg.UserHash)
	}
}

func TestJoin_EscCancels(t *testing.T) {
	s := newTestJoinScreen()
	s.step = joinConfigSources

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected cancel command")
	}
	msg := cmd()
	doneMsg, ok := msg.(JoinDoneMsg)
	if !ok {
		t.Fatalf("expected JoinDoneMsg, got %T", msg)
	}
	if doneMsg.PoolName != "" {
		t.Error("cancelled join should have empty pool name")
	}
}
