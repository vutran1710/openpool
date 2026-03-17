package relay_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/protocol"
	server "github.com/vutran1710/dating-dev/internal/relay"
)

// --- Test helpers ---

const testPoolURL = "owner/pool"
const testSalt = "test-salt"

func genKeys(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return pub, priv
}

func idHash(poolURL, provider, userID string) string {
	h := sha256.Sum256([]byte(poolURL + ":" + provider + ":" + userID))
	return hex.EncodeToString(h[:])
}

func binHash(salt, idH string) string {
	h := sha256.Sum256([]byte(salt + ":" + idH))
	return hex.EncodeToString(h[:])[:16]
}

func matchHash(salt, binH string) string {
	h := sha256.Sum256([]byte(salt + ":" + binH))
	return hex.EncodeToString(h[:])[:16]
}

type testEnv struct {
	srv   *server.Server
	ts    *httptest.Server
	url   string // http URL
	wsURL string // ws URL
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	srv := server.NewServer(server.ServerConfig{
		PoolURL: testPoolURL,
		Salt:    testSalt,
	})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { ts.Close() })
	return &testEnv{
		srv:   srv,
		ts:    ts,
		url:   ts.URL,
		wsURL: "ws" + strings.TrimPrefix(ts.URL, "http"),
	}
}

func (e *testEnv) addUser(userID, provider string, pub ed25519.PublicKey) (bin, match string) {
	ih := idHash(testPoolURL, provider, userID)
	bin = binHash(testSalt, ih)
	match = matchHash(testSalt, bin)
	e.srv.Store().UpsertUser(server.UserEntry{
		PubKey:    pub,
		UserID:    userID,
		Provider:  provider,
		BinHash:   bin,
		MatchHash: match,
	})
	return bin, match
}

func (e *testEnv) addMatch(hashA, hashB string) {
	e.srv.Store().AddMatch(hashA, hashB)
}

// connectWS dials a WebSocket with TOTP auth.
func connectWS(t *testing.T, env *testEnv, binH, matchH string, priv ed25519.PrivateKey) *websocket.Conn {
	t.Helper()
	sig := crypto.TOTPSign(binH, matchH, priv)
	url := fmt.Sprintf("%s/ws?bin=%s&sig=%s", env.wsURL, binH, sig)

	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, resp, err := dialer.Dial(url, nil)
	if err != nil {
		if resp != nil {
			t.Fatalf("connect failed (%d): %v", resp.StatusCode, err)
		}
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// sendMsg sends a Message frame.
func sendMsg(t *testing.T, conn *websocket.Conn, msg protocol.Message) {
	t.Helper()
	data, err := protocol.Encode(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// readFrame reads and decodes a frame with timeout.
func readFrame(t *testing.T, conn *websocket.Conn, timeout time.Duration) any {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(timeout))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	frame, err := protocol.DecodeFrame(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return frame
}

// ==================== TOTP Connect Tests ====================

func TestTOTP_ConnectSuccess(t *testing.T) {
	env := newTestEnv(t)
	pub, priv := genKeys(t)
	bin, match := env.addUser("alice", "github", pub)

	conn := connectWS(t, env, bin, match, priv)
	if conn == nil {
		t.Fatal("should have connected")
	}
}

func TestTOTP_ConnectBadSig(t *testing.T) {
	env := newTestEnv(t)
	pub, _ := genKeys(t)
	_, wrongPriv := genKeys(t)
	bin, match := env.addUser("alice", "github", pub)

	sig := crypto.TOTPSign(bin, match, wrongPriv)
	url := fmt.Sprintf("%s/ws?bin=%s&sig=%s", env.wsURL, bin, sig)

	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	_, resp, err := dialer.Dial(url, nil)
	if err == nil {
		t.Fatal("should have failed")
	}
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestTOTP_ConnectUnknownUser(t *testing.T) {
	env := newTestEnv(t)
	_, priv := genKeys(t)

	sig := crypto.TOTPSign("unknown_bin_hash", "unknown_match_ha", priv)
	url := fmt.Sprintf("%s/ws?bin=%s&sig=%s", env.wsURL, "unknown_bin_hash", sig)

	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	_, resp, err := dialer.Dial(url, nil)
	if err == nil {
		t.Fatal("should have failed")
	}
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestTOTP_ConnectExpiredSig(t *testing.T) {
	env := newTestEnv(t)
	pub, priv := genKeys(t)
	bin, match := env.addUser("alice", "github", pub)

	// Sign 2 windows ago — should be rejected
	tw := time.Now().Unix()/300 - 2
	sig := crypto.TOTPSignAt(bin, match, priv, tw)
	url := fmt.Sprintf("%s/ws?bin=%s&sig=%s", env.wsURL, bin, sig)

	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	_, resp, err := dialer.Dial(url, nil)
	if err == nil {
		t.Fatal("expired sig should be rejected")
	}
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestTOTP_ConnectMissingSig(t *testing.T) {
	env := newTestEnv(t)
	pub, _ := genKeys(t)
	bin, _ := env.addUser("alice", "github", pub)

	url := fmt.Sprintf("%s/ws?bin=%s", env.wsURL, bin)
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	_, resp, err := dialer.Dial(url, nil)
	if err == nil {
		t.Fatal("should fail without sig")
	}
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestTOTP_ConnectMissingBin(t *testing.T) {
	env := newTestEnv(t)

	url := fmt.Sprintf("%s/ws?sig=deadbeef", env.wsURL)
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	_, resp, err := dialer.Dial(url, nil)
	if err == nil {
		t.Fatal("should fail without bin")
	}
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestTOTP_ConnectMalformedSig(t *testing.T) {
	env := newTestEnv(t)
	pub, _ := genKeys(t)
	bin, _ := env.addUser("alice", "github", pub)

	url := fmt.Sprintf("%s/ws?bin=%s&sig=not-valid-hex-zzz", env.wsURL, bin)
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	_, resp, err := dialer.Dial(url, nil)
	if err == nil {
		t.Fatal("should fail with malformed sig")
	}
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// ==================== Message Routing Tests ====================

func TestTOTP_MessageRouting_BothOnline(t *testing.T) {
	env := newTestEnv(t)
	pubA, privA := genKeys(t)
	pubB, privB := genKeys(t)
	binA, matchA := env.addUser("alice", "github", pubA)
	binB, matchB := env.addUser("bob", "github", pubB)
	env.addMatch(binA, binB)

	connA := connectWS(t, env, binA, matchA, privA)
	connB := connectWS(t, env, binB, matchB, privB)

	sendMsg(t, connA, protocol.Message{
		Type:       protocol.TypeMsg,
		SourceHash: binA,
		TargetHash: binB,
		Body:       "hey bob!",
		Ts:         time.Now().Unix(),
	})

	frame := readFrame(t, connB, 5*time.Second)
	msg, ok := frame.(*protocol.Message)
	if !ok {
		t.Fatalf("expected Message, got %T", frame)
	}
	if msg.Body != "hey bob!" {
		t.Errorf("body = %q, want 'hey bob!'", msg.Body)
	}
	if msg.MsgID == "" {
		t.Error("msg_id should be set by relay")
	}
}

func TestTOTP_MessageRouting_Bidirectional(t *testing.T) {
	env := newTestEnv(t)
	pubA, privA := genKeys(t)
	pubB, privB := genKeys(t)
	binA, matchA := env.addUser("alice", "github", pubA)
	binB, matchB := env.addUser("bob", "github", pubB)
	env.addMatch(binA, binB)

	connA := connectWS(t, env, binA, matchA, privA)
	connB := connectWS(t, env, binB, matchB, privB)

	// A → B
	sendMsg(t, connA, protocol.Message{
		Type: protocol.TypeMsg, SourceHash: binA, TargetHash: binB,
		Body: "hello bob", Ts: time.Now().Unix(),
	})
	frame := readFrame(t, connB, 5*time.Second)
	if msg := frame.(*protocol.Message); msg.Body != "hello bob" {
		t.Errorf("A→B body = %q", msg.Body)
	}

	// B → A
	sendMsg(t, connB, protocol.Message{
		Type: protocol.TypeMsg, SourceHash: binB, TargetHash: binA,
		Body: "hello alice", Ts: time.Now().Unix(),
	})
	frame = readFrame(t, connA, 5*time.Second)
	if msg := frame.(*protocol.Message); msg.Body != "hello alice" {
		t.Errorf("B→A body = %q", msg.Body)
	}
}

func TestTOTP_MessageRouting_NotMatched(t *testing.T) {
	env := newTestEnv(t)
	pubA, privA := genKeys(t)
	pubB, _ := genKeys(t)
	binA, matchA := env.addUser("alice", "github", pubA)
	binB, _ := env.addUser("bob", "github", pubB)
	// No match added!

	connA := connectWS(t, env, binA, matchA, privA)

	sendMsg(t, connA, protocol.Message{
		Type: protocol.TypeMsg, SourceHash: binA, TargetHash: binB,
		Body: "hey!", Ts: time.Now().Unix(),
	})

	frame := readFrame(t, connA, 5*time.Second)
	errFrame, ok := frame.(*protocol.Error)
	if !ok {
		t.Fatalf("expected Error, got %T", frame)
	}
	if errFrame.Code != protocol.ErrNotMatched {
		t.Errorf("code = %q, want not_matched", errFrame.Code)
	}
}

func TestTOTP_MessageRouting_TargetOffline(t *testing.T) {
	env := newTestEnv(t)
	pubA, privA := genKeys(t)
	pubB, _ := genKeys(t)
	binA, matchA := env.addUser("alice", "github", pubA)
	binB, _ := env.addUser("bob", "github", pubB)
	env.addMatch(binA, binB)

	connA := connectWS(t, env, binA, matchA, privA)
	// Bob NOT connected

	// Should not crash — message silently dropped (TODO: queue)
	sendMsg(t, connA, protocol.Message{
		Type: protocol.TypeMsg, SourceHash: binA, TargetHash: binB,
		Body: "are you there?", Ts: time.Now().Unix(),
	})

	time.Sleep(100 * time.Millisecond)
	// No crash = pass
}

func TestTOTP_MessageRouting_WrongSource(t *testing.T) {
	env := newTestEnv(t)
	pubA, privA := genKeys(t)
	pubB, _ := genKeys(t)
	binA, matchA := env.addUser("alice", "github", pubA)
	binB, _ := env.addUser("bob", "github", pubB)
	env.addMatch(binA, binB)

	connA := connectWS(t, env, binA, matchA, privA)

	// Try to send with wrong source_hash
	sendMsg(t, connA, protocol.Message{
		Type: protocol.TypeMsg, SourceHash: "wrong_source_has", TargetHash: binB,
		Body: "spoofed!", Ts: time.Now().Unix(),
	})

	frame := readFrame(t, connA, 5*time.Second)
	errFrame, ok := frame.(*protocol.Error)
	if !ok {
		t.Fatalf("expected Error, got %T", frame)
	}
	if errFrame.Code != protocol.ErrAuthFailed {
		t.Errorf("code = %q, want auth_failed", errFrame.Code)
	}
}

// ==================== Concurrent + Lifecycle Tests ====================

func TestTOTP_MultipleUsers_Concurrent(t *testing.T) {
	env := newTestEnv(t)
	const n = 10

	type user struct {
		pub   ed25519.PublicKey
		priv  ed25519.PrivateKey
		bin   string
		match string
	}

	users := make([]user, n)
	for i := 0; i < n; i++ {
		pub, priv := genKeys(t)
		uid := strings.Repeat(string(rune('a'+i)), 3)
		bin, match := env.addUser(uid, "github", pub)
		users[i] = user{pub: pub, priv: priv, bin: bin, match: match}
	}

	var wg sync.WaitGroup
	conns := make([]*websocket.Conn, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			u := users[idx]
			sig := crypto.TOTPSign(u.bin, u.match, u.priv)
			url := fmt.Sprintf("%s/ws?bin=%s&sig=%s", env.wsURL, u.bin, sig)
			dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
			conn, _, err := dialer.Dial(url, nil)
			if err != nil {
				t.Errorf("connect user %d: %v", idx, err)
				return
			}
			conns[idx] = conn
		}(i)
	}
	wg.Wait()

	for _, c := range conns {
		if c != nil {
			c.Close()
		}
	}
	for i, c := range conns {
		if c == nil {
			t.Errorf("user %d failed to connect", i)
		}
	}
}

func TestTOTP_DisconnectReconnect(t *testing.T) {
	env := newTestEnv(t)
	pub, priv := genKeys(t)
	bin, match := env.addUser("alice", "github", pub)

	// First connection
	conn1 := connectWS(t, env, bin, match, priv)
	conn1.Close()
	time.Sleep(100 * time.Millisecond)

	// Reconnect with fresh TOTP
	conn2 := connectWS(t, env, bin, match, priv)
	if conn2 == nil {
		t.Fatal("reconnect should succeed")
	}
}

func TestTOTP_SessionReplacement(t *testing.T) {
	env := newTestEnv(t)
	pubA, privA := genKeys(t)
	pubB, privB := genKeys(t)
	binA, matchA := env.addUser("alice", "github", pubA)
	binB, matchB := env.addUser("bob", "github", pubB)
	env.addMatch(binA, binB)

	// Alice connects first session
	_ = connectWS(t, env, binA, matchA, privA)

	// Alice connects second session (replaces first)
	conn2 := connectWS(t, env, binA, matchA, privA)

	// Bob connects and sends to Alice
	connB := connectWS(t, env, binB, matchB, privB)
	sendMsg(t, connB, protocol.Message{
		Type: protocol.TypeMsg, SourceHash: binB, TargetHash: binA,
		Body: "which alice?", Ts: time.Now().Unix(),
	})

	// Message should go to conn2 (newest session)
	frame := readFrame(t, conn2, 5*time.Second)
	msg, ok := frame.(*protocol.Message)
	if !ok {
		t.Fatalf("expected Message, got %T", frame)
	}
	if msg.Body != "which alice?" {
		t.Errorf("body = %q", msg.Body)
	}
}

// ==================== Key Exchange Tests ====================

func TestTOTP_KeyRequest_Success(t *testing.T) {
	env := newTestEnv(t)
	pubA, privA := genKeys(t)
	pubB, _ := genKeys(t)
	binA, matchA := env.addUser("alice", "github", pubA)
	binB, _ := env.addUser("bob", "github", pubB)
	env.addMatch(binA, binB)

	connA := connectWS(t, env, binA, matchA, privA)

	// Request Bob's key
	data, _ := protocol.Encode(protocol.KeyRequest{
		Type: protocol.TypeKeyRequest, TargetHash: binB,
	})
	connA.WriteMessage(websocket.BinaryMessage, data)

	frame := readFrame(t, connA, 5*time.Second)
	resp, ok := frame.(*protocol.KeyResponse)
	if !ok {
		t.Fatalf("expected KeyResponse, got %T", frame)
	}
	if resp.TargetHash != binB {
		t.Errorf("target = %q, want %q", resp.TargetHash, binB)
	}
	keyBytes, _ := hex.DecodeString(resp.PubKey)
	if !pubB.Equal(ed25519.PublicKey(keyBytes)) {
		t.Error("returned key doesn't match Bob's pubkey")
	}
}

func TestTOTP_KeyRequest_NotMatched(t *testing.T) {
	env := newTestEnv(t)
	pubA, privA := genKeys(t)
	pubB, _ := genKeys(t)
	binA, matchA := env.addUser("alice", "github", pubA)
	binB, _ := env.addUser("bob", "github", pubB)
	// No match!

	connA := connectWS(t, env, binA, matchA, privA)

	data, _ := protocol.Encode(protocol.KeyRequest{
		Type: protocol.TypeKeyRequest, TargetHash: binB,
	})
	connA.WriteMessage(websocket.BinaryMessage, data)

	frame := readFrame(t, connA, 5*time.Second)
	errFrame, ok := frame.(*protocol.Error)
	if !ok {
		t.Fatalf("expected Error, got %T", frame)
	}
	if errFrame.Code != protocol.ErrNotMatched {
		t.Errorf("code = %q, want not_matched", errFrame.Code)
	}
}

func TestTOTP_KeyRequest_TargetNotFound(t *testing.T) {
	env := newTestEnv(t)
	pubA, privA := genKeys(t)
	binA, matchA := env.addUser("alice", "github", pubA)
	fakeHash := "nonexistent12345"
	env.addMatch(binA, fakeHash)

	connA := connectWS(t, env, binA, matchA, privA)

	data, _ := protocol.Encode(protocol.KeyRequest{
		Type: protocol.TypeKeyRequest, TargetHash: fakeHash,
	})
	connA.WriteMessage(websocket.BinaryMessage, data)

	frame := readFrame(t, connA, 5*time.Second)
	errFrame, ok := frame.(*protocol.Error)
	if !ok {
		t.Fatalf("expected Error, got %T", frame)
	}
	if errFrame.Code != protocol.ErrUserNotFound {
		t.Errorf("code = %q, want user_not_found", errFrame.Code)
	}
}

// ==================== Health Endpoint ====================

func TestTOTP_HealthEndpoint(t *testing.T) {
	env := newTestEnv(t)
	resp, err := http.Get(env.url + "/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

// ==================== Encrypted Message Tests ====================

func TestTOTP_EncryptedMessage_Flagged(t *testing.T) {
	env := newTestEnv(t)
	pubA, privA := genKeys(t)
	pubB, privB := genKeys(t)
	binA, matchA := env.addUser("alice", "github", pubA)
	binB, matchB := env.addUser("bob", "github", pubB)
	env.addMatch(binA, binB)

	connA := connectWS(t, env, binA, matchA, privA)
	connB := connectWS(t, env, binB, matchB, privB)

	sendMsg(t, connA, protocol.Message{
		Type: protocol.TypeMsg, SourceHash: binA, TargetHash: binB,
		Body: "encrypted_payload_here", Ts: time.Now().Unix(), Encrypted: true,
	})

	frame := readFrame(t, connB, 5*time.Second)
	msg := frame.(*protocol.Message)
	if !msg.Encrypted {
		t.Error("encrypted flag should be preserved")
	}
	if msg.Body != "encrypted_payload_here" {
		t.Errorf("body = %q", msg.Body)
	}
}

// ==================== Multi-turn Conversation ====================

func TestTOTP_Conversation_MultiTurn(t *testing.T) {
	env := newTestEnv(t)
	pubA, privA := genKeys(t)
	pubB, privB := genKeys(t)
	binA, matchA := env.addUser("alice", "github", pubA)
	binB, matchB := env.addUser("bob", "github", pubB)
	env.addMatch(binA, binB)

	connA := connectWS(t, env, binA, matchA, privA)
	connB := connectWS(t, env, binB, matchB, privB)

	turns := []struct {
		from, to       *websocket.Conn
		src, tgt, body string
		readFrom       *websocket.Conn
	}{
		{connA, connB, binA, binB, "hey!", connB},
		{connB, connA, binB, binA, "hi there!", connA},
		{connA, connB, binA, binB, "how are you?", connB},
		{connB, connA, binB, binA, "great, you?", connA},
	}

	for i, turn := range turns {
		sendMsg(t, turn.from, protocol.Message{
			Type: protocol.TypeMsg, SourceHash: turn.src, TargetHash: turn.tgt,
			Body: turn.body, Ts: time.Now().Unix(),
		})
		frame := readFrame(t, turn.readFrom, 5*time.Second)
		msg := frame.(*protocol.Message)
		if msg.Body != turn.body {
			t.Errorf("turn %d: body = %q, want %q", i, msg.Body, turn.body)
		}
	}
}
