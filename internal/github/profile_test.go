package github

import (
	"encoding/json"
	"testing"
)

func TestDatingProfile_Serialization(t *testing.T) {
	p := DatingProfile{
		DisplayName: "Alice",
		Bio:         "Backend engineer",
		Location:    "Berlin",
		AvatarURL:   "https://avatars.githubusercontent.com/u/123",
		Website:     "https://alice.dev",
		Social:      []string{"https://twitter.com/alice"},
		Interests:   []string{"rust", "hiking"},
		Intent:  []Intent{IntentDating, IntentFriendship},
		About:       "Looking for someone to debug life with",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded DatingProfile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.DisplayName != "Alice" {
		t.Errorf("expected Alice, got %s", decoded.DisplayName)
	}
	if len(decoded.Intent) != 2 {
		t.Errorf("expected 2 intent, got %d", len(decoded.Intent))
	}
	if decoded.Intent[0] != IntentDating {
		t.Errorf("expected dating, got %s", decoded.Intent[0])
	}
}

func TestDatingProfile_OmitEmpty(t *testing.T) {
	p := DatingProfile{
		DisplayName: "Bob",
		Bio:         "Dev",
		Location:    "Munich",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Omitted fields should not appear in JSON
	s := string(data)
	for _, field := range []string{"avatar_url", "website", "social", "showcase", "interests", "intent", "about"} {
		if contains(s, field) {
			t.Errorf("expected %s to be omitted, but found in JSON", field)
		}
	}
}

func TestDatingProfile_AllIntentOptions(t *testing.T) {
	opts := AllIntentOptions()
	if len(opts) != 4 {
		t.Errorf("expected 4 options, got %d", len(opts))
	}

	expected := map[string]bool{
		"dating":     true,
		"friendship": true,
		"yolo":       true,
		"networking": true,
	}
	for _, o := range opts {
		if !expected[o] {
			t.Errorf("unexpected option: %s", o)
		}
	}
}

func TestDatingProfile_AllGenderTargetOptions(t *testing.T) {
	opts := AllGenderTargetOptions()
	if len(opts) != 4 {
		t.Errorf("expected 4 options, got %d", len(opts))
	}

	expected := map[string]bool{
		"men":        true,
		"women":      true,
		"non-binary": true,
		"dev":        true,
	}
	for _, o := range opts {
		if !expected[o] {
			t.Errorf("unexpected option: %s", o)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && json.Valid([]byte(s)) && len(substr) > 0 && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
