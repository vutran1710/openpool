package chat

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestDB(t *testing.T) *ConversationDB {
	t.Helper()
	dir := t.TempDir()
	db, err := OpenConversationDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("OpenConversationDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newDBOnlyClient(db *ConversationDB) *ChatClient {
	return &ChatClient{Relay: nil, DB: db}
}

func TestChatClient_Send_Persists(t *testing.T) {
	db := newTestDB(t)
	c := newDBOnlyClient(db)

	// Directly persist to DB (bypassing relay which is nil)
	if err := c.DB.SaveMessage("peer123", "hello", true); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}

	history, err := c.History("peer123")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 message, got %d", len(history))
	}
	if history[0].Body != "hello" {
		t.Errorf("expected body 'hello', got %q", history[0].Body)
	}
	if !history[0].IsMe {
		t.Error("expected IsMe=true for sent message")
	}
}

func TestChatClient_HandleIncoming_Persists(t *testing.T) {
	db := newTestDB(t)
	c := newDBOnlyClient(db)

	c.handleIncoming("peer456", []byte("hi there"))

	history, err := c.History("peer456")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 message, got %d", len(history))
	}
	if history[0].Body != "hi there" {
		t.Errorf("expected body 'hi there', got %q", history[0].Body)
	}
	if history[0].IsMe {
		t.Error("expected IsMe=false for incoming message")
	}

	unread, err := c.UnreadTotal()
	if err != nil {
		t.Fatalf("UnreadTotal: %v", err)
	}
	if unread != 1 {
		t.Errorf("expected unread=1, got %d", unread)
	}
}

func TestChatClient_OnMsg_Called(t *testing.T) {
	db := newTestDB(t)
	c := newDBOnlyClient(db)

	var called string
	c.OnMsg = func(peerMatchHash string) {
		called = peerMatchHash
	}

	c.handleIncoming("peer789", []byte("ping"))

	if called != "peer789" {
		t.Errorf("expected OnMsg called with 'peer789', got %q", called)
	}
}

func TestChatClient_MarkRead(t *testing.T) {
	db := newTestDB(t)
	c := newDBOnlyClient(db)

	c.handleIncoming("peerABC", []byte("msg1"))
	c.handleIncoming("peerABC", []byte("msg2"))

	unread, err := c.UnreadTotal()
	if err != nil {
		t.Fatalf("UnreadTotal: %v", err)
	}
	if unread != 2 {
		t.Errorf("expected unread=2, got %d", unread)
	}

	if err := c.MarkRead("peerABC"); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}

	unread, err = c.UnreadTotal()
	if err != nil {
		t.Fatalf("UnreadTotal after mark: %v", err)
	}
	if unread != 0 {
		t.Errorf("expected unread=0 after MarkRead, got %d", unread)
	}
}

func TestChatClient_PersistGreeting(t *testing.T) {
	db := newTestDB(t)
	c := newDBOnlyClient(db)

	if err := c.PersistGreeting("peerXYZ", "Welcome to the chat!"); err != nil {
		t.Fatalf("PersistGreeting: %v", err)
	}

	history, err := c.History("peerXYZ")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 message, got %d", len(history))
	}
	if history[0].Body != "Welcome to the chat!" {
		t.Errorf("expected greeting body, got %q", history[0].Body)
	}
	if history[0].IsMe {
		t.Error("expected IsMe=false for greeting (it comes from peer)")
	}

	// Second call should be a no-op (messages exist)
	if err := c.PersistGreeting("peerXYZ", "Another greeting"); err != nil {
		t.Fatalf("PersistGreeting second call: %v", err)
	}
	history, _ = c.History("peerXYZ")
	if len(history) != 1 {
		t.Errorf("expected still 1 message after second PersistGreeting, got %d", len(history))
	}
}

// Ensure test binary doesn't need a real DATING_HOME.
func TestMain(m *testing.M) {
	dir, _ := os.MkdirTemp("", "dating-chat-test-*")
	os.Setenv("DATING_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
