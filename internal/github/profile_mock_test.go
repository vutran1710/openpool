package github

import "testing"

func TestMockProfiles_Count(t *testing.T) {
	profiles := MockProfiles()
	if len(profiles) != 5 {
		t.Errorf("expected 5 mock profiles, got %d", len(profiles))
	}
}

func TestMockProfiles_RichProfile(t *testing.T) {
	p := MockProfiles()[0] // Alice
	if p.DisplayName != "Alice Chen" {
		t.Errorf("expected Alice Chen, got %s", p.DisplayName)
	}
	if p.Bio == "" {
		t.Error("expected bio")
	}
	if p.Location == "" {
		t.Error("expected location")
	}
	if p.AvatarURL == "" {
		t.Error("expected avatar")
	}
	if p.Website == "" {
		t.Error("expected website")
	}
	if len(p.Social) == 0 {
		t.Error("expected social links")
	}
	if p.Showcase == "" {
		t.Error("expected showcase")
	}
	if len(p.Interests) == 0 {
		t.Error("expected interests")
	}
	if len(p.Intent) == 0 {
		t.Error("expected intent")
	}
	if len(p.GenderTarget) == 0 {
		t.Error("expected gender target")
	}
	if p.About == "" {
		t.Error("expected about")
	}
}

func TestMockProfiles_MinimalProfile(t *testing.T) {
	p := MockProfiles()[4] // eve
	if p.DisplayName != "eve" {
		t.Errorf("expected eve, got %s", p.DisplayName)
	}
	// Minimal should have no optional fields
	if p.AvatarURL != "" {
		t.Error("minimal should have no avatar")
	}
	if len(p.Interests) != 0 {
		t.Error("minimal should have no interests")
	}
	if p.Showcase != "" {
		t.Error("minimal should have no showcase")
	}
}

func TestMockProfiles_AllHaveNames(t *testing.T) {
	for i, p := range MockProfiles() {
		if p.DisplayName == "" {
			t.Errorf("profile %d has empty name", i)
		}
	}
}

func TestMockProfiles_IntentsAreValid(t *testing.T) {
	valid := map[string]bool{
		IntentDating:     true,
		IntentFriendship: true,
		IntentYolo:       true,
		IntentNetworking: true,
	}
	for i, p := range MockProfiles() {
		for _, intent := range p.Intent {
			if !valid[intent] {
				t.Errorf("profile %d has invalid intent: %s", i, intent)
			}
		}
	}
}

func TestMockProfiles_GenderTargetsAreValid(t *testing.T) {
	valid := map[string]bool{
		GenderMen:       true,
		GenderWomen:     true,
		GenderNonBinary: true,
		GenderDev:       true,
	}
	for i, p := range MockProfiles() {
		for _, gt := range p.GenderTarget {
			if !valid[gt] {
				t.Errorf("profile %d has invalid gender target: %s", i, gt)
			}
		}
	}
}
