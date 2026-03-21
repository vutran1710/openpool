package chat

import (
	"path/filepath"
	"testing"
	"time"
)

func testDB(t *testing.T) *ConversationDB {
	t.Helper()
	dir := t.TempDir()
	db, err := OpenConversationDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("OpenConversationDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSaveMessage_AndHistory(t *testing.T) {
	db := testDB(t)

	if err := db.SaveMessage("peer1", "hello", true); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}
	time.Sleep(time.Millisecond) // ensure ordering by created_at
	if err := db.SaveMessage("peer1", "world", false); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}

	msgs, err := db.LoadHistory("peer1")
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Body != "hello" || !msgs[0].IsMe {
		t.Errorf("first message: got body=%q isMe=%v, want body=%q isMe=true", msgs[0].Body, msgs[0].IsMe, "hello")
	}
	if msgs[1].Body != "world" || msgs[1].IsMe {
		t.Errorf("second message: got body=%q isMe=%v, want body=%q isMe=false", msgs[1].Body, msgs[1].IsMe, "world")
	}
}

func TestSaveMessage_UpdatesConversation(t *testing.T) {
	db := testDB(t)

	if err := db.SaveMessage("peer1", "first", false); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}
	if err := db.SaveMessage("peer1", "second", false); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}

	convos, err := db.ListConversations()
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(convos) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(convos))
	}
	if convos[0].LastMessage != "second" {
		t.Errorf("expected last_message=%q, got %q", "second", convos[0].LastMessage)
	}
	if convos[0].UnreadCount != 2 {
		t.Errorf("expected unread_count=2, got %d", convos[0].UnreadCount)
	}
}

func TestSaveMessage_SentDoesNotIncrementUnread(t *testing.T) {
	db := testDB(t)

	if err := db.SaveMessage("peer1", "hey there", true); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}

	convos, err := db.ListConversations()
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(convos) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(convos))
	}
	if convos[0].UnreadCount != 0 {
		t.Errorf("expected unread_count=0 for sent message, got %d", convos[0].UnreadCount)
	}
}

func TestMarkRead(t *testing.T) {
	db := testDB(t)

	if err := db.SaveMessage("peer1", "msg1", false); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}
	if err := db.SaveMessage("peer1", "msg2", false); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}

	if err := db.MarkRead("peer1"); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}

	convos, err := db.ListConversations()
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(convos) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(convos))
	}
	if convos[0].UnreadCount != 0 {
		t.Errorf("expected unread_count=0 after MarkRead, got %d", convos[0].UnreadCount)
	}
}

func TestUnreadTotal(t *testing.T) {
	db := testDB(t)

	if err := db.SaveMessage("peer1", "a", false); err != nil {
		t.Fatalf("SaveMessage peer1: %v", err)
	}
	if err := db.SaveMessage("peer1", "b", false); err != nil {
		t.Fatalf("SaveMessage peer1: %v", err)
	}
	if err := db.SaveMessage("peer2", "c", false); err != nil {
		t.Fatalf("SaveMessage peer2: %v", err)
	}

	total, err := db.UnreadTotal()
	if err != nil {
		t.Fatalf("UnreadTotal: %v", err)
	}
	if total != 3 {
		t.Errorf("expected total unread=3, got %d", total)
	}
}

func TestListConversations_SortedByRecency(t *testing.T) {
	db := testDB(t)

	if err := db.SaveMessage("peer1", "from peer1", false); err != nil {
		t.Fatalf("SaveMessage peer1: %v", err)
	}
	time.Sleep(time.Second) // ensure different unix timestamps
	if err := db.SaveMessage("peer2", "from peer2", false); err != nil {
		t.Fatalf("SaveMessage peer2: %v", err)
	}

	convos, err := db.ListConversations()
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(convos) != 2 {
		t.Fatalf("expected 2 conversations, got %d", len(convos))
	}
	if convos[0].PeerMatchHash != "peer2" {
		t.Errorf("expected peer2 first (most recent), got %q", convos[0].PeerMatchHash)
	}
	if convos[1].PeerMatchHash != "peer1" {
		t.Errorf("expected peer1 second, got %q", convos[1].PeerMatchHash)
	}
}

func TestPersistGreeting_Idempotent(t *testing.T) {
	db := testDB(t)

	if err := db.PersistGreeting("peer1", "Hello!"); err != nil {
		t.Fatalf("PersistGreeting first: %v", err)
	}
	if err := db.PersistGreeting("peer1", "Hello!"); err != nil {
		t.Fatalf("PersistGreeting second: %v", err)
	}

	msgs, err := db.LoadHistory("peer1")
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("expected 1 message after idempotent PersistGreeting, got %d", len(msgs))
	}
}

func TestPersistGreeting_SkipsIfMessagesExist(t *testing.T) {
	db := testDB(t)

	if err := db.SaveMessage("peer1", "existing message", true); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}

	if err := db.PersistGreeting("peer1", "greeting"); err != nil {
		t.Fatalf("PersistGreeting: %v", err)
	}

	msgs, err := db.LoadHistory("peer1")
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("expected 1 message (greeting skipped), got %d", len(msgs))
	}
	if msgs[0].Body != "existing message" {
		t.Errorf("expected body=%q, got %q", "existing message", msgs[0].Body)
	}
}

func TestSavePeerKey_AndGetPeerKey(t *testing.T) {
	db := testDB(t)

	pubkey := []byte("12345678901234567890123456789012") // 32 bytes
	if err := db.SavePeerKey("peer1", pubkey); err != nil {
		t.Fatalf("SavePeerKey: %v", err)
	}

	got, err := db.GetPeerKey("peer1")
	if err != nil {
		t.Fatalf("GetPeerKey: %v", err)
	}
	if string(got) != string(pubkey) {
		t.Fatalf("pubkey mismatch")
	}
}

func TestGetPeerKey_NotFound(t *testing.T) {
	db := testDB(t)

	_, err := db.GetPeerKey("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing peer key")
	}
}

func TestSavePeerKey_Upsert(t *testing.T) {
	db := testDB(t)

	key1 := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	key2 := []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

	db.SavePeerKey("peer1", key1)
	db.SavePeerKey("peer1", key2) // overwrite

	got, _ := db.GetPeerKey("peer1")
	if string(got) != string(key2) {
		t.Fatal("upsert should overwrite with latest key")
	}
}
