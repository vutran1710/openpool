# TUI Notifications & Conversations Panel Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add background relay connection, local message persistence, conversations panel on home screen, and notification badges — so users see incoming messages in real-time without entering the chat screen.

**Architecture:** New `ChatClient` (logic layer) composes relay client + ConversationDB (SQLite). App creates ChatClient on startup, connects relay in background. Screens are pure views — they read from ChatClient and send actions via tea.Msg. Home screen gets a split layout: menu (left) + conversations panel (right) with tab to switch focus.

**Tech Stack:** Go, Bubbletea, SQLite (`modernc.org/sqlite`), gorilla/websocket

---

## File Structure

### New Files
- `internal/cli/chat/db.go` — ConversationDB (SQLite operations)
- `internal/cli/chat/db_test.go` — DB tests
- `internal/cli/chat/client.go` — ChatClient (relay + DB composition)
- `internal/cli/chat/client_test.go` — ChatClient tests
- `internal/cli/tui/components/conversations.go` — Conversations panel component

### Modified Files
- `internal/cli/tui/app.go` — App holds ChatClient, connects relay on startup, routes messages
- `internal/cli/tui/screens/home.go` — Split layout: menu + conversations panel, tab focus
- `internal/cli/tui/screens/chat.go` — Remove ConnectChatCmd, use ChatClient, load history from DB
- `internal/cli/tui/screens/matches.go` — Persist greeting via ChatClient on match load

---

### Task 1: ConversationDB (SQLite)

**Files:**
- Create: `internal/cli/chat/db.go`
- Create: `internal/cli/chat/db_test.go`

- [ ] **Step 1: Write DB tests**

```go
package chat

import (
    "os"
    "path/filepath"
    "testing"
)

func testDB(t *testing.T) *ConversationDB {
    t.Helper()
    dir := t.TempDir()
    db, err := OpenConversationDB(filepath.Join(dir, "test.db"))
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { db.Close() })
    return db
}

func TestSaveMessage_AndHistory(t *testing.T) {
    db := testDB(t)
    db.SaveMessage("peer1", "hello", true)
    db.SaveMessage("peer1", "hi back", false)

    msgs, _ := db.LoadHistory("peer1")
    if len(msgs) != 2 { t.Fatalf("expected 2 messages, got %d", len(msgs)) }
    if msgs[0].Body != "hello" || msgs[0].IsMe != true { t.Fatal("first message wrong") }
    if msgs[1].Body != "hi back" || msgs[1].IsMe != false { t.Fatal("second message wrong") }
}

func TestSaveMessage_UpdatesConversation(t *testing.T) {
    db := testDB(t)
    db.SaveMessage("peer1", "first", false)
    db.SaveMessage("peer1", "second", false)

    convos, _ := db.ListConversations()
    if len(convos) != 1 { t.Fatalf("expected 1 conversation, got %d", len(convos)) }
    if convos[0].LastMessage != "second" { t.Fatalf("last message = %q", convos[0].LastMessage) }
    if convos[0].UnreadCount != 2 { t.Fatalf("unread = %d", convos[0].UnreadCount) }
}

func TestSaveMessage_SentDoesNotIncrementUnread(t *testing.T) {
    db := testDB(t)
    db.SaveMessage("peer1", "my message", true)  // sent by me

    convos, _ := db.ListConversations()
    if convos[0].UnreadCount != 0 { t.Fatalf("sent messages should not be unread") }
}

func TestMarkRead(t *testing.T) {
    db := testDB(t)
    db.SaveMessage("peer1", "msg", false)
    db.SaveMessage("peer1", "msg2", false)
    db.MarkRead("peer1")

    convos, _ := db.ListConversations()
    if convos[0].UnreadCount != 0 { t.Fatal("should be 0 after mark read") }
}

func TestUnreadTotal(t *testing.T) {
    db := testDB(t)
    db.SaveMessage("peer1", "a", false)
    db.SaveMessage("peer2", "b", false)
    db.SaveMessage("peer2", "c", false)

    total, _ := db.UnreadTotal()
    if total != 3 { t.Fatalf("expected 3, got %d", total) }
}

func TestListConversations_SortedByRecency(t *testing.T) {
    db := testDB(t)
    db.SaveMessage("peer1", "old", false)
    db.SaveMessage("peer2", "newer", false)

    convos, _ := db.ListConversations()
    if convos[0].PeerMatchHash != "peer2" { t.Fatal("newest should be first") }
}

func TestPersistGreeting_Idempotent(t *testing.T) {
    db := testDB(t)
    db.PersistGreeting("peer1", "Hello!")
    db.PersistGreeting("peer1", "Hello!")  // should not duplicate

    msgs, _ := db.LoadHistory("peer1")
    if len(msgs) != 1 { t.Fatalf("greeting should be persisted once, got %d", len(msgs)) }
}
```

- [ ] **Step 2: Implement ConversationDB**

```go
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
    // Check if any messages exist for this peer
    var count int
    d.db.QueryRow("SELECT COUNT(*) FROM messages WHERE peer_match_hash = ?", peerMatchHash).Scan(&count)
    if count > 0 {
        return nil // greeting already persisted or conversation exists
    }
    return d.SaveMessage(peerMatchHash, greeting, false)
}

func boolToInt(b bool) int {
    if b { return 1 }
    return 0
}
```

- [ ] **Step 3: Run tests**

Run: `DATING_HOME=$(mktemp -d) go test -v ./internal/cli/chat/ -count=1`

- [ ] **Step 4: Commit**

```bash
git add internal/cli/chat/
git commit -m "feat: ConversationDB — SQLite message persistence"
```

---

### Task 2: ChatClient

**Files:**
- Create: `internal/cli/chat/client.go`
- Create: `internal/cli/chat/client_test.go`

- [ ] **Step 1: Implement ChatClient**

```go
package chat

import (
    "crypto/ed25519"
    "log"

    relayclient "github.com/vutran1710/dating-dev/internal/cli/relay"
)

type ChatClient struct {
    Relay *relayclient.Client
    DB    *ConversationDB
    OnMsg func(peerMatchHash string) // notify UI of new message
}

func NewChatClient(relay *relayclient.Client, db *ConversationDB) *ChatClient {
    c := &ChatClient{Relay: relay, DB: db}

    // Wire up relay callback
    relay.OnMessage(func(senderMatchHash string, plaintext []byte) {
        c.handleIncoming(senderMatchHash, plaintext)
    })

    return c
}

func (c *ChatClient) handleIncoming(senderMatchHash string, plaintext []byte) {
    if err := c.DB.SaveMessage(senderMatchHash, string(plaintext), false); err != nil {
        log.Printf("save incoming: %v", err)
    }
    if c.OnMsg != nil {
        c.OnMsg(senderMatchHash)
    }
}

func (c *ChatClient) Send(peerMatchHash, text string) error {
    if err := c.Relay.SendMessage(peerMatchHash, text); err != nil {
        return err
    }
    return c.DB.SaveMessage(peerMatchHash, text, true)
}

func (c *ChatClient) History(peerMatchHash string) ([]Message, error) {
    return c.DB.LoadHistory(peerMatchHash)
}

func (c *ChatClient) Conversations() ([]Conversation, error) {
    return c.DB.ListConversations()
}

func (c *ChatClient) MarkRead(peerMatchHash string) error {
    return c.DB.MarkRead(peerMatchHash)
}

func (c *ChatClient) PersistGreeting(peerMatchHash, greeting string) error {
    return c.DB.PersistGreeting(peerMatchHash, greeting)
}

func (c *ChatClient) UnreadTotal() (int, error) {
    return c.DB.UnreadTotal()
}

func (c *ChatClient) SetPeerKey(peerMatchHash string, peerPub ed25519.PublicKey) {
    c.Relay.SetPeerKey(peerMatchHash, peerPub)
}
```

- [ ] **Step 2: Write tests**

Test that Send persists, HandleIncoming persists + increments unread, MarkRead resets, PersistGreeting is idempotent. Use a real DB (temp dir) + a mock relay client (or nil relay for DB-only tests).

- [ ] **Step 3: Run tests**

Run: `DATING_HOME=$(mktemp -d) go test -v ./internal/cli/chat/ -count=1`

- [ ] **Step 4: Commit**

```bash
git add internal/cli/chat/
git commit -m "feat: ChatClient — relay + DB composition"
```

---

### Task 3: Conversations panel component

**Files:**
- Create: `internal/cli/tui/components/conversations.go`

- [ ] **Step 1: Implement conversations panel**

A Bubbletea component that renders a list of conversations with unread badges:

```go
package components

import (
    "fmt"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/vutran1710/dating-dev/internal/cli/chat"
    "github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

// ConversationSelectMsg is emitted when user presses enter on a conversation
type ConversationSelectMsg struct {
    PeerMatchHash string
}

type ConversationsPanel struct {
    conversations []chat.Conversation
    cursor        int
    focused       bool
    width         int
    height        int
}

func NewConversationsPanel() ConversationsPanel {
    return ConversationsPanel{}
}

func (p ConversationsPanel) SetConversations(convos []chat.Conversation) ConversationsPanel {
    p.conversations = convos
    if p.cursor >= len(convos) {
        p.cursor = max(0, len(convos)-1)
    }
    return p
}

func (p ConversationsPanel) SetFocused(focused bool) ConversationsPanel {
    p.focused = focused
    return p
}

func (p ConversationsPanel) SetSize(w, h int) ConversationsPanel {
    p.width = w
    p.height = h
    return p
}

func (p ConversationsPanel) Update(msg tea.Msg) (ConversationsPanel, tea.Cmd) {
    if !p.focused {
        return p, nil
    }
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "up", "k":
            if p.cursor > 0 { p.cursor-- }
        case "down", "j":
            if p.cursor < len(p.conversations)-1 { p.cursor++ }
        case "enter":
            if p.cursor < len(p.conversations) {
                return p, func() tea.Msg {
                    return ConversationSelectMsg{
                        PeerMatchHash: p.conversations[p.cursor].PeerMatchHash,
                    }
                }
            }
        }
    }
    return p, nil
}

func (p ConversationsPanel) View() string {
    if len(p.conversations) == 0 {
        return theme.DimStyle.Render("  No conversations yet")
    }

    var b strings.Builder
    for i, c := range p.conversations {
        cursor := "  "
        nameStyle := theme.TextStyle
        if p.focused && i == p.cursor {
            cursor = theme.BrandStyle.Render("▸ ")
            nameStyle = theme.BoldStyle
        }

        hash := c.PeerMatchHash
        if len(hash) > 12 {
            hash = hash[:12] + "..."
        }

        msg := c.LastMessage
        if len(msg) > 20 {
            msg = msg[:17] + "..."
        }

        line := fmt.Sprintf("%s%s  %s", cursor, nameStyle.Render(hash), theme.DimStyle.Render("\""+msg+"\""))

        if c.UnreadCount > 0 {
            line += " " + theme.BrandStyle.Render(fmt.Sprintf("(%d)", c.UnreadCount))
        }

        b.WriteString(line + "\n")
    }
    return b.String()
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/cli/tui/components/conversations.go
git commit -m "feat: ConversationsPanel — conversations list component"
```

---

### Task 4: Update home screen — split layout with tab focus

**Files:**
- Modify: `internal/cli/tui/screens/home.go`

- [ ] **Step 1: Read the current home.go**

Understand the current `HomeScreen` struct and `View()`.

- [ ] **Step 2: Add conversations panel + focus state**

Add to `HomeScreen`:
```go
type HomeScreen struct {
    menu          components.Menu
    conversations components.ConversationsPanel
    menuFocused   bool  // true = menu focused, false = conversations focused
}
```

Default: `menuFocused = true`.

- [ ] **Step 3: Handle Tab key**

In `Update()`, on `tab`:
```go
case "tab":
    s.menuFocused = !s.menuFocused
    s.conversations = s.conversations.SetFocused(!s.menuFocused)
    // Also update menu focus state
```

- [ ] **Step 4: Handle ConversationSelectMsg**

When user selects a conversation → emit a message that app.go routes to chat screen. Reuse `MatchChatMsg` or create a new `ConversationOpenMsg`:

```go
case components.ConversationSelectMsg:
    return s, func() tea.Msg {
        return ConversationOpenMsg{PeerMatchHash: msg.PeerMatchHash}
    }
```

- [ ] **Step 5: Split View rendering**

Use `lipgloss.JoinHorizontal` to render menu (left) and conversations (right):

```go
func (s HomeScreen) View() string {
    leftWidth := s.Width / 2
    rightWidth := s.Width - leftWidth

    left := s.menu.View()  // existing menu
    right := "Conversations\n" + s.conversations.View()

    // Join with a vertical border
    return lipgloss.JoinHorizontal(lipgloss.Top,
        lipgloss.NewStyle().Width(leftWidth).Render(left),
        lipgloss.NewStyle().Width(rightWidth).BorderLeft(true).
            BorderStyle(lipgloss.NormalBorder()).
            BorderForeground(theme.Border).
            PaddingLeft(1).Render(right),
    )
}
```

- [ ] **Step 6: Add notification badges to menu items**

The menu items need dynamic labels. Add a method to set badge counts:
```go
s.menu.SetBadge("matches", matchCount)
s.menu.SetBadge("inbox", inboxCount)
```

Or simpler: update the menu item descriptions when counts change:
```go
// When conversations update:
items[1].Desc = fmt.Sprintf("view your matches (%d new)", newMatchCount)
items[2].Desc = fmt.Sprintf("incoming interests (%d)", inboxCount)
```

- [ ] **Step 7: Commit**

```bash
git add internal/cli/tui/screens/home.go
git commit -m "feat: split home screen — menu + conversations panel with tab focus"
```

---

### Task 5: Wire ChatClient into App

**Files:**
- Modify: `internal/cli/tui/app.go`

- [ ] **Step 1: Add ChatClient to app struct**

```go
type app struct {
    // ... existing fields
    chatClient *chat.ChatClient
}
```

- [ ] **Step 2: Initialize ChatClient on startup**

In the app initialization (after onboarding, when pool is active):
```go
// Open conversations DB
convoDB, _ := chat.OpenConversationDB(filepath.Join(config.Dir(), "conversations.db"))

// Create relay client
relayClient := relayclient.NewClient(relayclient.Config{
    RelayURL:  pool.RelayURL,
    PoolURL:   pool.Repo,
    IDHash:    pool.IDHash,
    MatchHash: pool.MatchHash,
    Pub:       pub,
    Priv:      priv,
})

// Create ChatClient
a.chatClient = chat.NewChatClient(relayClient, convoDB)

// Set onMsg callback → push tea.Msg to update UI
a.chatClient.OnMsg = func(peerMatchHash string) {
    a.program.Send(ChatIncomingMsg{SenderMatchHash: peerMatchHash})
}

// Connect relay in background
go func() {
    ctx := context.Background()
    if err := relayClient.Connect(ctx); err != nil {
        log.Printf("relay connect: %v", err)
    }
}()
```

Note: `a.program` is the `*tea.Program` — need to store it on the app struct for `Send()`.

- [ ] **Step 3: Route ChatIncomingMsg to update conversations panel**

In `app.Update()`:
```go
case ChatIncomingMsg:
    // Refresh conversations panel on home screen
    convos, _ := a.chatClient.Conversations()
    a.home = a.home.SetConversations(convos)
    // If on chat screen with this peer, display the message
    if a.screen == screenChat && a.chat.TargetID == msg.SenderMatchHash {
        // Load latest message from DB and append to chat viewport
    }
```

- [ ] **Step 4: Route ConversationOpenMsg to chat screen**

```go
case screens.ConversationOpenMsg:
    a.screen = screenChat
    a.chat = screens.NewChatScreen(msg.PeerMatchHash, a.width, a.height)
    // Load history from DB
    history, _ := a.chatClient.History(msg.PeerMatchHash)
    a.chat.LoadHistory(history)
    a.chatClient.MarkRead(msg.PeerMatchHash)
    a.updateHelp()
```

- [ ] **Step 5: Update MatchChatMsg handler**

Existing handler creates ConnectChatCmd. Now just open chat with history:
```go
case screens.MatchChatMsg:
    a.screen = screenChat
    a.chat = screens.NewChatScreen(msg.MatchHash, a.width, a.height)
    a.chatClient.SetPeerKey(msg.MatchHash, msg.PubKey)
    history, _ := a.chatClient.History(msg.MatchHash)
    a.chat.LoadHistory(history)
    a.chatClient.MarkRead(msg.MatchHash)
```

- [ ] **Step 6: Commit**

```bash
git add internal/cli/tui/app.go
git commit -m "feat: wire ChatClient into App — background relay, conversation routing"
```

---

### Task 6: Update chat screen — use ChatClient, load history

**Files:**
- Modify: `internal/cli/tui/screens/chat.go`

- [ ] **Step 1: Remove ConnectChatCmd**

Delete `ConnectChatCmd` function entirely. The relay connection is now global.

- [ ] **Step 2: Remove relay client from ChatScreen**

```go
type ChatScreen struct {
    TargetID  string
    Messages  []ChatMessage
    Viewport  viewport.Model
    Width, Height int
    Ready     bool
    // Remove: client, connected, err
}
```

The chat screen is now a pure view — no relay client, no connection state.

- [ ] **Step 3: Add LoadHistory method**

```go
func (s *ChatScreen) LoadHistory(msgs []chat.Message) {
    s.Messages = nil
    for _, m := range msgs {
        sender := "you"
        isMe := m.IsMe
        if !isMe {
            sender = s.TargetID
            if len(sender) > 12 {
                sender = sender[:12] + "..."
            }
        }
        s.Messages = append(s.Messages, ChatMessage{
            Sender: sender,
            Body:   m.Body,
            Time:   m.CreatedAt.Format("15:04"),
            IsMe:   isMe,
        })
    }
    s.Ready = true
    s.Viewport.SetContent(s.renderMessages())
    s.Viewport.GotoBottom()
}
```

- [ ] **Step 4: Update SubmitMsg handler**

Instead of `s.client.SendMessage()`, emit a `ChatSendMsg`:
```go
case components.SubmitMsg:
    if msg.Value != "" {
        return s, func() tea.Msg {
            return ChatSendMsg{PeerMatchHash: s.TargetID, Text: msg.Value}
        }
    }
```

App.go handles `ChatSendMsg`:
```go
case screens.ChatSendMsg:
    a.chatClient.Send(msg.PeerMatchHash, msg.Text)
    // Append to chat screen
    a.chat.AppendMessage(msg.Text, true)
```

- [ ] **Step 5: Update incoming message handler**

`ChatIncomingMsg` now carries just the peer hash. App loads the latest message from DB and appends:

```go
case ChatIncomingMsg:
    if a.screen == screenChat && a.chat.TargetID == msg.SenderMatchHash {
        // Get latest message from DB
        history, _ := a.chatClient.History(msg.SenderMatchHash)
        if len(history) > 0 {
            latest := history[len(history)-1]
            a.chat.AppendMessage(latest.Body, false)
        }
    }
```

- [ ] **Step 6: Add AppendMessage helper**

```go
func (s *ChatScreen) AppendMessage(body string, isMe bool) {
    sender := "you"
    if !isMe {
        sender = s.TargetID
        if len(sender) > 12 { sender = sender[:12] + "..." }
    }
    s.Messages = append(s.Messages, ChatMessage{
        Sender: sender,
        Body:   body,
        Time:   time.Now().Format("15:04"),
        IsMe:   isMe,
    })
    s.Viewport.SetContent(s.renderMessages())
    s.Viewport.GotoBottom()
}
```

- [ ] **Step 7: Commit**

```bash
git add internal/cli/tui/screens/chat.go
git commit -m "refactor: chat screen — pure view, no relay client, history from DB"
```

---

### Task 7: Persist greeting on match load

**Files:**
- Modify: `internal/cli/tui/screens/matches.go`

- [ ] **Step 1: Update LoadMatchesCmd**

After loading matches, persist each greeting to the ChatClient. The matches screen needs access to the ChatClient — pass it as a parameter or via a tea.Msg.

Simplest: `LoadMatchesCmd` returns matches as before, then in app.go when `MatchesFetchedMsg` is handled:

```go
case screens.MatchesFetchedMsg:
    // Existing: update matches screen
    // New: persist greetings
    for _, m := range msg.Matches {
        if m.Greeting != "" && a.chatClient != nil {
            a.chatClient.PersistGreeting(m.MatchHash, m.Greeting)
            a.chatClient.SetPeerKey(m.MatchHash, m.PubKey)
        }
    }
    // Refresh conversations panel
    convos, _ := a.chatClient.Conversations()
    a.home = a.home.SetConversations(convos)
```

- [ ] **Step 2: Commit**

```bash
git add internal/cli/tui/app.go internal/cli/tui/screens/matches.go
git commit -m "feat: persist match greeting as first conversation message"
```

---

### Task 8: Full build + test suite

- [ ] **Step 1: Fix compile errors**

Run: `go build ./...`

Fix any remaining issues from the refactor (removed ConnectChatCmd references, new message types, etc.).

- [ ] **Step 2: Run all tests**

Run: `DATING_HOME=$(mktemp -d) go test ./... -count=1`

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "fix: all compile errors and tests pass"
```

---

### Task 9: E2E test — TUI with conversations panel

- [ ] **Step 1: Set up two users (same as previous E2E)**

Use the same e2etest pattern: two DATING_HOME dirs, .bin files on GitHub, match file, signed comments.

- [ ] **Step 2: Launch two TUI instances in tmux**

- [ ] **Step 3: Verify conversations panel**

- Navigate to Matches → greeting should appear
- Go back to Home → conversations panel should show the greeting
- Send a message from A → B's conversations panel should update (if on home)
- Tab to conversations panel → enter → opens chat with history

- [ ] **Step 4: Commit**

```bash
git commit -m "test: E2E verified conversations panel + notifications"
```
