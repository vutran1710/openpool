package screens

import (
	"testing"

	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
)

func TestPoolJoinMsg_Active(t *testing.T) {
	msg := PoolJoinMsg{Name: "pool1", Status: "active"}
	if msg.Status != "active" {
		t.Errorf("expected active, got %s", msg.Status)
	}
}

func TestPoolJoinMsg_Pending(t *testing.T) {
	msg := PoolJoinMsg{Name: "pool1", Status: "pending"}
	if msg.Status != "pending" {
		t.Errorf("expected pending, got %s", msg.Status)
	}
}

func TestPoolJoinMsg_Rejected_AllowsRejoin(t *testing.T) {
	msg := PoolJoinMsg{Name: "pool1", Status: "rejected"}
	// rejected should NOT be "active" or "pending" — allows rejoin
	if msg.Status == "active" || msg.Status == "pending" {
		t.Error("rejected should allow rejoin")
	}
}

func TestPoolJoinMsg_Empty_AllowsJoin(t *testing.T) {
	msg := PoolJoinMsg{Name: "pool1", Status: ""}
	if msg.Status == "active" || msg.Status == "pending" {
		t.Error("empty status should allow join")
	}
}

func TestPoolListItem_StatusIcons(t *testing.T) {
	tests := []struct {
		status   string
		contains string
	}{
		{"active", "✓"},
		{"pending", "⏱"},
		{"rejected", "✗"},
		{"", "join"},
	}

	for _, tt := range tests {
		rendered := components.RenderPoolListItem("test", "desc", tt.status, false, 50)
		if !containsString(rendered, tt.contains) {
			t.Errorf("status %q: expected %q in output, got: %s", tt.status, tt.contains, rendered)
		}
	}
}

func TestPoolCardAction_StatusText(t *testing.T) {
	tests := []struct {
		status   string
		contains string
	}{
		{"active", "member"},
		{"pending", "pending"},
		{"rejected", "join"},
		{"", "join"},
	}

	for _, tt := range tests {
		card := components.PoolCardData{
			Name:   "test",
			Status: tt.status,
		}
		rendered := components.RenderPoolCard(card, 50, false)
		if !containsString(rendered, tt.contains) {
			t.Errorf("status %q: expected %q in card output", tt.status, tt.contains)
		}
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
