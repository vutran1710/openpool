package relay_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vutran1710/dating-dev/internal/crypto"
	relay "github.com/vutran1710/dating-dev/internal/relay"
)

// --- Test helpers ---

type testEnv struct {
	srv     *relay.Server
	httpSrv *httptest.Server
	mockGH  *httptest.Server
	salt    string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	salt := "test-salt-123"

	mockGH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))

	srv := relay.NewServer(relay.ServerConfig{
		PoolURL:    "test/pool",
		Salt:       salt,
		RawBaseURL: mockGH.URL,
	})

	httpSrv := httptest.NewServer(srv.Handler())

	env := &testEnv{srv: srv, httpSrv: httpSrv, mockGH: mockGH, salt: salt}
	t.Cleanup(env.close)
	return env
}

func (e *testEnv) close() {
	e.httpSrv.Close()
	e.mockGH.Close()
}

func (e *testEnv) wsURL() string {
	return "ws" + strings.TrimPrefix(e.httpSrv.URL, "http")
}

func (e *testEnv) registerUser(t *testing.T) (idHash, binHash, matchHash string, pub ed25519.PublicKey, priv ed25519.PrivateKey) {
	t.Helper()
	pub, priv, _ = ed25519.GenerateKey(rand.Reader)
	idHash = fmt.Sprintf("%x", sha256.Sum256([]byte("test-id-"+hex.EncodeToString(pub[:8]))))
	binHash = e.srv.BinHash(idHash)
	matchHash = e.srv.MatchHash(binHash)
	e.srv.GitHubCache().SetPubkey(binHash, pub)
	return
}

func (e *testEnv) connectWS(t *testing.T, idHash, matchHash string, priv ed25519.PrivateKey) *websocket.Conn {
	t.Helper()
	sig := crypto.TOTPSign(priv)
	url := e.wsURL() + "/ws?id=" + idHash + "&match=" + matchHash + "&sig=" + sig
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, resp, err := dialer.Dial(url, nil)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("dial failed: %v (status=%d)", err, status)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func (e *testEnv) dialExpectFail(t *testing.T, url string) int {
	t.Helper()
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	_, resp, err := dialer.Dial(url, nil)
	if err == nil {
		t.Fatal("expected dial to fail, but it succeeded")
	}
	if resp == nil {
		t.Fatalf("no HTTP response; error: %v", err)
	}
	return resp.StatusCode
}

// readText reads a text message with timeout.
func readText(t *testing.T, conn *websocket.Conn, timeout time.Duration) string {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(timeout))
	msgType, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if msgType != websocket.TextMessage {
		t.Fatalf("expected text message, got type %d", msgType)
	}
	return string(data)
}

// readBinary reads a binary message with timeout.
func readBinary(t *testing.T, conn *websocket.Conn, timeout time.Duration) (senderMatchHash string, payload []byte) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(timeout))
	msgType, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if msgType != websocket.BinaryMessage {
		t.Fatalf("expected binary message, got type %d (data=%q)", msgType, string(data))
	}
	if len(data) < 8 {
		t.Fatalf("binary frame too short: %d bytes", len(data))
	}
	senderMatchHash = hex.EncodeToString(data[:8])
	payload = data[8:]
	return
}

// sendBinary sends a binary frame: [8B target_match_hash][payload].
func sendBinary(t *testing.T, conn *websocket.Conn, targetMatchHash string, payload []byte) {
	t.Helper()
	targetBytes, err := hex.DecodeString(targetMatchHash)
	if err != nil {
		t.Fatalf("decode target match hash: %v", err)
	}
	frame := append(targetBytes, payload...)
	if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// ==================== Auth Tests ====================

func TestAuth_ValidChain(t *testing.T) {
	env := newTestEnv(t)
	idHash, _, matchHash, _, priv := env.registerUser(t)

	conn := env.connectWS(t, idHash, matchHash, priv)
	if conn == nil {
		t.Fatal("should have connected")
	}
}

func TestAuth_WrongSig(t *testing.T) {
	env := newTestEnv(t)
	idHash, _, matchHash, _, _ := env.registerUser(t)
	_, wrongPriv, _ := ed25519.GenerateKey(rand.Reader)

	sig := crypto.TOTPSign(wrongPriv)
	url := env.wsURL() + "/ws?id=" + idHash + "&match=" + matchHash + "&sig=" + sig
	status := env.dialExpectFail(t, url)
	if status != 401 {
		t.Errorf("status = %d, want 401", status)
	}
}

func TestAuth_MismatchedChain(t *testing.T) {
	env := newTestEnv(t)
	idHash, _, _, _, priv := env.registerUser(t)

	// Use a wrong match_hash that doesn't derive from this id_hash
	wrongMatch := "abcdef0123456789"
	sig := crypto.TOTPSign(priv)
	url := env.wsURL() + "/ws?id=" + idHash + "&match=" + wrongMatch + "&sig=" + sig
	status := env.dialExpectFail(t, url)
	if status != 401 {
		t.Errorf("status = %d, want 401", status)
	}
}

func TestAuth_MissingParams(t *testing.T) {
	env := newTestEnv(t)

	cases := []struct {
		name, query string
	}{
		{"no params", ""},
		{"only id", "?id=abc"},
		{"only match", "?match=abc"},
		{"only sig", "?sig=abc"},
		{"missing sig", "?id=abc&match=def"},
		{"missing id", "?match=abc&sig=def"},
		{"missing match", "?id=abc&sig=def"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url := env.wsURL() + "/ws" + tc.query
			status := env.dialExpectFail(t, url)
			if status != 401 {
				t.Errorf("status = %d, want 401", status)
			}
		})
	}
}

func TestAuth_ExpiredSig(t *testing.T) {
	env := newTestEnv(t)
	idHash, _, matchHash, _, priv := env.registerUser(t)

	// Sign 2 windows ago — outside ±1 tolerance
	tw := time.Now().Unix()/300 - 2
	sig := crypto.TOTPSignAt(priv, tw)
	url := env.wsURL() + "/ws?id=" + idHash + "&match=" + matchHash + "&sig=" + sig
	status := env.dialExpectFail(t, url)
	if status != 401 {
		t.Errorf("status = %d, want 401", status)
	}
}

func TestAuth_UnknownUser(t *testing.T) {
	env := newTestEnv(t)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)

	// Use a valid-looking id_hash but no pubkey injected
	idHash := fmt.Sprintf("%x", sha256.Sum256([]byte("unknown-user")))
	binHash := env.srv.BinHash(idHash)
	matchHash := env.srv.MatchHash(binHash)
	// Do NOT inject pubkey — this user is unknown

	sig := crypto.TOTPSign(priv)
	url := env.wsURL() + "/ws?id=" + idHash + "&match=" + matchHash + "&sig=" + sig
	status := env.dialExpectFail(t, url)
	if status != 401 {
		t.Errorf("status = %d, want 401", status)
	}
}

// ==================== Routing Tests ====================

func TestRouting_BothOnline(t *testing.T) {
	env := newTestEnv(t)
	idA, _, matchA, _, privA := env.registerUser(t)
	idB, _, matchB, _, privB := env.registerUser(t)

	// Set up match
	pairHash := relay.PairHash(matchA, matchB)
	env.srv.GitHubCache().SetMatch(pairHash, true)

	connA := env.connectWS(t, idA, matchA, privA)
	connB := env.connectWS(t, idB, matchB, privB)

	// Allow sessions to register
	time.Sleep(50 * time.Millisecond)

	// Alice sends to Bob
	sendBinary(t, connA, matchB, []byte("hello bob"))

	// Bob should receive with Alice's match_hash prepended
	senderHash, payload := readBinary(t, connB, 2*time.Second)
	if senderHash != matchA {
		t.Errorf("sender match_hash = %q, want %q", senderHash, matchA)
	}
	if string(payload) != "hello bob" {
		t.Errorf("payload = %q, want %q", string(payload), "hello bob")
	}
}

func TestRouting_NotMatched(t *testing.T) {
	env := newTestEnv(t)
	idA, _, matchA, _, privA := env.registerUser(t)
	_, _, matchB, _, _ := env.registerUser(t)
	// No match set!

	connA := env.connectWS(t, idA, matchA, privA)
	time.Sleep(50 * time.Millisecond)

	sendBinary(t, connA, matchB, []byte("hey!"))

	msg := readText(t, connA, 2*time.Second)
	if msg != "not matched" {
		t.Errorf("response = %q, want 'not matched'", msg)
	}
}

func TestRouting_SenderMatchHashPrepended(t *testing.T) {
	env := newTestEnv(t)
	idA, _, matchA, _, privA := env.registerUser(t)
	idB, _, matchB, _, privB := env.registerUser(t)

	pairHash := relay.PairHash(matchA, matchB)
	env.srv.GitHubCache().SetMatch(pairHash, true)

	connA := env.connectWS(t, idA, matchA, privA)
	connB := env.connectWS(t, idB, matchB, privB)
	time.Sleep(50 * time.Millisecond)

	payload := []byte("test payload 12345")
	sendBinary(t, connA, matchB, payload)

	// Read raw binary frame from Bob's connection
	connB.SetReadDeadline(time.Now().Add(2 * time.Second))
	msgType, data, err := connB.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if msgType != websocket.BinaryMessage {
		t.Fatalf("expected binary, got type %d", msgType)
	}

	// First 8 bytes should be Alice's match_hash (decoded from hex)
	if len(data) < 8 {
		t.Fatalf("frame too short: %d bytes", len(data))
	}
	gotSenderHex := hex.EncodeToString(data[:8])
	if gotSenderHex != matchA {
		t.Errorf("prepended sender hash = %q, want %q", gotSenderHex, matchA)
	}
	if string(data[8:]) != "test payload 12345" {
		t.Errorf("payload = %q, want %q", string(data[8:]), "test payload 12345")
	}
}

func TestRouting_Bidirectional(t *testing.T) {
	env := newTestEnv(t)
	idA, _, matchA, _, privA := env.registerUser(t)
	idB, _, matchB, _, privB := env.registerUser(t)

	pairHash := relay.PairHash(matchA, matchB)
	env.srv.GitHubCache().SetMatch(pairHash, true)

	connA := env.connectWS(t, idA, matchA, privA)
	connB := env.connectWS(t, idB, matchB, privB)
	time.Sleep(50 * time.Millisecond)

	// A → B
	sendBinary(t, connA, matchB, []byte("hello bob"))
	senderHash, payload := readBinary(t, connB, 2*time.Second)
	if senderHash != matchA || string(payload) != "hello bob" {
		t.Errorf("A→B: sender=%q payload=%q", senderHash, string(payload))
	}

	// B → A
	sendBinary(t, connB, matchA, []byte("hello alice"))
	senderHash, payload = readBinary(t, connA, 2*time.Second)
	if senderHash != matchB || string(payload) != "hello alice" {
		t.Errorf("B→A: sender=%q payload=%q", senderHash, string(payload))
	}
}

// ==================== Queue Tests ====================

func TestQueue_OfflineQueued(t *testing.T) {
	env := newTestEnv(t)
	idA, _, matchA, _, privA := env.registerUser(t)
	idB, _, matchB, _, privB := env.registerUser(t)

	pairHash := relay.PairHash(matchA, matchB)
	env.srv.GitHubCache().SetMatch(pairHash, true)

	// Only Alice connects; Bob is offline
	connA := env.connectWS(t, idA, matchA, privA)
	time.Sleep(50 * time.Millisecond)

	// Alice sends to offline Bob
	sendBinary(t, connA, matchB, []byte("are you there?"))

	// Alice should get "queued" text frame
	msg := readText(t, connA, 2*time.Second)
	if msg != "queued" {
		t.Errorf("response = %q, want 'queued'", msg)
	}

	// Bob connects — should receive the queued message
	connB := env.connectWS(t, idB, matchB, privB)
	time.Sleep(50 * time.Millisecond)

	senderHash, payload := readBinary(t, connB, 2*time.Second)
	if senderHash != matchA {
		t.Errorf("sender = %q, want %q", senderHash, matchA)
	}
	if string(payload) != "are you there?" {
		t.Errorf("payload = %q, want %q", string(payload), "are you there?")
	}
}

func TestQueue_CapEnforced(t *testing.T) {
	env := newTestEnv(t)
	idA, _, matchA, _, privA := env.registerUser(t)
	_, _, matchB, _, _ := env.registerUser(t)

	pairHash := relay.PairHash(matchA, matchB)
	env.srv.GitHubCache().SetMatch(pairHash, true)

	connA := env.connectWS(t, idA, matchA, privA)
	time.Sleep(50 * time.Millisecond)

	// Send 21 messages to offline Bob
	for i := 0; i < 21; i++ {
		payload := fmt.Sprintf("msg-%d", i)
		sendBinary(t, connA, matchB, []byte(payload))

		msg := readText(t, connA, 2*time.Second)
		if i < 20 {
			if msg != "queued" {
				t.Fatalf("msg %d: expected 'queued', got %q", i, msg)
			}
		} else {
			if msg != "queue full" {
				t.Fatalf("msg %d: expected 'queue full', got %q", i, msg)
			}
		}
	}
}

// ==================== Session Tests ====================

func TestSession_ReconnectReplacesOld(t *testing.T) {
	env := newTestEnv(t)
	idA, _, matchA, _, privA := env.registerUser(t)
	idB, _, matchB, _, privB := env.registerUser(t)

	pairHash := relay.PairHash(matchA, matchB)
	env.srv.GitHubCache().SetMatch(pairHash, true)

	// Alice connects first session
	conn1 := env.connectWS(t, idA, matchA, privA)
	time.Sleep(50 * time.Millisecond)

	// Alice connects second session (should replace first)
	conn2 := env.connectWS(t, idA, matchA, privA)
	time.Sleep(50 * time.Millisecond)

	// Old connection should be closed — reading from it should fail
	conn1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err := conn1.ReadMessage()
	if err == nil {
		t.Error("old connection should be closed after replacement")
	}

	// Bob sends to Alice — should arrive on conn2
	connB := env.connectWS(t, idB, matchB, privB)
	time.Sleep(50 * time.Millisecond)

	sendBinary(t, connB, matchA, []byte("which alice?"))

	senderHash, payload := readBinary(t, conn2, 2*time.Second)
	if senderHash != matchB || string(payload) != "which alice?" {
		t.Errorf("got sender=%q payload=%q", senderHash, string(payload))
	}
}

func TestSession_MultipleConcurrentUsers(t *testing.T) {
	env := newTestEnv(t)
	const n = 5

	type user struct {
		idHash    string
		matchHash string
		priv      ed25519.PrivateKey
	}

	users := make([]user, n)
	for i := range users {
		idH, _, matchH, _, priv := env.registerUser(t)
		users[i] = user{idHash: idH, matchHash: matchH, priv: priv}
	}

	var wg sync.WaitGroup
	conns := make([]*websocket.Conn, n)
	errors := make([]error, n)

	for i := range users {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			u := users[idx]
			sig := crypto.TOTPSign(u.priv)
			url := env.wsURL() + "/ws?id=" + u.idHash + "&match=" + u.matchHash + "&sig=" + sig
			dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
			conn, _, err := dialer.Dial(url, nil)
			if err != nil {
				errors[idx] = err
				return
			}
			conns[idx] = conn
		}(i)
	}
	wg.Wait()

	for i, c := range conns {
		if c == nil {
			t.Errorf("user %d failed to connect: %v", i, errors[i])
		} else {
			c.Close()
		}
	}
}

// ==================== Health Test ====================

func TestHealth_Endpoint(t *testing.T) {
	env := newTestEnv(t)

	resp, err := http.Get(env.httpSrv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode json: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", body["status"])
	}

	// Check that online and queued fields exist
	if _, ok := body["online"]; !ok {
		t.Error("missing 'online' field")
	}
	if _, ok := body["queued"]; !ok {
		t.Error("missing 'queued' field")
	}
}

func TestHealth_ReflectsOnlineCount(t *testing.T) {
	env := newTestEnv(t)

	// No users connected
	resp, _ := http.Get(env.httpSrv.URL + "/health")
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	if body["online"].(float64) != 0 {
		t.Errorf("expected 0 online, got %v", body["online"])
	}

	// Connect a user
	idH, _, matchH, _, priv := env.registerUser(t)
	_ = env.connectWS(t, idH, matchH, priv)
	time.Sleep(50 * time.Millisecond)

	resp, _ = http.Get(env.httpSrv.URL + "/health")
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	if body["online"].(float64) != 1 {
		t.Errorf("expected 1 online, got %v", body["online"])
	}
}

// ==================== Encrypted Roundtrip Test ====================

func TestEncrypted_Roundtrip(t *testing.T) {
	env := newTestEnv(t)
	idA, _, matchA, pubA, privA := env.registerUser(t)
	idB, _, matchB, pubB, privB := env.registerUser(t)

	pairHash := relay.PairHash(matchA, matchB)
	env.srv.GitHubCache().SetMatch(pairHash, true)

	connA := env.connectWS(t, idA, matchA, privA)
	connB := env.connectWS(t, idB, matchB, privB)
	time.Sleep(50 * time.Millisecond)

	// Derive shared secret (both sides produce the same key)
	sharedAB, err := crypto.SharedSecret(privA, pubB)
	if err != nil {
		t.Fatalf("SharedSecret(A→B): %v", err)
	}
	sharedBA, err := crypto.SharedSecret(privB, pubA)
	if err != nil {
		t.Fatalf("SharedSecret(B→A): %v", err)
	}
	if hex.EncodeToString(sharedAB) != hex.EncodeToString(sharedBA) {
		t.Fatal("shared secrets should match")
	}

	// Alice encrypts a message to Bob using SealRaw with shared key
	plaintext := []byte("secret message for bob")
	ciphertext, err := crypto.SealRaw(sharedAB, plaintext)
	if err != nil {
		t.Fatalf("SealRaw: %v", err)
	}

	// Send encrypted ciphertext via relay
	sendBinary(t, connA, matchB, ciphertext)

	// Bob receives and decrypts with same shared key
	_, receivedCiphertext := readBinary(t, connB, 2*time.Second)
	decrypted, err := crypto.OpenRaw(sharedBA, receivedCiphertext)
	if err != nil {
		t.Fatalf("OpenRaw: %v", err)
	}
	if string(decrypted) != "secret message for bob" {
		t.Errorf("decrypted = %q, want %q", string(decrypted), "secret message for bob")
	}
}

// ==================== Edge Cases ====================

func TestRouting_InvalidFrame_TooShort(t *testing.T) {
	env := newTestEnv(t)
	idA, _, matchA, _, privA := env.registerUser(t)
	connA := env.connectWS(t, idA, matchA, privA)
	time.Sleep(50 * time.Millisecond)

	// Send a binary frame that's too short (< 8 bytes)
	connA.WriteMessage(websocket.BinaryMessage, []byte("short"))

	msg := readText(t, connA, 2*time.Second)
	if msg != "invalid frame" {
		t.Errorf("response = %q, want 'invalid frame'", msg)
	}
}

func TestRouting_TextFrameIgnored(t *testing.T) {
	env := newTestEnv(t)
	idA, _, matchA, _, privA := env.registerUser(t)
	connA := env.connectWS(t, idA, matchA, privA)
	time.Sleep(50 * time.Millisecond)

	// Send a text frame — session should ignore it (no crash, no response)
	connA.WriteMessage(websocket.TextMessage, []byte("some text"))

	// Send a valid binary frame to verify connection is still alive
	idB, _, matchB, _, privB := env.registerUser(t)
	pairHash := relay.PairHash(matchA, matchB)
	env.srv.GitHubCache().SetMatch(pairHash, true)

	connB := env.connectWS(t, idB, matchB, privB)
	time.Sleep(50 * time.Millisecond)

	sendBinary(t, connA, matchB, []byte("still alive"))
	_, payload := readBinary(t, connB, 2*time.Second)
	if string(payload) != "still alive" {
		t.Errorf("payload = %q, want 'still alive'", string(payload))
	}
}
