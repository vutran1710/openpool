# TUI Notifications & Conversations Panel

## Goal

Add a conversations panel to the home screen, notification badges on menu items, background relay connection for real-time message delivery, and local message persistence via SQLite.

## Home Screen Layout

Split panel — menu (left) + conversations (right). Tab switches focus between panels.

```
┌─ ♥ dating.dev ────────────────────────────────────────────────────┐
│                                                                    │
│  Menu                        │  Conversations                     │
│  ❯ Discover  new people      │  ▸ d39e5f2e...  "hey!" (2 new)   │
│    Matches  (1 new)          │    c40d85af...  "see you"          │
│    Inbox  (3)                │    fab41489...  "thanks!"          │
│    Pools                     │                                     │
│    Profile                   │                                     │
│    Settings                  │                                     │
│                              │                                     │
│  ↑↓ navigate · enter select  │  tab switch · ↑↓ navigate          │
└───────────────────────────────────────────────────────────────────┘
```

### Menu Panel (left)

Existing menu items with notification badges:

- `Matches (1 new)` — count of new matches since last viewed
- `Inbox (3)` — count of open interest issues targeting the user

Badges poll GitHub periodically (existing 5-min pattern).

### Conversations Panel (right)

Shows active conversations (only those with messages) sorted by recency:

```
▸ d39e5f2e...  "hey!"       (2 new)
  c40d85af...  "see you"
  fab41489...  "thanks!"
```

Each entry:
- Truncated match_hash
- Latest message preview (truncated)
- Unread badge if any

Enter on a conversation → opens chat screen.

### Focus Switching

Tab toggles focus between menu and conversations panel. Active panel has highlighted cursor. Inactive panel is dimmed.

## ChatClient

Composes relay client + conversation DB. Single class, used by all screens. No screen touches relay or DB directly.

```go
// internal/cli/chat/client.go

type ChatClient struct {
    relay   *relayclient.Client
    db      *ConversationDB
    onMsg   func(peerMatchHash string)  // notify UI of new message
}

// Send encrypts + sends via relay, persists to DB as sent message
func (c *ChatClient) Send(peerMatchHash, text string) error

// HandleIncoming is the relay OnMessage callback
// Decrypts, persists to DB, increments unread, calls onMsg
func (c *ChatClient) HandleIncoming(senderMatchHash string, plaintext []byte)

// History loads messages from DB for a peer
func (c *ChatClient) History(peerMatchHash string) ([]Message, error)

// Conversations returns all active conversations for the panel
func (c *ChatClient) Conversations() ([]Conversation, error)

// MarkRead resets unread count for a peer
func (c *ChatClient) MarkRead(peerMatchHash string) error

// PersistGreeting saves a match greeting as the first message in a conversation
func (c *ChatClient) PersistGreeting(peerMatchHash, greeting string) error
```

### Greeting as First Message

When a mutual match is detected and the match notification is decrypted, the greeting is persisted as the first message in the conversation:

```go
// In matches loading flow:
chatClient.PersistGreeting(matchedMatchHash, greeting)
```

This only applies to **matched** pairs. Incoming interests (inbox) show greetings as preview but do NOT create conversations.

The greeting appears in:
- Conversations panel as the latest message (until a real message is sent)
- Chat history as the first message when opening the conversation

## ConversationDB

SQLite at `~/.dating/conversations.db` (or `$DATING_HOME/conversations.db`).

```sql
CREATE TABLE conversations (
    peer_match_hash TEXT PRIMARY KEY,
    last_message    TEXT,
    last_message_at INTEGER,
    unread_count    INTEGER DEFAULT 0
);

CREATE TABLE messages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    peer_match_hash TEXT NOT NULL,
    body            TEXT NOT NULL,
    is_me           INTEGER NOT NULL,  -- 1 = sent, 0 = received
    created_at      INTEGER NOT NULL
);

CREATE INDEX idx_messages_peer ON messages(peer_match_hash, created_at);
```

`conversations` table powers the panel (fast query for list + unread counts).
`messages` table stores full chat history (decrypted plaintext).

### Operations

```go
// internal/cli/chat/db.go

type ConversationDB struct { db *sql.DB }

func OpenConversationDB(path string) (*ConversationDB, error)

func (d *ConversationDB) SaveMessage(peerMatchHash, body string, isMe bool) error
// Inserts into messages + upserts conversations (last_message, last_message_at, unread_count++)

func (d *ConversationDB) ListConversations() ([]Conversation, error)
// SELECT * FROM conversations ORDER BY last_message_at DESC

func (d *ConversationDB) LoadHistory(peerMatchHash string) ([]Message, error)
// SELECT * FROM messages WHERE peer_match_hash = ? ORDER BY created_at

func (d *ConversationDB) MarkRead(peerMatchHash string) error
// UPDATE conversations SET unread_count = 0 WHERE peer_match_hash = ?

func (d *ConversationDB) UnreadTotal() (int, error)
// SELECT SUM(unread_count) FROM conversations
```

### Types

```go
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
```

## Background Relay Connection

The TUI App connects to the relay on startup. The relay client lives on the App struct.

```go
type App struct {
    // ... existing fields
    chatClient *chat.ChatClient
}
```

### Startup Flow

1. Load config → get pool with relay URL, match_hash, keys
2. Create `ConversationDB`
3. Create relay client with config
4. Set peer keys for all known matches (from DB or GitHub)
5. Create `ChatClient` with relay + DB
6. Connect relay in background
7. `ChatClient.onMsg` sends a `tea.Msg` to update UI

### Message Routing

```go
// ChatClient.HandleIncoming (relay OnMessage callback):
func (c *ChatClient) HandleIncoming(senderMatchHash string, plaintext []byte) {
    c.db.SaveMessage(senderMatchHash, string(plaintext), false)
    if c.onMsg != nil {
        c.onMsg(senderMatchHash)  // triggers tea.Msg → UI update
    }
}
```

Regardless of which screen is active:
- Message saved to DB
- Unread count incremented
- If on home screen → conversations panel updates
- If on chat screen with this peer → message displayed inline
- If on any other screen → conversations panel updates on next home visit

## Chat Screen Changes

No more `ConnectChatCmd`. The chat screen reuses `app.chatClient`:

- On enter: `chatClient.MarkRead(peerMatchHash)` + load history from DB
- Send: `chatClient.Send(peerMatchHash, text)`
- Receive: handled by background relay → `ChatIncomingMsg` from `onMsg` callback
- History: `chatClient.History(peerMatchHash)` — shows all past messages including greeting

## Inbox Notifications

Poll GitHub periodically (existing 5-min pattern) for open interest issues:

```go
count, _ := pool.ListInterestsForMeIssues(ctx, myMatchHash)
// Update inbox badge: "Inbox (3)"
```

Interests show as a count + list with greeting preview. No conversation created until mutual match.

## Files

### New Files
- `internal/cli/chat/client.go` — ChatClient
- `internal/cli/chat/db.go` — ConversationDB (SQLite)
- `internal/cli/chat/db_test.go` — DB tests
- `internal/cli/chat/client_test.go` — ChatClient tests

### Modified Files
- `internal/cli/tui/app.go` — App holds ChatClient, connects relay on startup, routes messages
- `internal/cli/tui/screens/home.go` (or wherever home screen is) — split panel layout, tab focus switching
- `internal/cli/tui/screens/chat.go` — remove ConnectChatCmd, use ChatClient, load history from DB
- `internal/cli/tui/screens/matches.go` — persist greeting via ChatClient.PersistGreeting on match load

## Testing

- Unit tests: ConversationDB (save, list, history, mark read, unread total)
- Unit tests: ChatClient (send persists, incoming persists + increments unread, mark read resets)
- Unit tests: PersistGreeting (creates conversation + first message, idempotent)
- TUI tests: home screen renders conversations panel, tab switches focus
- TUI tests: notification badges update on new messages
- Integration: background relay delivers message → DB updated → panel reflects
