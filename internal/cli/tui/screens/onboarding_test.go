package screens

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

func TestOnboarding_InitialState(t *testing.T) {
	s := NewOnboardingScreen()
	if s.step != stepWelcome {
		t.Errorf("expected stepWelcome, got %d", s.step)
	}
}

func TestOnboarding_WelcomeToCheckGH(t *testing.T) {
	s := NewOnboardingScreen()
	s, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if s.step != stepCheckGH {
		t.Errorf("expected stepCheckGH, got %d", s.step)
	}
	if cmd == nil {
		t.Error("expected a command")
	}
}

func TestOnboarding_GHCheckFail_ShowsTokenInput(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepCheckGH

	s, _ = s.Update(ghCheckResult{err: fmt.Errorf("gh not found")})
	if s.step != stepAskToken {
		t.Errorf("expected stepAskToken, got %d", s.step)
	}
}

func TestOnboarding_GHCheckSuccess_SkipsToValidation(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepCheckGH

	s, cmd := s.Update(ghCheckResult{token: "ghp_test123"})
	if s.step != stepValidating {
		t.Errorf("expected stepValidating, got %d", s.step)
	}
	if s.token != "ghp_test123" {
		t.Errorf("expected token ghp_test123, got %s", s.token)
	}
	if cmd == nil {
		t.Error("expected a command")
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
		t.Error("expected a command")
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
	if s.errRetry != stepAskToken {
		t.Errorf("expected retry to stepAskToken, got %d", s.errRetry)
	}
}

func TestOnboarding_Error_RetryGoesToCorrectStep(t *testing.T) {
	// Token error → retry goes to token input
	s := NewOnboardingScreen()
	s.step = stepError
	s.errRetry = stepAskToken
	s.errMsg = "bad token"

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if s.step != stepAskToken {
		t.Errorf("expected stepAskToken on retry, got %d", s.step)
	}
	if s.errMsg != "" {
		t.Errorf("expected error cleared, got %s", s.errMsg)
	}

	// Registry error → retry goes to registry input
	s.step = stepError
	s.errRetry = stepAskRegistry
	s.errMsg = "bad registry"

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if s.step != stepAskRegistry {
		t.Errorf("expected stepAskRegistry on retry, got %d", s.step)
	}
}

func TestOnboarding_EmptyToken_DoesNotSubmit(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepAskToken
	s.input.SetValue("")

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if s.step != stepAskToken {
		t.Errorf("expected to stay on stepAskToken with empty input, got %d", s.step)
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

func TestOnboarding_KeysSuccess_MovesToRegistryInput(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepGenerateKeys
	s.displayName = "Alice"
	s.username = "alice"

	fakeKey := make([]byte, 32)
	s, cmd := s.Update(keysResult{pub: fakeKey})
	if s.step != stepAskRegistry {
		t.Errorf("expected stepAskRegistry, got %d", s.step)
	}
	if cmd == nil {
		t.Error("expected a command (textinput.Blink)")
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

func TestOnboarding_EmptyRegistry_DoesNotSubmit(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepAskRegistry
	s.input.SetValue("")

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if s.step != stepAskRegistry {
		t.Errorf("expected to stay on stepAskRegistry with empty input, got %d", s.step)
	}
}

func TestOnboarding_RegistryInput_Submit(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepAskRegistry
	s.input.SetValue("vutran1710/dating-test-registry")

	s, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if s.step != stepCloningRegistry {
		t.Errorf("expected stepCloningRegistry, got %d", s.step)
	}
	if s.registryURL != "vutran1710/dating-test-registry" {
		t.Errorf("expected registryURL set, got %s", s.registryURL)
	}
	if cmd == nil {
		t.Error("expected a command")
	}
}

func TestOnboarding_RegistryClone_Success(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepCloningRegistry

	s, cmd := s.Update(registryCloneResult{repoURL: "https://github.com/owner/reg.git"})
	if s.step != stepFetchingPools {
		t.Errorf("expected stepFetchingPools, got %d", s.step)
	}
	if s.registryURL != "https://github.com/owner/reg.git" {
		t.Errorf("expected registryURL updated, got %s", s.registryURL)
	}
	if cmd == nil {
		t.Error("expected a command")
	}
}

func TestOnboarding_RegistryClone_Failure(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepCloningRegistry

	s, _ = s.Update(registryCloneResult{err: fmt.Errorf("clone failed")})
	if s.step != stepError {
		t.Errorf("expected stepError, got %d", s.step)
	}
	if s.errRetry != stepAskRegistry {
		t.Errorf("expected retry to stepAskRegistry, got %d", s.errRetry)
	}
}

func TestOnboarding_PoolsFetch_Success(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepFetchingPools

	pools := []gh.PoolEntry{
		{Name: "test-pool", Description: "A test pool"},
	}
	s, cmd := s.Update(poolsFetchResult{pools: pools})
	if s.step != stepSaving {
		t.Errorf("expected stepSaving, got %d", s.step)
	}
	if len(s.pools) != 1 {
		t.Errorf("expected 1 pool, got %d", len(s.pools))
	}
	if cmd == nil {
		t.Error("expected a command")
	}
}

func TestOnboarding_PoolsFetch_EmptyIsOK(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepFetchingPools

	s, _ = s.Update(poolsFetchResult{pools: nil})
	if s.step != stepSaving {
		t.Errorf("expected stepSaving even with empty pools, got %d", s.step)
	}
}

func TestOnboarding_PoolsFetch_Failure(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepFetchingPools

	s, _ = s.Update(poolsFetchResult{err: fmt.Errorf("fetch failed")})
	if s.step != stepError {
		t.Errorf("expected stepError, got %d", s.step)
	}
	if s.errRetry != stepAskRegistry {
		t.Errorf("expected retry to stepAskRegistry, got %d", s.errRetry)
	}
}

func TestOnboarding_Save_Success(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepSaving

	s, _ = s.Update(saveResult{err: nil})
	if s.step != stepDone {
		t.Errorf("expected stepDone, got %d", s.step)
	}
}

func TestOnboarding_Save_Failure(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepSaving

	s, _ = s.Update(saveResult{err: fmt.Errorf("save failed")})
	if s.step != stepError {
		t.Errorf("expected stepError, got %d", s.step)
	}
}

func TestOnboarding_Done_EmitsDoneMsg(t *testing.T) {
	s := NewOnboardingScreen()
	s.step = stepDone
	s.displayName = "Alice"
	s.username = "alice"
	s.userID = "99"
	s.token = "ghp_abc"
	s.registryURL = "https://github.com/owner/reg.git"

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
	if doneMsg.Registry != "https://github.com/owner/reg.git" {
		t.Errorf("expected registry URL, got %s", doneMsg.Registry)
	}
}

func TestOnboarding_HelpBindings(t *testing.T) {
	s := NewOnboardingScreen()

	s.step = stepAskToken
	bindings := s.HelpBindings()
	if len(bindings) != 2 {
		t.Errorf("expected 2 bindings for token step, got %d", len(bindings))
	}

	s.step = stepAskRegistry
	bindings = s.HelpBindings()
	if len(bindings) != 2 {
		t.Errorf("expected 2 bindings for registry step, got %d", len(bindings))
	}

	s.step = stepWelcome
	bindings = s.HelpBindings()
	if len(bindings) != 1 {
		t.Errorf("expected 1 binding for welcome step, got %d", len(bindings))
	}
}

func TestOnboarding_FullHappyPath(t *testing.T) {
	s := NewOnboardingScreen()

	// 1. Welcome → enter
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if s.step != stepCheckGH {
		t.Fatalf("step 1: expected stepCheckGH, got %d", s.step)
	}

	// 2. GH check succeeds
	s, _ = s.Update(ghCheckResult{token: "ghp_auto"})
	if s.step != stepValidating {
		t.Fatalf("step 2: expected stepValidating, got %d", s.step)
	}

	// 3. Token valid
	s, _ = s.Update(tokenValidResult{userID: "1", username: "dev", displayName: "Dev"})
	if s.step != stepGenerateKeys {
		t.Fatalf("step 3: expected stepGenerateKeys, got %d", s.step)
	}

	// 4. Keys generated
	s, _ = s.Update(keysResult{pub: make([]byte, 32)})
	if s.step != stepAskRegistry {
		t.Fatalf("step 4: expected stepAskRegistry, got %d", s.step)
	}

	// 5. Enter registry
	s.input.SetValue("owner/registry")
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if s.step != stepCloningRegistry {
		t.Fatalf("step 5: expected stepCloningRegistry, got %d", s.step)
	}

	// 6. Registry cloned
	s, _ = s.Update(registryCloneResult{repoURL: "https://github.com/owner/registry.git"})
	if s.step != stepFetchingPools {
		t.Fatalf("step 6: expected stepFetchingPools, got %d", s.step)
	}

	// 7. Pools fetched
	s, _ = s.Update(poolsFetchResult{pools: []gh.PoolEntry{{Name: "pool1"}}})
	if s.step != stepSaving {
		t.Fatalf("step 7: expected stepSaving, got %d", s.step)
	}

	// 8. Saved
	s, _ = s.Update(saveResult{})
	if s.step != stepDone {
		t.Fatalf("step 8: expected stepDone, got %d", s.step)
	}

	// 9. Done → emit
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	done, ok := msg.(OnboardingDoneMsg)
	if !ok {
		t.Fatalf("step 9: expected OnboardingDoneMsg, got %T", msg)
	}
	if done.Registry != "https://github.com/owner/registry.git" {
		t.Errorf("expected registry in done msg, got %s", done.Registry)
	}
}
