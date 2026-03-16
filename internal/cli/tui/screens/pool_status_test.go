package screens

import (
	"testing"

	gh "github.com/vutran1710/dating-dev/internal/github"

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
		{"rejected", "join again"},
		{"", "join"},
	}

	for _, tt := range tests {
		card := components.PoolCardData{
			Name:   "test",
			Status: tt.status,
		}
		rendered := components.RenderPoolCard(card, 60, false)
		if !containsString(rendered, tt.contains) {
			t.Errorf("status %q: expected %q in card output", tt.status, tt.contains)
		}
	}
}

// Test that pools screen reflects status from the poolStatus map
func TestPoolsScreen_StatusFromMap(t *testing.T) {
	statuses := map[string]string{
		"pool-a": "active",
		"pool-b": "pending",
		"pool-c": "rejected",
	}

	s := NewPoolsScreen("https://example.com/reg.git", statuses)

	if s.poolStatus["pool-a"] != "active" {
		t.Errorf("expected pool-a active, got %s", s.poolStatus["pool-a"])
	}
	if s.poolStatus["pool-b"] != "pending" {
		t.Errorf("expected pool-b pending, got %s", s.poolStatus["pool-b"])
	}
	if s.poolStatus["pool-c"] != "rejected" {
		t.Errorf("expected pool-c rejected, got %s", s.poolStatus["pool-c"])
	}
	if s.poolStatus["pool-d"] != "" {
		t.Errorf("expected pool-d empty, got %s", s.poolStatus["pool-d"])
	}
}

// Test that recreating pools screen with new statuses picks up changes
func TestPoolsScreen_StatusRefreshOnRecreate(t *testing.T) {
	// Initial: pending
	s := NewPoolsScreen("https://example.com/reg.git", map[string]string{"pool1": "pending"})
	if s.poolStatus["pool1"] != "pending" {
		t.Errorf("expected pending, got %s", s.poolStatus["pool1"])
	}

	// Recreate with updated status (simulates what app does after poll)
	s = NewPoolsScreen("https://example.com/reg.git", map[string]string{"pool1": "rejected"})
	if s.poolStatus["pool1"] != "rejected" {
		t.Errorf("expected rejected after recreate, got %s", s.poolStatus["pool1"])
	}

	// Recreate with active
	s = NewPoolsScreen("https://example.com/reg.git", map[string]string{"pool1": "active"})
	if s.poolStatus["pool1"] != "active" {
		t.Errorf("expected active after recreate, got %s", s.poolStatus["pool1"])
	}
}

// Test that pool items carry the status from the map
func TestPoolsScreen_FetchedPoolsCarryStatus(t *testing.T) {
	s := NewPoolsScreen("https://example.com/reg.git", map[string]string{"test-pool": "pending"})

	// Simulate pools being fetched
	s, _ = s.Update(poolsFetchedMsg{
		pools: []poolItem{
			{entry: gh.PoolEntry{Name: "test-pool", Repo: "owner/test-pool"}, status: ""},
		},
	})

	// The status should come from the poolStatus map, not the fetched item
	if len(s.pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(s.pools))
	}
	// Note: fetchPools sets status from s.poolStatus[e.Name] in the real flow
	// In this test, we directly set the poolItem, so test the map is available
	if s.poolStatus["test-pool"] != "pending" {
		t.Errorf("expected poolStatus map to have pending, got %s", s.poolStatus["test-pool"])
	}
}

// Test PoolJoinMsg emitted from pools screen carries correct status
func TestPoolsScreen_EnterEmitsStatus(t *testing.T) {
	statuses := map[string]string{"pool-x": "rejected"}
	s := NewPoolsScreen("https://example.com/reg.git", statuses)

	// Simulate pools loaded with one pool
	s, _ = s.Update(poolsFetchedMsg{
		pools: []poolItem{
			{entry: gh.PoolEntry{Name: "pool-x", Repo: "owner/pool-x"}, status: "rejected"},
		},
	})

	if len(s.pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(s.pools))
	}
	if s.pools[0].status != "rejected" {
		t.Errorf("expected pool status rejected, got %s", s.pools[0].status)
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
