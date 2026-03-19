package relay

import "testing"

func TestHub_QueueCap(t *testing.T) {
	hub := NewHub()
	for i := 0; i < maxQueueSize+5; i++ {
		status := hub.Send("offline-user", []byte("msg"))
		if i < maxQueueSize {
			if status != "queued" {
				t.Fatalf("msg %d: expected 'queued', got %q", i, status)
			}
		} else {
			if status != "queue full" {
				t.Fatalf("msg %d: expected 'queue full', got %q", i, status)
			}
		}
	}
	if hub.QueuedCount() != maxQueueSize {
		t.Fatalf("expected %d queued, got %d", maxQueueSize, hub.QueuedCount())
	}
}
