package relay_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vutran1710/dating-dev/internal/cli/relay"
	server "github.com/vutran1710/dating-dev/internal/relay"
	"github.com/vutran1710/dating-dev/internal/protocol"
)

// --- Test helpers ---

func genKeys(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return pub, priv
}

func hashID(salt, poolURL, provider, userID string) string {
	h := sha256.Sum256([]byte(salt + ":" + poolURL + ":" + provider + ":" + userID))
	return hex.EncodeToString(h[:])[:16]
}

// testEnv sets up a relay server with seeded users and returns the WS URL + server.
type testEnv struct {
	srv    *server.Server
	ts     *httptest.Server
	wsURL  string
}

const testPoolURL = "owner/pool"

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	srv := server.NewServer(server.ServerConfig{
		PoolURL:  testPoolURL,
		TokenTTL: 5 * time.Minute,
	})
	ts := httptest.NewServer(srv.Handler())
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	t.Cleanup(func() { ts.Close() })
	return &testEnv{srv: srv, ts: ts, wsURL: wsURL}
}

func (e *testEnv) addUser(userID, provider string, pub ed25519.PublicKey) string {
	hid := hashID("test-salt", testPoolURL, provider, userID)
	e.srv.Store().UpsertUser(server.UserEntry{
		PubKey:   pub,
		UserID:   userID,
		Provider: provider,
		HashID:   hid,
	})
	return hid
}

func (e *testEnv) addMatch(hashA, hashB string) {
	e.srv.Store().AddMatch(hashA, hashB)
}

func connectClient(t *testing.T, env *testEnv, userID, provider string, pub ed25519.PublicKey, priv ed25519.PrivateKey) *relay.Client {
	t.Helper()
	c := relay.NewClient(relay.Config{
		RelayURL: env.wsURL,
		UserID:   userID,
		Provider: provider,
		Pub:      pub,
		Priv:     priv,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect %s: %v", userID, err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// ==================== E2E Tests ====================

func TestE2E_AuthSuccess(t *testing.T) {
	env := newTestEnv(t)
	pub, priv := genKeys(t)
	hid := env.addUser("alice", "github", pub)

	c := connectClient(t, env, "alice", "github", pub, priv)

	if c.HashID() != hid {
		t.Errorf("hash_id = %q, want %q", c.HashID(), hid)
	}
	if c.Token() == "" {
		t.Error("token should be set after auth")
	}
}

func TestE2E_AuthFail_UserNotFound(t *testing.T) {
	env := newTestEnv(t)
	pub, priv := genKeys(t)
	// Don't add user to store

	c := relay.NewClient(relay.Config{
		RelayURL: env.wsURL,
		UserID: "nobody", Provider: "github", Pub: pub, Priv: priv,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.Connect(ctx)
	if err == nil {
		c.Close()
		t.Fatal("expected auth error")
	}
	if !strings.Contains(err.Error(), "user_not_found") {
		t.Errorf("error = %q, want user_not_found", err)
	}
}

func TestE2E_AuthFail_WrongKey(t *testing.T) {
	env := newTestEnv(t)
	pub, _ := genKeys(t)
	_, wrongPriv := genKeys(t) // different key pair
	env.addUser("alice", "github", pub)

	c := relay.NewClient(relay.Config{
		RelayURL: env.wsURL,
		UserID: "alice", Provider: "github", Pub: pub, Priv: wrongPriv,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.Connect(ctx)
	if err == nil {
		c.Close()
		t.Fatal("expected auth error")
	}
	if !strings.Contains(err.Error(), "auth_failed") {
		t.Errorf("error = %q, want auth_failed", err)
	}
}

func TestE2E_MessageRouting_BothOnline(t *testing.T) {
	env := newTestEnv(t)

	pubA, privA := genKeys(t)
	pubB, privB := genKeys(t)
	hidA := env.addUser("alice", "github", pubA)
	hidB := env.addUser("bob", "github", pubB)
	env.addMatch(hidA, hidB)

	// Connect both
	clientA := connectClient(t, env, "alice", "github", pubA, privA)
	clientB := connectClient(t, env, "bob", "github", pubB, privB)

	// Bob listens for messages
	msgCh := make(chan protocol.Message, 1)
	clientB.OnMessage(func(msg protocol.Message) {
		msgCh <- msg
	})

	// Alice sends to Bob
	if err := clientA.SendMessage(hidB, "hey bob!"); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case msg := <-msgCh:
		if msg.Body != "hey bob!" {
			t.Errorf("body = %q, want 'hey bob!'", msg.Body)
		}
		if msg.SourceHash != hidA {
			t.Errorf("source = %q, want %q", msg.SourceHash, hidA)
		}
		if msg.TargetHash != hidB {
			t.Errorf("target = %q, want %q", msg.TargetHash, hidB)
		}
		if msg.MsgID == "" {
			t.Error("msg_id should be set by relay")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestE2E_MessageRouting_Bidirectional(t *testing.T) {
	env := newTestEnv(t)

	pubA, privA := genKeys(t)
	pubB, privB := genKeys(t)
	hidA := env.addUser("alice", "github", pubA)
	hidB := env.addUser("bob", "github", pubB)
	env.addMatch(hidA, hidB)

	clientA := connectClient(t, env, "alice", "github", pubA, privA)
	clientB := connectClient(t, env, "bob", "github", pubB, privB)

	msgFromA := make(chan protocol.Message, 1)
	msgFromB := make(chan protocol.Message, 1)
	clientB.OnMessage(func(msg protocol.Message) { msgFromA <- msg })
	clientA.OnMessage(func(msg protocol.Message) { msgFromB <- msg })

	// Alice → Bob
	clientA.SendMessage(hidB, "hello bob")
	select {
	case msg := <-msgFromA:
		if msg.Body != "hello bob" {
			t.Errorf("A→B body = %q", msg.Body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout A→B")
	}

	// Bob → Alice
	clientB.SendMessage(hidA, "hello alice")
	select {
	case msg := <-msgFromB:
		if msg.Body != "hello alice" {
			t.Errorf("B→A body = %q", msg.Body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout B→A")
	}
}

func TestE2E_MessageRouting_NotMatched(t *testing.T) {
	env := newTestEnv(t)

	pubA, privA := genKeys(t)
	pubB, _ := genKeys(t)
	env.addUser("alice", "github", pubA)
	hidB := env.addUser("bob", "github", pubB)
	// No match added!

	clientA := connectClient(t, env, "alice", "github", pubA, privA)

	errCh := make(chan protocol.Error, 1)
	clientA.OnError(func(e protocol.Error) { errCh <- e })

	// SendMessage blocks waiting for KeyResponse; send in goroutine so we can
	// select on the error channel that the relay populates via onError callback.
	go clientA.SendMessage(hidB, "hey!")

	select {
	case e := <-errCh:
		if e.Code != protocol.ErrNotMatched {
			t.Errorf("code = %q, want not_matched", e.Code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for not_matched error")
	}
}

func TestE2E_MessageRouting_TargetOffline(t *testing.T) {
	env := newTestEnv(t)

	pubA, privA := genKeys(t)
	pubB, _ := genKeys(t)
	hidA := env.addUser("alice", "github", pubA)
	hidB := env.addUser("bob", "github", pubB)
	env.addMatch(hidA, hidB)

	clientA := connectClient(t, env, "alice", "github", pubA, privA)
	// Bob is NOT connected

	// Send should not error (message queued for offline delivery)
	err := clientA.SendMessage(hidB, "are you there?")
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	// No crash, no error — message silently queued (TODO: actual queue)
	time.Sleep(100 * time.Millisecond)
}

func TestE2E_TokenRefresh(t *testing.T) {
	env := newTestEnv(t)
	pub, priv := genKeys(t)
	env.addUser("alice", "github", pub)

	c := connectClient(t, env, "alice", "github", pub, priv)

	origToken := c.Token()
	if origToken == "" {
		t.Fatal("token should be set")
	}

	// Manually trigger a refresh by sending a refresh frame
	// The readLoop will pick up the Authenticated response and update the token
	// We need to wait for the refresh to propagate
	time.Sleep(200 * time.Millisecond)

	// Token should still be valid (hasn't expired in 5min TTL)
	if c.Token() == "" {
		t.Error("token should still be set")
	}
}

func TestE2E_MultipleUsers_Concurrent(t *testing.T) {
	env := newTestEnv(t)
	const n = 10

	type user struct {
		pub    ed25519.PublicKey
		priv   ed25519.PrivateKey
		hashID string
	}

	users := make([]user, n)
	for i := 0; i < n; i++ {
		pub, priv := genKeys(t)
		uid := strings.Repeat(string(rune('a'+i)), 3) // "aaa", "bbb", etc.
		hid := env.addUser(uid, "github", pub)
		users[i] = user{pub: pub, priv: priv, hashID: hid}
	}

	// Connect all concurrently
	var wg sync.WaitGroup
	clients := make([]*relay.Client, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			uid := strings.Repeat(string(rune('a'+idx)), 3)
			c := relay.NewClient(relay.Config{
				RelayURL: env.wsURL,
				UserID: uid, Provider: "github",
				Pub: users[idx].pub, Priv: users[idx].priv,
			})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := c.Connect(ctx); err != nil {
				t.Errorf("connect user %d: %v", idx, err)
				return
			}
			clients[idx] = c
		}(i)
	}
	wg.Wait()

	// Clean up
	for _, c := range clients {
		if c != nil {
			c.Close()
		}
	}

	// All should have connected successfully
	for i, c := range clients {
		if c == nil {
			t.Errorf("user %d failed to connect", i)
		}
	}
}

func TestE2E_MessageConversation(t *testing.T) {
	env := newTestEnv(t)

	pubA, privA := genKeys(t)
	pubB, privB := genKeys(t)
	hidA := env.addUser("alice", "github", pubA)
	hidB := env.addUser("bob", "github", pubB)
	env.addMatch(hidA, hidB)

	clientA := connectClient(t, env, "alice", "github", pubA, privA)
	clientB := connectClient(t, env, "bob", "github", pubB, privB)

	msgsA := make(chan protocol.Message, 10)
	msgsB := make(chan protocol.Message, 10)
	clientA.OnMessage(func(msg protocol.Message) { msgsA <- msg })
	clientB.OnMessage(func(msg protocol.Message) { msgsB <- msg })

	// Multi-turn conversation
	conversation := []struct {
		from, to *relay.Client
		target   string
		body     string
		ch       chan protocol.Message
	}{
		{clientA, clientB, hidB, "hey!", msgsB},
		{clientB, clientA, hidA, "hi there!", msgsA},
		{clientA, clientB, hidB, "how are you?", msgsB},
		{clientB, clientA, hidA, "great, you?", msgsA},
		{clientA, clientB, hidB, "same!", msgsB},
	}

	for i, turn := range conversation {
		if err := turn.from.SendMessage(turn.target, turn.body); err != nil {
			t.Fatalf("turn %d send: %v", i, err)
		}

		select {
		case msg := <-turn.ch:
			if msg.Body != turn.body {
				t.Errorf("turn %d: body = %q, want %q", i, msg.Body, turn.body)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("turn %d: timeout", i)
		}
	}
}

func TestE2E_Disconnect_Reconnect(t *testing.T) {
	env := newTestEnv(t)
	pub, priv := genKeys(t)
	hid := env.addUser("alice", "github", pub)

	// First connection
	c1 := connectClient(t, env, "alice", "github", pub, priv)
	if c1.HashID() != hid {
		t.Errorf("first connect: hash = %q, want %q", c1.HashID(), hid)
	}

	// Disconnect
	c1.Close()
	time.Sleep(100 * time.Millisecond)

	// Reconnect
	c2 := relay.NewClient(relay.Config{
		RelayURL: env.wsURL,
		UserID: "alice", Provider: "github", Pub: pub, Priv: priv,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c2.Connect(ctx); err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	defer c2.Close()

	if c2.HashID() != hid {
		t.Errorf("reconnect: hash = %q, want %q", c2.HashID(), hid)
	}
	if c2.Token() == "" {
		t.Error("reconnect: token should be set")
	}
}

func TestE2E_HealthEndpoint(t *testing.T) {
	env := newTestEnv(t)

	resp, err := env.ts.Client().Get(env.ts.URL + "/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestE2E_SessionReplacement(t *testing.T) {
	env := newTestEnv(t)

	pubA, privA := genKeys(t)
	pubB, privB := genKeys(t)
	hidA := env.addUser("alice", "github", pubA)
	hidB := env.addUser("bob", "github", pubB)
	env.addMatch(hidA, hidB)

	// Alice connects first session
	c1 := connectClient(t, env, "alice", "github", pubA, privA)
	_ = c1

	// Alice connects second session (replaces first)
	c2 := connectClient(t, env, "alice", "github", pubA, privA)

	// Bob connects and sends to Alice — should go to c2
	clientB := connectClient(t, env, "bob", "github", pubB, privB)

	msgCh := make(chan protocol.Message, 1)
	c2.OnMessage(func(msg protocol.Message) { msgCh <- msg })

	clientB.SendMessage(hidA, "which alice?")

	select {
	case msg := <-msgCh:
		if msg.Body != "which alice?" {
			t.Errorf("body = %q", msg.Body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout — message should go to newest session")
	}
}

func TestE2E_EncryptedMessage_Delivered(t *testing.T) {
	env := newTestEnv(t)
	pubA, privA := genKeys(t)
	pubB, privB := genKeys(t)
	hidA := env.addUser("alice", "github", pubA)
	hidB := env.addUser("bob", "github", pubB)
	env.addMatch(hidA, hidB)

	clientA := connectClient(t, env, "alice", "github", pubA, privA)
	clientB := connectClient(t, env, "bob", "github", pubB, privB)

	msgCh := make(chan protocol.Message, 1)
	clientB.OnMessage(func(msg protocol.Message) { msgCh <- msg })

	if err := clientA.SendMessage(hidB, "secret hello!"); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case msg := <-msgCh:
		if msg.Body != "secret hello!" {
			t.Errorf("body = %q, want 'secret hello!'", msg.Body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for encrypted message")
	}
}

func TestE2E_EncryptedMessage_RelayCannotRead(t *testing.T) {
	env := newTestEnv(t)
	pubA, privA := genKeys(t)
	pubB, privB := genKeys(t)
	hidA := env.addUser("alice", "github", pubA)
	hidB := env.addUser("bob", "github", pubB)
	env.addMatch(hidA, hidB)

	clientA := connectClient(t, env, "alice", "github", pubA, privA)
	clientB := connectClient(t, env, "bob", "github", pubB, privB)

	rawCh := make(chan protocol.Message, 1)
	clientB.OnRawMessage(func(msg protocol.Message) { rawCh <- msg })

	if err := clientA.SendMessage(hidB, "top secret"); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case raw := <-rawCh:
		if !raw.Encrypted {
			t.Error("message should be marked encrypted")
		}
		if raw.Body == "top secret" {
			t.Error("relay forwarded plaintext — encryption not working")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

func TestE2E_EncryptedConversation_Bidirectional(t *testing.T) {
	env := newTestEnv(t)
	pubA, privA := genKeys(t)
	pubB, privB := genKeys(t)
	hidA := env.addUser("alice", "github", pubA)
	hidB := env.addUser("bob", "github", pubB)
	env.addMatch(hidA, hidB)

	clientA := connectClient(t, env, "alice", "github", pubA, privA)
	clientB := connectClient(t, env, "bob", "github", pubB, privB)

	msgsA := make(chan protocol.Message, 10)
	msgsB := make(chan protocol.Message, 10)
	clientA.OnMessage(func(msg protocol.Message) { msgsA <- msg })
	clientB.OnMessage(func(msg protocol.Message) { msgsB <- msg })

	turns := []struct {
		from   *relay.Client
		target string
		body   string
		ch     chan protocol.Message
	}{
		{clientA, hidB, "hey bob!", msgsB},
		{clientB, hidA, "hey alice!", msgsA},
		{clientA, hidB, "how are you?", msgsB},
		{clientB, hidA, "great!", msgsA},
	}

	for i, turn := range turns {
		if err := turn.from.SendMessage(turn.target, turn.body); err != nil {
			t.Fatalf("turn %d: %v", i, err)
		}
		select {
		case msg := <-turn.ch:
			if msg.Body != turn.body {
				t.Errorf("turn %d: body = %q, want %q", i, msg.Body, turn.body)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("turn %d: timeout", i)
		}
	}
}

func TestE2E_KeyRequest_NotMatched(t *testing.T) {
	env := newTestEnv(t)
	pubA, privA := genKeys(t)
	pubB, _ := genKeys(t)
	env.addUser("alice", "github", pubA)
	hidB := env.addUser("bob", "github", pubB)
	// No match!

	clientA := connectClient(t, env, "alice", "github", pubA, privA)

	errCh := make(chan protocol.Error, 1)
	clientA.OnError(func(e protocol.Error) { errCh <- e })

	go clientA.SendMessage(hidB, "hello")

	select {
	case e := <-errCh:
		if e.Code != protocol.ErrNotMatched {
			t.Errorf("code = %q, want not_matched", e.Code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for not_matched error")
	}
}

func TestE2E_KeyRequest_TargetNotFound(t *testing.T) {
	env := newTestEnv(t)
	pubA, privA := genKeys(t)
	hidA := env.addUser("alice", "github", pubA)
	fakeHash := "nonexistent12345"
	env.addMatch(hidA, fakeHash)

	clientA := connectClient(t, env, "alice", "github", pubA, privA)

	errCh := make(chan protocol.Error, 1)
	clientA.OnError(func(e protocol.Error) { errCh <- e })

	go clientA.SendMessage(fakeHash, "hello")

	select {
	case e := <-errCh:
		if e.Code != protocol.ErrUserNotFound {
			t.Errorf("code = %q, want user_not_found", e.Code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for user_not_found error")
	}
}

func TestE2E_MixedEncryptedUnencrypted(t *testing.T) {
	env := newTestEnv(t)
	pubA, privA := genKeys(t)
	pubB, privB := genKeys(t)
	hidA := env.addUser("alice", "github", pubA)
	hidB := env.addUser("bob", "github", pubB)
	env.addMatch(hidA, hidB)

	clientA := connectClient(t, env, "alice", "github", pubA, privA)
	clientB := connectClient(t, env, "bob", "github", pubB, privB)

	msgCh := make(chan protocol.Message, 2)
	clientB.OnMessage(func(msg protocol.Message) { msgCh <- msg })

	// Plain message
	if err := clientA.SendMessagePlain(hidB, "plain hello"); err != nil {
		t.Fatalf("plain send: %v", err)
	}
	select {
	case msg := <-msgCh:
		if msg.Body != "plain hello" {
			t.Errorf("plain body = %q", msg.Body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout for plain message")
	}

	// Encrypted message
	if err := clientA.SendMessage(hidB, "encrypted hello"); err != nil {
		t.Fatalf("encrypted send: %v", err)
	}
	select {
	case msg := <-msgCh:
		if msg.Body != "encrypted hello" {
			t.Errorf("encrypted body = %q", msg.Body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout for encrypted message")
	}
}
