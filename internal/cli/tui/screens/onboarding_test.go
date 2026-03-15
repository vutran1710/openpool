package screens

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestOnboarding_InitialState(t *testing.T) {
	s := NewOnboardingScreen()
	if s.step != stepWelcome {
		t.Errorf("expected stepWelcome, got %d", s.step)
	}
}

func TestOnboarding_WelcomeToCheckGH(t *testing.T) {
	s := NewOnboardingScreen()

	// Press enter on welcome → should move to stepCheckGH
	s, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if s.step != stepCheckGH {
		t.Errorf("expected stepCheckGH, got %d", s.step)
	}
	if cmd == nil {
		t.Error("expected a command (checkGH + spinner.Tick)")
	}
}

func TestOnboarding_GHCheckFail_ShowsTokenInput(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepCheckGH

	// Simulate gh check failure
	s, _ = s.Update(ghCheckResult{err: fmt.Errorf("gh not found")})
	if s.step != stepAskToken {
		t.Errorf("expected stepAskToken, got %d", s.step)
	}
}

func TestOnboarding_GHCheckSuccess_SkipsToValidation(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepCheckGH

	// Simulate gh check success
	s, cmd := s.Update(ghCheckResult{token: "ghp_test123"})
	if s.step != stepValidating {
		t.Errorf("expected stepValidating, got %d", s.step)
	}
	if s.token != "ghp_test123" {
		t.Errorf("expected token ghp_test123, got %s", s.token)
	}
	if cmd == nil {
		t.Error("expected a command (validateToken + spinner.Tick)")
	}
}

func TestOnboarding_TokenValidation_Success(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepValidating
	s.token = "ghp_test123"

	s, cmd := s.Update(tokenValidResult{
		userID:      "12345",
		username:    "testuser",
		displayName: "Test User",
	})
	if s.step != stepGenerateKeys {
		t.Errorf("expected stepGenerateKeys, got %d", s.step)
	}
	if s.userID != "12345" {
		t.Errorf("expected userID 12345, got %s", s.userID)
	}
	if s.username != "testuser" {
		t.Errorf("expected username testuser, got %s", s.username)
	}
	if cmd == nil {
		t.Error("expected a command (generateKeys + spinner.Tick)")
	}
}

func TestOnboarding_TokenValidation_Failure(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepValidating

	s, _ = s.Update(tokenValidResult{err: fmt.Errorf("invalid token")})
	if s.step != stepError {
		t.Errorf("expected stepError, got %d", s.step)
	}
	if s.errMsg != "invalid token" {
		t.Errorf("expected error 'invalid token', got %s", s.errMsg)
	}
}

func TestOnboarding_Error_RetryGoesToTokenInput(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepError
	s.errMsg = "something failed"

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if s.step != stepAskToken {
		t.Errorf("expected stepAskToken on retry, got %d", s.step)
	}
	if s.errMsg != "" {
		t.Errorf("expected error cleared, got %s", s.errMsg)
	}
}

func TestOnboarding_EmptyToken_DoesNotSubmit(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepAskToken
	s.input.SetValue("")

	s, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if s.step != stepAskToken {
		t.Errorf("expected to stay on stepAskToken with empty input, got %d", s.step)
	}
	// Should not produce a validateToken command
	if cmd != nil {
		// cmd might be from textinput, check we didn't move step
		if s.step != stepAskToken {
			t.Error("should not advance with empty token")
		}
	}
}

func TestOnboarding_ManualToken_Submit(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepAskToken
	s.input.SetValue("ghp_manual_token_123")

	s, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if s.step != stepValidating {
		t.Errorf("expected stepValidating, got %d", s.step)
	}
	if s.token != "ghp_manual_token_123" {
		t.Errorf("expected token ghp_manual_token_123, got %s", s.token)
	}
	if cmd == nil {
		t.Error("expected a command")
	}
}

func TestOnboarding_Done_EmitsDoneMsg(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepDone
	s.displayName = "Alice"
	s.username = "alice"
	s.userID = "99"
	s.token = "ghp_abc"

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected OnboardingDoneMsg command")
	}

	msg := cmd()
	doneMsg, ok := msg.(OnboardingDoneMsg)
	if !ok {
		t.Fatalf("expected OnboardingDoneMsg, got %T", msg)
	}
	if doneMsg.DisplayName != "Alice" {
		t.Errorf("expected Alice, got %s", doneMsg.DisplayName)
	}
	if doneMsg.Username != "alice" {
		t.Errorf("expected alice, got %s", doneMsg.Username)
	}
	if doneMsg.Token != "ghp_abc" {
		t.Errorf("expected ghp_abc, got %s", doneMsg.Token)
	}
}

func TestOnboarding_KeysSuccess_MovesToSaving(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepGenerateKeys

	fakeKey := make([]byte, 32)
	s, cmd := s.Update(keysResult{pub: fakeKey})
	if s.step != stepSaving {
		t.Errorf("expected stepSaving, got %d", s.step)
	}
	if cmd == nil {
		t.Error("expected a command")
	}
}

func TestOnboarding_KeysFailure_ShowsError(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepGenerateKeys

	s, _ = s.Update(keysResult{err: fmt.Errorf("key gen failed")})
	if s.step != stepError {
		t.Errorf("expected stepError, got %d", s.step)
	}
}

func TestOnboarding_HelpBindings(t *testing.T) {
	s := NewOnboardingScreen()

	s.step = stepAskToken
	bindings := s.HelpBindings()
	if len(bindings) != 2 {
		t.Errorf("expected 2 bindings for token step, got %d", len(bindings))
	}

	s.step = stepWelcome
	bindings = s.HelpBindings()
	if len(bindings) != 1 {
		t.Errorf("expected 1 binding for welcome step, got %d", len(bindings))
	}
}
