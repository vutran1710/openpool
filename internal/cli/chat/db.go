package chat

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Conversation struct {
	PeerMatchHash string
	LastMessage   string
	LastMessageAt time.Time
	UnreadCount   int
}

type Message struct {
	Body      string
	IsMe      bool
	CreatedAt time.Time
}

type ConversationDB struct {
	db *sql.DB
}

func OpenConversationDB(path string) (*ConversationDB, error) {
	db, err := sql.Open("sqlite", path+"?_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS conversations (
			peer_match_hash TEXT PRIMARY KEY,
			last_message    TEXT,
			last_message_at INTEGER,
			unread_count    INTEGER DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS messages (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			peer_match_hash TEXT NOT NULL,
			body            TEXT NOT NULL,
			is_me           INTEGER NOT NULL,
			created_at      INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_messages_peer ON messages(peer_match_hash, created_at);
	`)
	if err != nil {
		return nil, fmt.Errorf("creating tables: %w", err)
	}
	return &ConversationDB{db: db}, nil
}

func (d *ConversationDB) Close() error { return d.db.Close() }

func (d *ConversationDB) SaveMessage(peerMatchHash, body string, isMe bool) error {
	now := time.Now().Unix()
	_, err := d.db.Exec(
		"INSERT INTO messages (peer_match_hash, body, is_me, created_at) VALUES (?, ?, ?, ?)",
		peerMatchHash, body, boolToInt(isMe), now,
	)
	if err != nil {
		return err
	}
	unreadInc := 0
	if !isMe {
		unreadInc = 1
	}
	_, err = d.db.Exec(`
		INSERT INTO conversations (peer_match_hash, last_message, last_message_at, unread_count)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(peer_match_hash) DO UPDATE SET
			last_message = excluded.last_message,
			last_message_at = excluded.last_message_at,
			unread_count = unread_count + ?
	`, peerMatchHash, body, now, unreadInc, unreadInc)
	return err
}

func (d *ConversationDB) ListConversations() ([]Conversation, error) {
	rows, err := d.db.Query("SELECT peer_match_hash, last_message, last_message_at, unread_count FROM conversations ORDER BY last_message_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var convos []Conversation
	for rows.Next() {
		var c Conversation
		var ts int64
		rows.Scan(&c.PeerMatchHash, &c.LastMessage, &ts, &c.UnreadCount)
		c.LastMessageAt = time.Unix(ts, 0)
		convos = append(convos, c)
	}
	return convos, nil
}

func (d *ConversationDB) LoadHistory(peerMatchHash string) ([]Message, error) {
	rows, err := d.db.Query(
		"SELECT body, is_me, created_at FROM messages WHERE peer_match_hash = ? ORDER BY created_at",
		peerMatchHash,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []Message
	for rows.Next() {
		var m Message
		var isMe int
		var ts int64
		rows.Scan(&m.Body, &isMe, &ts)
		m.IsMe = isMe == 1
		m.CreatedAt = time.Unix(ts, 0)
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func (d *ConversationDB) MarkRead(peerMatchHash string) error {
	_, err := d.db.Exec("UPDATE conversations SET unread_count = 0 WHERE peer_match_hash = ?", peerMatchHash)
	return err
}

func (d *ConversationDB) UnreadTotal() (int, error) {
	var total int
	err := d.db.QueryRow("SELECT COALESCE(SUM(unread_count), 0) FROM conversations").Scan(&total)
	return total, err
}

func (d *ConversationDB) PersistGreeting(peerMatchHash, greeting string) error {
	if greeting == "" {
		return nil
	}
	var count int
	d.db.QueryRow("SELECT COUNT(*) FROM messages WHERE peer_match_hash = ?", peerMatchHash).Scan(&count)
	if count > 0 {
		return nil
	}
	return d.SaveMessage(peerMatchHash, greeting, false)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
