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
		LookingFor:  []LookingFor{LookingForDating, LookingForFriendship},
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
	if len(decoded.LookingFor) != 2 {
		t.Errorf("expected 2 looking_for, got %d", len(decoded.LookingFor))
	}
	if decoded.LookingFor[0] != LookingForDating {
		t.Errorf("expected dating, got %s", decoded.LookingFor[0])
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
	for _, field := range []string{"avatar_url", "website", "social", "showcase", "interests", "looking_for", "about"} {
		if contains(s, field) {
			t.Errorf("expected %s to be omitted, but found in JSON", field)
		}
	}
}

func TestDatingProfile_AllLookingForOptions(t *testing.T) {
	opts := AllLookingForOptions()
	if len(opts) != 5 {
		t.Errorf("expected 5 options, got %d", len(opts))
	}

	expected := map[string]bool{
		"friendship":   true,
		"dating":       true,
		"relationship": true,
		"networking":   true,
		"open":         true,
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
