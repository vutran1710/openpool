# Stateless Relay Refactor Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate SQLite from the relay, replace MessagePack protocol with binary frames, remove key exchange, and make match notifications deliver peer pubkeys.

**Architecture:** Relay derives id_hash→bin_hash→match_hash from salt, fetches pubkeys from GitHub raw content, checks matches via GitHub raw content. Hub keyed by match_hash. Binary wire format: `[match_hash_8B][ciphertext]`. Text frames for control/status messages.

**Tech Stack:** Go, gorilla/websocket, ed25519, NaCl secretbox, GitHub raw content API

---

## File Structure

### New Files
- `internal/relay/github.go` — GitHub raw content fetcher (pubkey + match check with caching)

### Modified Files
- `internal/crypto/totp.go` — Simplify TOTP to sign only time_window
- `internal/crypto/conversation.go` — Add `SealRaw`/`OpenRaw` (raw bytes, no base64)
- `internal/cli/config/config.go` — Add `IDHash` to `PoolConfig` (per-pool, computed from pool_url:provider:user_id)
- `internal/relay/server.go` — Stateless auth, remove sync/match/store, add caches
- `internal/relay/session.go` — Binary frame routing, text frame control messages
- `internal/relay/hub.go` — Re-key by match_hash, add offline queue
- `internal/cli/relay/client.go` — id_hash connect, binary frames, remove key exchange
- `internal/cli/chat.go` — `<match_hash>` target, remove protocol import
- `internal/cli/tui/screens/matches.go` — MatchItem.MatchHash + PubKey, optional greeting
- `internal/cli/tui/screens/chat.go` — Wire to match_hash, remove protocol import
- `cmd/matchcrypt/main.go` — Updated encrypt payload (match_hash + pubkey)
- `cmd/relay/main.go` — Remove DB_PATH
- `templates/actions/pool-interest.yml` — Updated matchcrypt encrypt invocation

### Deleted Files
- `internal/relay/store.go` — Already deleted on branch
- `internal/protocol/types.go` — No more MessagePack frames
- `internal/protocol/codec.go` — No more MessagePack encoding

### Test Files
- `internal/crypto/totp_test.go` — Update for new TOTP signature
- `internal/crypto/conversation_test.go` — Tests for SealRaw/OpenRaw
- `internal/relay/relay_e2e_test.go` — Full rewrite
- `internal/relay/github_test.go` — Pubkey + match fetch with caching

---

### Task 1: Simplify TOTP to sign only time_window

**Files:**
- Modify: `internal/crypto/totp.go`
- Modify: `internal/crypto/totp_test.go`

- [ ] **Step 1: Read existing TOTP tests**

Run: `cat internal/crypto/totp_test.go`

- [ ] **Step 2: Update TOTP tests for new signature format**

The TOTP now signs `sha256(time_window)` only — no binHash/matchHash in signed message. Update `TOTPSign`/`TOTPVerify` to take no hash params (only priv/pub key + optional timeWindow for testing).

```go
// New signatures:
// TOTPSign(priv ed25519.PrivateKey) string
// TOTPSignAt(priv ed25519.PrivateKey, timeWindow int64) string
// TOTPVerify(sigHex string, pub ed25519.PublicKey) bool
```

Write tests:
```go
func TestTOTPSign_Valid(t *testing.T) {
    pub, priv, _ := ed25519.GenerateKey(nil)
    sig := TOTPSign(priv)
    if !TOTPVerify(sig, pub) {
        t.Fatal("valid signature should verify")
    }
}

func TestTOTPVerify_WrongKey(t *testing.T) {
    _, priv, _ := ed25519.GenerateKey(nil)
    pub2, _, _ := ed25519.GenerateKey(nil)
    sig := TOTPSign(priv)
    if TOTPVerify(sig, pub2) {
        t.Fatal("wrong key should not verify")
    }
}

func TestTOTPVerify_DriftWindow(t *testing.T) {
    pub, priv, _ := ed25519.GenerateKey(nil)
    now := time.Now().Unix() / 300
    sig := TOTPSignAt(priv, now-1)
    if !TOTPVerify(sig, pub) {
        t.Fatal("±1 window should verify")
    }
}

func TestTOTPVerify_Expired(t *testing.T) {
    pub, priv, _ := ed25519.GenerateKey(nil)
    now := time.Now().Unix() / 300
    sig := TOTPSignAt(priv, now-2)
    if TOTPVerify(sig, pub) {
        t.Fatal("±2 window should not verify")
    }
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `make test -- -run TestTOTP -v ./internal/crypto/`
Expected: FAIL (signature mismatch — old TOTPSign takes binHash, matchHash)

- [ ] **Step 4: Update TOTP implementation**

```go
package crypto

import (
    "crypto/ed25519"
    "crypto/sha256"
    "encoding/hex"
    "strconv"
    "time"
)

func TOTPSign(priv ed25519.PrivateKey) string {
    tw := time.Now().Unix() / 300
    return TOTPSignAt(priv, tw)
}

func TOTPSignAt(priv ed25519.PrivateKey, timeWindow int64) string {
    msg := totpMessage(timeWindow)
    sig := ed25519.Sign(priv, msg)
    return hex.EncodeToString(sig)
}

func TOTPVerify(sigHex string, pub ed25519.PublicKey) bool {
    sigBytes, err := hex.DecodeString(sigHex)
    if err != nil || len(sigBytes) != ed25519.SignatureSize {
        return false
    }
    now := time.Now().Unix() / 300
    for _, tw := range []int64{now, now - 1, now + 1} {
        msg := totpMessage(tw)
        if ed25519.Verify(pub, msg, sigBytes) {
            return true
        }
    }
    return false
}

func totpMessage(tw int64) []byte {
    h := sha256.Sum256([]byte(strconv.FormatInt(tw, 10)))
    return h[:]
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `make test -- -run TestTOTP -v ./internal/crypto/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/crypto/totp.go internal/crypto/totp_test.go
git commit -m "refactor: simplify TOTP — sign only time_window"
```

---

### Task 2: Add SealRaw / OpenRaw for binary wire format

**Files:**
- Modify: `internal/crypto/conversation.go`
- Modify: `internal/crypto/conversation_test.go`

- [ ] **Step 1: Write failing tests for SealRaw/OpenRaw**

```go
func TestSealRaw_OpenRaw_Roundtrip(t *testing.T) {
    key := make([]byte, 32)
    rand.Read(key)
    plaintext := []byte("hello world")

    sealed, err := SealRaw(key, plaintext)
    if err != nil {
        t.Fatal(err)
    }
    // sealed is raw bytes: version(1) + nonce(24) + ciphertext
    if sealed[0] != 0x01 {
        t.Fatalf("expected version 0x01, got 0x%02x", sealed[0])
    }
    if len(sealed) < 1+24+secretbox.Overhead {
        t.Fatal("sealed too short")
    }

    opened, err := OpenRaw(key, sealed)
    if err != nil {
        t.Fatal(err)
    }
    if string(opened) != "hello world" {
        t.Fatalf("got %q, want %q", opened, "hello world")
    }
}

func TestOpenRaw_WrongKey(t *testing.T) {
    key1 := make([]byte, 32)
    key2 := make([]byte, 32)
    rand.Read(key1)
    rand.Read(key2)

    sealed, _ := SealRaw(key1, []byte("secret"))
    _, err := OpenRaw(key2, sealed)
    if err == nil {
        t.Fatal("wrong key should fail")
    }
}

func TestSealRaw_InvalidKeyLength(t *testing.T) {
    _, err := SealRaw([]byte("short"), []byte("data"))
    if err == nil {
        t.Fatal("short key should fail")
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `make test -- -run TestSealRaw -v ./internal/crypto/`
Expected: FAIL (undefined: SealRaw)

- [ ] **Step 3: Implement SealRaw and OpenRaw**

Add to `internal/crypto/conversation.go`:

```go
// SealRaw encrypts plaintext with a 32-byte key, returning raw bytes:
// version(1) || nonce(24) || secretbox ciphertext.
// Unlike SealMessage, this returns []byte (not base64) for binary wire protocols.
func SealRaw(key, plaintext []byte) ([]byte, error) {
    if len(key) != 32 {
        return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
    }
    var k [32]byte
    copy(k[:], key)
    var nonce [24]byte
    if _, err := rand.Read(nonce[:]); err != nil {
        return nil, fmt.Errorf("generating nonce: %w", err)
    }
    ciphertext := secretbox.Seal(nil, plaintext, &nonce, &k)
    out := make([]byte, 0, 1+24+len(ciphertext))
    out = append(out, e2eVersion)
    out = append(out, nonce[:]...)
    out = append(out, ciphertext...)
    return out, nil
}

// OpenRaw decrypts raw bytes produced by SealRaw.
func OpenRaw(key, sealed []byte) ([]byte, error) {
    if len(key) != 32 {
        return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
    }
    minLen := 1 + 24 + secretbox.Overhead
    if len(sealed) < minLen {
        return nil, fmt.Errorf("message too short: got %d bytes, need at least %d", len(sealed), minLen)
    }
    if sealed[0] != e2eVersion {
        return nil, fmt.Errorf("unsupported version byte: 0x%02x", sealed[0])
    }
    var nonce [24]byte
    copy(nonce[:], sealed[1:25])
    var k [32]byte
    copy(k[:], key)
    plaintext, ok := secretbox.Open(nil, sealed[25:], &nonce, &k)
    if !ok {
        return nil, fmt.Errorf("decryption failed: authentication tag mismatch")
    }
    return plaintext, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `make test -- -run "TestSealRaw|TestOpenRaw" -v ./internal/crypto/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/crypto/conversation.go internal/crypto/conversation_test.go
git commit -m "feat: add SealRaw/OpenRaw for binary wire protocol"
```

---

### Task 3: GitHub raw content fetcher with caching

**Files:**
- Create: `internal/relay/github.go`
- Create: `internal/relay/github_test.go`

- [ ] **Step 1: Write failing tests**

```go
package relay

import (
    "crypto/ed25519"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"
)

func TestFetchPubkey_Success(t *testing.T) {
    pub, _, _ := ed25519.GenerateKey(nil)
    binData := append(pub, make([]byte, 100)...) // 32B pubkey + fake profile

    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write(binData)
    }))
    defer srv.Close()

    gc := NewGitHubCache(srv.URL + "/owner/pool")
    got, err := gc.FetchPubkey("abcdef0123456789")
    if err != nil {
        t.Fatal(err)
    }
    if !pub.Equal(ed25519.PublicKey(got)) {
        t.Fatal("pubkey mismatch")
    }
}

func TestFetchPubkey_Cached(t *testing.T) {
    calls := 0
    pub, _, _ := ed25519.GenerateKey(nil)
    binData := append(pub, make([]byte, 100)...)

    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        calls++
        w.Write(binData)
    }))
    defer srv.Close()

    gc := NewGitHubCache(srv.URL + "/owner/pool")
    gc.FetchPubkey("abc123")
    gc.FetchPubkey("abc123")
    if calls != 1 {
        t.Fatalf("expected 1 fetch, got %d", calls)
    }
}

func TestFetchPubkey_NotFound(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(404)
    }))
    defer srv.Close()

    gc := NewGitHubCache(srv.URL + "/owner/pool")
    _, err := gc.FetchPubkey("unknown")
    if err == nil {
        t.Fatal("should error on 404")
    }
}

func TestCheckMatch_Positive(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"match_hash_1":"a","match_hash_2":"b"}`))
    }))
    defer srv.Close()

    gc := NewGitHubCache(srv.URL + "/owner/pool")
    ok, err := gc.CheckMatch("abcdef012345")
    if err != nil {
        t.Fatal(err)
    }
    if !ok {
        t.Fatal("should be matched")
    }
}

func TestCheckMatch_Negative(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(404)
    }))
    defer srv.Close()

    gc := NewGitHubCache(srv.URL + "/owner/pool")
    ok, err := gc.CheckMatch("unknown_pair")
    if err != nil {
        t.Fatal(err)
    }
    if ok {
        t.Fatal("should not be matched")
    }
}

func TestCheckMatch_CacheTTL(t *testing.T) {
    calls := 0
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        calls++
        w.Write([]byte(`{}`))
    }))
    defer srv.Close()

    gc := NewGitHubCache(srv.URL + "/owner/pool")
    gc.matchTTL = 50 * time.Millisecond // short TTL for test
    gc.CheckMatch("pair1")
    gc.CheckMatch("pair1") // cached
    if calls != 1 {
        t.Fatalf("expected 1 call, got %d", calls)
    }
    time.Sleep(60 * time.Millisecond)
    gc.CheckMatch("pair1") // expired
    if calls != 2 {
        t.Fatalf("expected 2 calls after TTL, got %d", calls)
    }
}

func TestCheckMatch_TransientError_NotCached(t *testing.T) {
    calls := 0
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        calls++
        w.WriteHeader(500)
    }))
    defer srv.Close()

    gc := NewGitHubCache(srv.URL + "/owner/pool")
    _, err := gc.CheckMatch("pair1")
    if err == nil {
        t.Fatal("should error on 500")
    }
    _, _ = gc.CheckMatch("pair1") // should NOT be cached
    if calls != 2 {
        t.Fatalf("transient error should not be cached, got %d calls", calls)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `make test -- -run "TestFetch|TestCheckMatch" -v ./internal/relay/`
Expected: FAIL (undefined: NewGitHubCache)

- [ ] **Step 3: Implement GitHubCache**

Create `internal/relay/github.go`:

```go
package relay

import (
    "fmt"
    "io"
    "net/http"
    "sync"
    "time"
)

const (
    defaultMatchTTL = 5 * time.Minute
    pubkeySize      = 32
)

type matchCacheEntry struct {
    exists    bool
    fetchedAt time.Time
}

// GitHubCache fetches and caches pubkeys and match files from GitHub raw content.
type GitHubCache struct {
    baseURL  string // e.g., "https://raw.githubusercontent.com/owner/pool/main"
    client   *http.Client

    pubkeys  sync.Map // bin_hash → []byte
    matches  sync.Map // pair_hash → matchCacheEntry
    matchTTL time.Duration
}

// NewGitHubCache creates a cache that fetches from the given base URL.
// baseURL should be like "https://raw.githubusercontent.com/owner/pool/main"
// or a test server URL + "/owner/pool".
func NewGitHubCache(baseURL string) *GitHubCache {
    return &GitHubCache{
        baseURL:  baseURL,
        client:   &http.Client{Timeout: 10 * time.Second},
        matchTTL: defaultMatchTTL,
    }
}

// FetchPubkey returns the ed25519 pubkey (first 32 bytes of .bin file).
// Cached forever (pubkeys are immutable).
func (gc *GitHubCache) FetchPubkey(binHash string) ([]byte, error) {
    if v, ok := gc.pubkeys.Load(binHash); ok {
        return v.([]byte), nil
    }

    url := gc.baseURL + "/users/" + binHash + ".bin"
    resp, err := gc.client.Get(url)
    if err != nil {
        return nil, fmt.Errorf("fetching pubkey: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode == 404 {
        return nil, fmt.Errorf("unknown user: %s", binHash)
    }
    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("fetching pubkey: HTTP %d", resp.StatusCode)
    }

    data, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("reading pubkey: %w", err)
    }
    if len(data) < pubkeySize {
        return nil, fmt.Errorf("bin file too short: %d bytes", len(data))
    }

    pubkey := make([]byte, pubkeySize)
    copy(pubkey, data[:pubkeySize])
    gc.pubkeys.Store(binHash, pubkey)
    return pubkey, nil
}

// CheckMatch checks if a match file exists for the given pair_hash.
// Results cached with 5-min TTL. Only HTTP 200 and 404 are cached.
// 5xx/network errors return error and are NOT cached.
func (gc *GitHubCache) CheckMatch(pairHash string) (bool, error) {
    if v, ok := gc.matches.Load(pairHash); ok {
        entry := v.(matchCacheEntry)
        if time.Since(entry.fetchedAt) < gc.matchTTL {
            return entry.exists, nil
        }
        gc.matches.Delete(pairHash)
    }

    url := gc.baseURL + "/matches/" + pairHash + ".json"
    resp, err := gc.client.Get(url)
    if err != nil {
        return false, fmt.Errorf("match check failed")
    }
    defer resp.Body.Close()
    io.Copy(io.Discard, resp.Body)

    if resp.StatusCode == 200 {
        gc.matches.Store(pairHash, matchCacheEntry{exists: true, fetchedAt: time.Now()})
        return true, nil
    }
    if resp.StatusCode == 404 {
        gc.matches.Store(pairHash, matchCacheEntry{exists: false, fetchedAt: time.Now()})
        return false, nil
    }

    // 5xx or other — transient, do NOT cache
    return false, fmt.Errorf("match check failed")
}

// SetPubkey injects a pubkey into the cache (for testing).
func (gc *GitHubCache) SetPubkey(binHash string, pubkey []byte) {
    gc.pubkeys.Store(binHash, pubkey)
}

// SetMatch injects a match result into the cache (for testing).
func (gc *GitHubCache) SetMatch(pairHash string, exists bool) {
    gc.matches.Store(pairHash, matchCacheEntry{exists: exists, fetchedAt: time.Now()})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `make test -- -run "TestFetch|TestCheckMatch" -v ./internal/relay/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/relay/github.go internal/relay/github_test.go
git commit -m "feat: GitHub raw content fetcher with caching for relay"
```

---

### Task 4: Rewrite hub.go — re-key by match_hash + offline queue

**Files:**
- Modify: `internal/relay/hub.go`

- [ ] **Step 1: Rewrite hub.go**

```go
package relay

import (
    "log"
    "sync"

    "github.com/gorilla/websocket"
)

const maxQueueSize = 20

// Hub manages active WebSocket sessions keyed by match_hash.
type Hub struct {
    mu       sync.RWMutex
    sessions map[string]*Session // match_hash → session
    queue    map[string][][]byte // match_hash → pending binary messages
}

func NewHub() *Hub {
    return &Hub{
        sessions: make(map[string]*Session),
        queue:    make(map[string][][]byte),
    }
}

// Register adds a session, closing any existing session for the same match_hash.
// Flushes queued messages to the new session.
func (h *Hub) Register(matchHash string, s *Session) {
    h.mu.Lock()
    if old, ok := h.sessions[matchHash]; ok && old != s {
        old.conn.Close()
    }
    h.sessions[matchHash] = s

    // Flush queued messages
    queued := h.queue[matchHash]
    delete(h.queue, matchHash)
    h.mu.Unlock()

    for _, msg := range queued {
        s.writeMu.Lock()
        s.conn.WriteMessage(websocket.BinaryMessage, msg)
        s.writeMu.Unlock()
    }

    log.Printf("hub: registered %s (flushed %d queued)", matchHash, len(queued))
}

// Unregister removes a session (only if it matches the current one).
func (h *Hub) Unregister(matchHash string, s *Session) {
    h.mu.Lock()
    defer h.mu.Unlock()
    if existing, ok := h.sessions[matchHash]; ok && existing == s {
        delete(h.sessions, matchHash)
        log.Printf("hub: unregistered %s", matchHash)
    }
}

// Send routes a binary message to a target. Returns status:
// "sent" if delivered, "queued" if target offline, "queue full" if queue at capacity.
func (h *Hub) Send(targetMatchHash string, data []byte) string {
    h.mu.RLock()
    s, online := h.sessions[targetMatchHash]
    h.mu.RUnlock()

    if online {
        s.writeMu.Lock()
        s.conn.WriteMessage(websocket.BinaryMessage, data)
        s.writeMu.Unlock()
        return ""
    }

    h.mu.Lock()
    defer h.mu.Unlock()
    q := h.queue[targetMatchHash]
    if len(q) >= maxQueueSize {
        return "queue full"
    }
    h.queue[targetMatchHash] = append(q, data)
    return "queued"
}

func (h *Hub) OnlineCount() int {
    h.mu.RLock()
    defer h.mu.RUnlock()
    return len(h.sessions)
}

func (h *Hub) QueuedCount() int {
    h.mu.RLock()
    defer h.mu.RUnlock()
    total := 0
    for _, q := range h.queue {
        total += len(q)
    }
    return total
}
```

- [ ] **Step 2: Write hub unit tests**

Create or update `internal/relay/hub_test.go`:

```go
package relay

import (
    "testing"

    "github.com/gorilla/websocket"
)

func TestHub_RegisterAndSend(t *testing.T) {
    hub := NewHub()
    // Create a mock session (need a real websocket for Send to work,
    // so use httptest + upgrader in integration tests)
    if hub.OnlineCount() != 0 {
        t.Fatal("expected 0 online")
    }
}

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
```

- [ ] **Step 3: Run tests**

Run: `make test -- -run TestHub -v ./internal/relay/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/relay/hub.go internal/relay/hub_test.go
git commit -m "refactor: hub keyed by match_hash with offline queue + unit tests"
```

---

### Task 5: Rewrite server.go — stateless auth + caches

**Files:**
- Modify: `internal/relay/server.go`

- [ ] **Step 1: Rewrite server.go**

```go
package relay

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "log"
    "net/http"

    "github.com/gorilla/websocket"
    "github.com/vutran1710/dating-dev/internal/crypto"
)

type Server struct {
    hub     *Hub
    gh      *GitHubCache
    poolURL string
    salt    string
    upgrader websocket.Upgrader
}

type ServerConfig struct {
    PoolURL  string
    Salt     string
    RawBaseURL string // override for testing (default: https://raw.githubusercontent.com/{PoolURL}/main)
}

func NewServer(cfg ServerConfig) *Server {
    baseURL := cfg.RawBaseURL
    if baseURL == "" {
        baseURL = "https://raw.githubusercontent.com/" + cfg.PoolURL + "/main"
    }
    return &Server{
        hub:     NewHub(),
        gh:      NewGitHubCache(baseURL),
        poolURL: cfg.PoolURL,
        salt:    cfg.Salt,
        upgrader: websocket.Upgrader{
            CheckOrigin: func(r *http.Request) bool { return true },
        },
    }
}

// GitHubCache returns the server's cache for test injection.
func (s *Server) GitHubCache() *GitHubCache {
    return s.gh
}

// BinHash derives bin_hash from id_hash using the server's salt.
func (s *Server) BinHash(idHash string) string {
    h := sha256.Sum256([]byte(s.salt + ":" + idHash))
    return hex.EncodeToString(h[:])[:16]
}

// MatchHash derives match_hash from bin_hash using the server's salt.
func (s *Server) MatchHash(binHash string) string {
    h := sha256.Sum256([]byte(s.salt + ":" + binHash))
    return hex.EncodeToString(h[:])[:16]
}

// PairHash computes the match pair identifier.
func PairHash(matchHashA, matchHashB string) string {
    a, b := matchHashA, matchHashB
    if a > b {
        a, b = b, a
    }
    h := sha256.Sum256([]byte(a + ":" + b))
    return hex.EncodeToString(h[:])[:12]
}

func (s *Server) Handler() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("GET /ws", s.HandleWS)
    mux.HandleFunc("GET /health", s.HandleHealth)
    return mux
}

func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
    idHash := r.URL.Query().Get("id")
    matchHash := r.URL.Query().Get("match")
    sigHex := r.URL.Query().Get("sig")

    if idHash == "" || matchHash == "" || sigHex == "" {
        http.Error(w, "missing id, match, or sig", http.StatusUnauthorized)
        return
    }

    // Derive and validate chain
    binHash := s.BinHash(idHash)
    expectedMatch := s.MatchHash(binHash)
    if expectedMatch != matchHash {
        http.Error(w, "invalid identity", http.StatusUnauthorized)
        return
    }

    // Fetch pubkey from GitHub
    pubkey, err := s.gh.FetchPubkey(binHash)
    if err != nil {
        if err.Error() == "unknown user: "+binHash {
            http.Error(w, "unknown user", http.StatusUnauthorized)
        } else {
            http.Error(w, "service unavailable", http.StatusServiceUnavailable)
        }
        return
    }

    // Verify TOTP signature
    if !crypto.TOTPVerify(sigHex, pubkey) {
        http.Error(w, "invalid signature", http.StatusUnauthorized)
        return
    }

    conn, err := s.upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Printf("upgrade: %v", err)
        return
    }

    sess := newSession(conn, s, matchHash)
    go sess.run()
}

func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]any{
        "status": "ok",
        "online": s.hub.OnlineCount(),
        "queued": s.hub.QueuedCount(),
    })
}

func (s *Server) ListenAndServe(addr string) error {
    log.Printf("relay server listening on %s (pool=%s)", addr, s.poolURL)
    return http.ListenAndServe(addr, s.Handler())
}

func Addr(port string) string {
    if port == "" {
        port = "8081"
    }
    return fmt.Sprintf(":%s", port)
}
```

- [ ] **Step 2: Write hash derivation unit tests**

Add to `internal/relay/server_test.go`:

```go
package relay

import "testing"

func TestBinHash_Deterministic(t *testing.T) {
    srv := &Server{salt: "test-salt"}
    h1 := srv.BinHash("abc123")
    h2 := srv.BinHash("abc123")
    if h1 != h2 {
        t.Fatal("BinHash should be deterministic")
    }
    if len(h1) != 16 {
        t.Fatalf("BinHash should be 16 hex chars, got %d", len(h1))
    }
}

func TestMatchHash_Deterministic(t *testing.T) {
    srv := &Server{salt: "test-salt"}
    h := srv.MatchHash("abc123")
    if len(h) != 16 {
        t.Fatalf("MatchHash should be 16 hex chars, got %d", len(h))
    }
}

func TestPairHash_Order(t *testing.T) {
    h1 := PairHash("aaa", "bbb")
    h2 := PairHash("bbb", "aaa")
    if h1 != h2 {
        t.Fatal("PairHash should be order-independent")
    }
    if len(h1) != 12 {
        t.Fatalf("PairHash should be 12 hex chars, got %d", len(h1))
    }
}

func TestChain_Derivation(t *testing.T) {
    srv := &Server{salt: "test-salt"}
    bin := srv.BinHash("some-id-hash")
    match := srv.MatchHash(bin)
    // Different inputs produce different outputs
    if bin == match {
        t.Fatal("bin_hash and match_hash should differ")
    }
}
```

- [ ] **Step 3: Run tests**

Run: `make test -- -run "TestBinHash|TestMatchHash|TestPairHash|TestChain" -v ./internal/relay/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/relay/server.go internal/relay/server_test.go
git commit -m "refactor: stateless relay server with hash derivation tests"
```

---

### Task 6: Rewrite session.go — binary frame routing

**Files:**
- Modify: `internal/relay/session.go`

- [ ] **Step 1: Rewrite session.go**

```go
package relay

import (
    "encoding/hex"
    "log"
    "sync"
    "time"

    "github.com/gorilla/websocket"
)

const matchHashRawLen = 8 // 16 hex chars = 8 bytes

type Session struct {
    conn      *websocket.Conn
    server    *Server
    writeMu   sync.Mutex
    matchHash string
}

func newSession(conn *websocket.Conn, srv *Server, matchHash string) *Session {
    return &Session{
        conn:      conn,
        server:    srv,
        matchHash: matchHash,
    }
}

func (s *Session) run() {
    s.server.hub.Register(s.matchHash, s)
    log.Printf("session: connected %s", s.matchHash)

    defer func() {
        s.server.hub.Unregister(s.matchHash, s)
        s.conn.Close()
        log.Printf("session: disconnected %s", s.matchHash)
    }()

    for {
        msgType, data, err := s.conn.ReadMessage()
        if err != nil {
            return
        }

        // Ignore text frames from client (text is server→client only)
        if msgType != websocket.BinaryMessage {
            continue
        }

        if len(data) <= matchHashRawLen {
            s.sendText("invalid frame")
            continue
        }

        // Extract target match_hash
        targetMatchHash := hex.EncodeToString(data[:matchHashRawLen])
        ciphertext := data[matchHashRawLen:]

        // Check match authorization
        pairHash := PairHash(s.matchHash, targetMatchHash)
        matched, err := s.server.gh.CheckMatch(pairHash)
        if err != nil {
            s.sendText("match check failed")
            continue
        }
        if !matched {
            s.sendText("not matched")
            continue
        }

        // Build forwarded frame: [sender_match_hash][ciphertext]
        senderBytes, _ := hex.DecodeString(s.matchHash)
        forwarded := append(senderBytes, ciphertext...)

        // Route via hub
        status := s.server.hub.Send(targetMatchHash, forwarded)
        if status != "" {
            s.sendText(status) // "queued" or "queue full"
        }
    }
}

func (s *Session) sendText(msg string) {
    s.writeMu.Lock()
    defer s.writeMu.Unlock()
    s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
    s.conn.WriteMessage(websocket.TextMessage, []byte(msg))
}
```

- [ ] **Step 2: Verify the relay package compiles**

Run: `go build ./internal/relay/`
Expected: PASS (all relay files now consistent)

- [ ] **Step 3: Commit**

```bash
git add internal/relay/session.go
git commit -m "refactor: session with binary frame routing and text control messages"
```

---

### Task 7: Delete protocol package

**Files:**
- Delete: `internal/protocol/types.go`
- Delete: `internal/protocol/codec.go`

- [ ] **Step 1: Delete protocol files**

```bash
rm -rf internal/protocol/
```

- [ ] **Step 2: Note files that will need updating**

Run: `grep -r '"github.com/vutran1710/dating-dev/internal/protocol"' --include='*.go' -l`

These files still import `protocol` and will be fixed in Tasks 8 (client.go), 10 (chat.go), and 13 (TUI chat.go). The codebase will NOT compile until those tasks are done — this is expected.

- [ ] **Step 3: Commit**

```bash
git add -A internal/protocol/
git commit -m "delete: remove protocol package — replaced by binary frames"
```

---

### Task 8: Rewrite relay client — binary frames, no key exchange

**Files:**
- Modify: `internal/cli/relay/client.go`

- [ ] **Step 1: Rewrite client.go**

```go
package relay

import (
    "context"
    "crypto/ed25519"
    "encoding/hex"
    "fmt"
    "strings"
    "sync"
    "time"

    "github.com/gorilla/websocket"
    "github.com/vutran1710/dating-dev/internal/crypto"
)

type Client struct {
    conn      *websocket.Conn
    url       string
    poolURL   string
    idHash    string
    matchHash string
    pub       ed25519.PublicKey
    priv      ed25519.PrivateKey

    mu       sync.Mutex
    onMsg    func(senderMatchHash string, plaintext []byte)
    onCtrl   func(msg string)
    done     chan struct{}
    closed   bool

    keys     map[string][]byte         // peer_match_hash → conversation key
    peerKeys map[string]ed25519.PublicKey // peer_match_hash → peer pubkey
}

type Config struct {
    RelayURL  string
    PoolURL   string
    IDHash    string
    MatchHash string
    Pub       ed25519.PublicKey
    Priv      ed25519.PrivateKey
}

func NewClient(cfg Config) *Client {
    return &Client{
        url:       cfg.RelayURL,
        poolURL:   cfg.PoolURL,
        idHash:    cfg.IDHash,
        matchHash: cfg.MatchHash,
        pub:       cfg.Pub,
        priv:      cfg.Priv,
        done:      make(chan struct{}),
        keys:      make(map[string][]byte),
        peerKeys:  make(map[string]ed25519.PublicKey),
    }
}

// SetPeerKey registers a peer's pubkey for key derivation (from match notification).
func (c *Client) SetPeerKey(peerMatchHash string, peerPub ed25519.PublicKey) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.peerKeys[peerMatchHash] = peerPub
}

func (c *Client) Connect(ctx context.Context) error {
    sig := crypto.TOTPSign(c.priv)
    wsURL := c.wsURL() + "/ws?id=" + c.idHash + "&match=" + c.matchHash + "&sig=" + sig

    dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
    conn, resp, err := dialer.DialContext(ctx, wsURL, nil)
    if err != nil {
        if resp != nil && resp.StatusCode == 401 {
            return fmt.Errorf("authentication failed")
        }
        return fmt.Errorf("connecting to relay: %w", err)
    }
    c.conn = conn
    go c.readLoop()
    return nil
}

func (c *Client) Close() {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.closed {
        return
    }
    c.closed = true
    close(c.done)
    if c.conn != nil {
        c.conn.Close()
    }
}

// OnMessage sets the callback for incoming decrypted messages.
func (c *Client) OnMessage(fn func(senderMatchHash string, plaintext []byte)) {
    c.onMsg = fn
}

// OnControl sets the callback for control/status text frames.
func (c *Client) OnControl(fn func(msg string)) {
    c.onCtrl = fn
}

// SendMessage encrypts and sends a message to a peer.
func (c *Client) SendMessage(targetMatchHash string, plaintext string) error {
    key, err := c.getOrDeriveKey(targetMatchHash)
    if err != nil {
        return fmt.Errorf("deriving key: %w", err)
    }

    sealed, err := crypto.SealRaw(key, []byte(plaintext))
    if err != nil {
        return fmt.Errorf("encrypting: %w", err)
    }

    targetBytes, err := hex.DecodeString(targetMatchHash)
    if err != nil {
        return fmt.Errorf("invalid target match hash: %w", err)
    }

    frame := append(targetBytes, sealed...)

    c.mu.Lock()
    defer c.mu.Unlock()
    if c.conn == nil {
        return fmt.Errorf("not connected")
    }
    return c.conn.WriteMessage(websocket.BinaryMessage, frame)
}

func (c *Client) getOrDeriveKey(peerMatchHash string) ([]byte, error) {
    c.mu.Lock()
    key, ok := c.keys[peerMatchHash]
    c.mu.Unlock()
    if ok {
        return key, nil
    }

    c.mu.Lock()
    peerPub, hasPub := c.peerKeys[peerMatchHash]
    c.mu.Unlock()
    if !hasPub {
        return nil, fmt.Errorf("no pubkey for peer %s — check match notifications", peerMatchHash)
    }

    shared, err := crypto.SharedSecret(c.priv, peerPub)
    if err != nil {
        return nil, fmt.Errorf("computing shared secret: %w", err)
    }

    key, err = crypto.DeriveConversationKey(shared, c.matchHash, peerMatchHash, c.poolURL)
    if err != nil {
        return nil, fmt.Errorf("deriving conversation key: %w", err)
    }

    c.mu.Lock()
    c.keys[peerMatchHash] = key
    c.mu.Unlock()
    return key, nil
}

func (c *Client) wsURL() string {
    u := c.url
    u = strings.TrimSuffix(u, "/")
    u = strings.Replace(u, "http://", "ws://", 1)
    u = strings.Replace(u, "https://", "wss://", 1)
    return u
}

func (c *Client) readLoop() {
    for {
        select {
        case <-c.done:
            return
        default:
        }

        msgType, data, err := c.conn.ReadMessage()
        if err != nil {
            return
        }

        if msgType == websocket.TextMessage {
            if c.onCtrl != nil {
                c.onCtrl(string(data))
            }
            continue
        }

        if msgType != websocket.BinaryMessage || len(data) <= 8 {
            continue
        }

        senderMatchHash := hex.EncodeToString(data[:8])
        ciphertext := data[8:]

        go func() {
            key, err := c.getOrDeriveKey(senderMatchHash)
            if err != nil {
                return
            }
            plaintext, err := crypto.OpenRaw(key, ciphertext)
            if err != nil {
                return
            }
            if c.onMsg != nil {
                c.onMsg(senderMatchHash, plaintext)
            }
        }()
    }
}
```

- [ ] **Step 2: Delete client_test.go (will be rewritten with e2e tests)**

Check if `internal/cli/relay/client_test.go` exists and references old protocol types. If so, delete it — tests will be covered by the relay e2e test rewrite.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/cli/relay/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/cli/relay/client.go
git rm -f internal/cli/relay/client_test.go 2>/dev/null; true
git commit -m "refactor: relay client — binary frames, no key exchange, id_hash connect"
```

---

### Task 9: Add IDHash to PoolConfig

**Files:**
- Modify: `internal/cli/config/config.go`

- [ ] **Step 1: Add IDHash field to PoolConfig**

```go
type PoolConfig struct {
    Name           string `toml:"name"`
    Repo           string `toml:"repo"`
    OperatorPubKey string `toml:"operator_public_key"`
    RelayURL       string `toml:"relay_url,omitempty"`
    Status         string `toml:"status,omitempty"`
    UserHash       string `toml:"user_hash,omitempty"`
    PendingIssue   int    `toml:"pending_issue,omitempty"`
    IDHash         string `toml:"id_hash,omitempty"`
    BinHash        string `toml:"bin_hash,omitempty"`
    MatchHash      string `toml:"match_hash,omitempty"`
}
```

`IDHash` is set during registration (when the CLI receives hashes from the Action response). It's `sha256(pool_url:provider:user_id)` — the same value the relay will derive from `id_hash` param.

- [ ] **Step 2: Update registration flow to persist IDHash**

In `internal/cli/pool.go`, find the two places where `BinHash`/`MatchHash` are set:

1. Around line 418 (initial registration):
```go
BinHash:   binHash,
MatchHash: matchHash,
```
Add `IDHash` to the same struct:
```go
IDHash:    string(crypto.UserHash(pool.Repo, cfg.User.Provider, cfg.User.ProviderUserID)),
BinHash:   binHash,
MatchHash: matchHash,
```

2. Around line 500 (polling completion):
```go
cfg.Pools[i].BinHash = binHash
cfg.Pools[i].MatchHash = matchHash
```
Add:
```go
cfg.Pools[i].IDHash = string(crypto.UserHash(cfg.Pools[i].Repo, cfg.User.Provider, cfg.User.ProviderUserID))
cfg.Pools[i].BinHash = binHash
cfg.Pools[i].MatchHash = matchHash
```

- [ ] **Step 3: Verify it compiles and existing tests pass**

Run: `make test`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/cli/config/config.go
git commit -m "feat: add IDHash to PoolConfig for relay authentication"
```

---

### Task 10: Update chat.go CLI command

**Files:**
- Modify: `internal/cli/chat.go`

- [ ] **Step 1: Update chat.go**

Change `dating chat <bin_hash>` to `dating chat <match_hash>`. Remove `protocol` import. Update callbacks.

```go
package cli

import (
    "bufio"
    "context"
    "fmt"
    "os"
    "time"

    "github.com/spf13/cobra"
    "github.com/vutran1710/dating-dev/internal/cli/config"
    relayclient "github.com/vutran1710/dating-dev/internal/cli/relay"
    "github.com/vutran1710/dating-dev/internal/crypto"
)

func newChatCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "chat <match_hash>",
        Short: "Chat with a match via relay",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            cfg, err := config.Load()
            if err != nil {
                return err
            }

            pool := cfg.ActivePool()
            if pool == nil {
                printError("No active pool.")
                return nil
            }
            if pool.RelayURL == "" {
                printError("No relay URL configured for this pool.")
                return nil
            }
            if pool.MatchHash == "" {
                printError("Not registered. Complete registration first.")
                return nil
            }

            pub, priv, err := crypto.LoadKeyPair(config.KeysDir())
            if err != nil {
                return fmt.Errorf("loading keys: %w", err)
            }

            targetMatchHash := args[0]

            idHash := pool.IDHash

            client := relayclient.NewClient(relayclient.Config{
                RelayURL:  pool.RelayURL,
                PoolURL:   pool.Repo,
                IDHash:    idHash,
                MatchHash: pool.MatchHash,
                Pub:       pub,
                Priv:      priv,
            })

            // Load peer's pubkey from match notifications
            peerPub, err := loadPeerPubkey(pool, targetMatchHash, priv)
            if err != nil {
                return fmt.Errorf("loading peer pubkey: %w", err)
            }
            client.SetPeerKey(targetMatchHash, peerPub)

            ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
            defer cancel()

            fmt.Println("  Connecting to relay...")
            if err := client.Connect(ctx); err != nil {
                return fmt.Errorf("connecting to relay: %w", err)
            }
            defer client.Close()

            printSuccess("Connected!")
            fmt.Println()
            printDim(fmt.Sprintf("  Chatting with %s", targetMatchHash[:12]+"..."))
            printDim("  Type a message and press Enter. /exit to leave.")
            fmt.Println()

            client.OnMessage(func(senderMatchHash string, plaintext []byte) {
                timestamp := time.Now().Format("15:04")
                sender := senderMatchHash[:12] + "..."
                fmt.Printf("\r  %s %s: %s\n", dim.Render("["+timestamp+"]"), brand.Render(sender), string(plaintext))
                fmt.Printf("%s ", dim.Render("you>"))
            })

            client.OnControl(func(msg string) {
                fmt.Printf("\r  %s %s\n", dim.Render("[relay]"), msg)
                fmt.Printf("%s ", dim.Render("you>"))
            })

            scanner := bufio.NewScanner(os.Stdin)
            for {
                fmt.Printf("%s ", dim.Render("you>"))
                if !scanner.Scan() {
                    break
                }

                input := scanner.Text()
                if input == "/exit" {
                    printDim("  Left chat.")
                    return nil
                }
                if input == "" {
                    continue
                }

                if err := client.SendMessage(targetMatchHash, input); err != nil {
                    fmt.Printf("  %s\n", dim.Render("(send failed: "+err.Error()+")"))
                    continue
                }

                timestamp := time.Now().Format("15:04")
                fmt.Printf("  %s %s: %s\n", dim.Render("["+timestamp+"]"), brand.Render("you"), input)
            }
            return nil
        },
    }
}
```

Add `loadPeerPubkey` helper (reuses `decryptMatchNotification` logic from matches screen — or extract to shared package):

```go
// loadPeerPubkey scans closed interest PRs for the match notification containing
// the given peer match_hash, decrypts it, and returns the peer's pubkey.
func loadPeerPubkey(pool *config.PoolConfig, peerMatchHash string, priv ed25519.PrivateKey) (ed25519.PublicKey, error) {
    ghToken, err := resolveGHToken()
    if err != nil {
        return nil, err
    }
    client := gh.NewPool(pool.Repo, ghToken)
    prs, err := client.Client().ListPullRequests(context.Background(), "closed")
    if err != nil {
        return nil, err
    }
    for _, pr := range prs {
        hasLabel := false
        for _, l := range pr.Labels {
            if l.Name == "interest" {
                hasLabel = true
                break
            }
        }
        if !hasLabel {
            continue
        }
        comments, err := client.Client().ListIssueComments(context.Background(), pr.Number)
        if err != nil {
            continue
        }
        for _, c := range comments {
            if c.User.Login != "github-actions[bot]" {
                continue
            }
            item, err := decryptMatchNotification(c.Body, priv)
            if err != nil {
                continue
            }
            if item.MatchHash == peerMatchHash {
                return item.PubKey, nil
            }
        }
    }
    return nil, fmt.Errorf("no match notification found for %s", peerMatchHash)
}
```

Note: `resolveGHToken` and `decryptMatchNotification` should be moved to a shared location (e.g., `internal/cli/matches_util.go`) since both `chat.go` and `screens/matches.go` need them. Alternatively, import from screens package.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/cli/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/cli/chat.go
git commit -m "refactor: chat command uses match_hash, loads peer pubkey from notifications"
```

---

### Task 11: Update matchcrypt — deliver pubkey in match notification

**Files:**
- Modify: `cmd/matchcrypt/main.go`
- Modify: `templates/actions/pool-interest.yml`

- [ ] **Step 1: Update cmdEncrypt in matchcrypt**

```go
func cmdEncrypt() {
    userPubHex := envOrArg("--user-pubkey", "")
    matchHash := envOrArg("--match-hash", "")
    peerPubHex := envOrArg("--peer-pubkey", "")
    greeting := envOrArg("--greeting", "")

    userPub, err := hex.DecodeString(userPubHex)
    if err != nil || len(userPub) != ed25519.PublicKeySize {
        log.Fatal("invalid user pubkey")
    }

    payload, _ := json.Marshal(map[string]string{
        "matched_match_hash": matchHash,
        "greeting":           greeting,
        "pubkey":             peerPubHex,
    })

    encrypted, err := crypto.Encrypt(ed25519.PublicKey(userPub), payload)
    if err != nil {
        log.Fatalf("encrypt: %v", err)
    }

    fmt.Print(base64.StdEncoding.EncodeToString(encrypted))
}
```

Also update the doc comment at the top:
```go
// Encrypt match notification for a user:
//   matchcrypt encrypt --user-pubkey <hex> --match-hash <peer_match_hash> --peer-pubkey <peer_pub_hex> --greeting "message"
//   → stdout: base64 encrypted blob containing {matched_match_hash, greeting, pubkey}
```

- [ ] **Step 2: Update pool-interest.yml**

Replace the two matchcrypt encrypt lines:

```yaml
          # Encrypt match notification for the current PR author
          # → tells them the reciprocal person's match_hash + pubkey + greeting
          AUTHOR_NOTIFICATION=$(./matchcrypt encrypt --user-pubkey "$AUTHOR_PUBKEY" --match-hash "$RECIP_MATCH_HASH" --peer-pubkey "$RECIP_PUBKEY" --greeting "$RECIP_GREETING")
          gh pr comment "$PR_NUMBER" --repo "$REPO" --body "$AUTHOR_NOTIFICATION"

          # Encrypt match notification for the reciprocal PR author
          # → tells them the current person's match_hash + pubkey + greeting
          RECIP_NOTIFICATION=$(./matchcrypt encrypt --user-pubkey "$RECIP_PUBKEY" --match-hash "$AUTHOR_MATCH_HASH" --peer-pubkey "$AUTHOR_PUBKEY" --greeting "$AUTHOR_GREETING")
          gh pr comment "$RECIP_NUMBER" --repo "$REPO" --body "$RECIP_NOTIFICATION"
```

- [ ] **Step 3: Verify matchcrypt compiles**

Run: `go build ./cmd/matchcrypt/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/matchcrypt/main.go templates/actions/pool-interest.yml
git commit -m "feat: match notification delivers peer pubkey + match_hash"
```

---

### Task 12: Update matches screen — MatchItem with MatchHash + PubKey

**Files:**
- Modify: `internal/cli/tui/screens/matches.go`

- [ ] **Step 1: Update MatchItem and decryptMatchNotification**

```go
type MatchItem struct {
    MatchHash string
    Greeting  string
    PubKey    ed25519.PublicKey
}
```

Update `decryptMatchNotification`:
```go
func decryptMatchNotification(body string, priv ed25519.PrivateKey) (*MatchItem, error) {
    blobBytes, err := base64.StdEncoding.DecodeString(body)
    if err != nil {
        return nil, err
    }
    plaintext, err := crypto.Decrypt(priv, blobBytes)
    if err != nil {
        return nil, err
    }
    var data struct {
        MatchedMatchHash string `json:"matched_match_hash"`
        Greeting         string `json:"greeting"`
        PubKey           string `json:"pubkey"`
    }
    if err := json.Unmarshal(plaintext, &data); err != nil {
        return nil, err
    }
    if data.MatchedMatchHash == "" {
        return nil, fmt.Errorf("missing match_hash")
    }
    pubBytes, err := hex.DecodeString(data.PubKey)
    if err != nil || len(pubBytes) != ed25519.PublicKeySize {
        return nil, fmt.Errorf("invalid pubkey")
    }
    return &MatchItem{
        MatchHash: data.MatchedMatchHash,
        Greeting:  data.Greeting,
        PubKey:    ed25519.PublicKey(pubBytes),
    }, nil
}
```

Update `MatchChatMsg`:
```go
type MatchChatMsg struct {
    MatchHash string
    PubKey    ed25519.PublicKey
}
```

Update the View method to handle optional greeting:
```go
for i, m := range s.matches {
    cursor := "  "
    nameStyle := theme.TextStyle
    if i == s.cursor {
        cursor = theme.BrandStyle.Render("▸ ")
        nameStyle = theme.BoldStyle
    }
    line := nameStyle.Render(m.MatchHash[:16])
    if m.Greeting != "" {
        greeting := m.Greeting
        if len(greeting) > 40 {
            greeting = greeting[:37] + "..."
        }
        line += "  " + theme.DimStyle.Render("\""+greeting+"\"")
    }
    body += fmt.Sprintf("%s%s\n", cursor, line)
}
```

Update the enter key handler to pass PubKey:
```go
case "enter":
    if s.cursor < len(s.matches) {
        return s, func() tea.Msg {
            return MatchChatMsg{
                MatchHash: s.matches[s.cursor].MatchHash,
                PubKey:    s.matches[s.cursor].PubKey,
            }
        }
    }
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/cli/tui/screens/`
Expected: May fail if chat.go still imports protocol — will be fixed next.

- [ ] **Step 3: Commit**

```bash
git add internal/cli/tui/screens/matches.go
git commit -m "refactor: MatchItem uses match_hash + pubkey, optional greeting"
```

---

### Task 13: Update TUI chat screen — match_hash + remove protocol

**Files:**
- Modify: `internal/cli/tui/screens/chat.go`

- [ ] **Step 1: Update chat.go screen**

Key changes:
- Remove `protocol` import
- `ChatIncomingMsg` carries `SenderMatchHash string` and `Plaintext []byte` instead of `protocol.Message`
- `ConnectChatCmd` takes `targetMatchHash string` and `peerPub ed25519.PublicKey`
- Uses `IDHash` in client config (computed from config)
- Calls `client.SetPeerKey` before connecting
- `client.OnMessage` callback signature changed
- `client.OnControl` for text frame status

```go
package screens

import (
    "context"
    "crypto/ed25519"
    "fmt"
    "time"

    "github.com/charmbracelet/bubbles/viewport"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/vutran1710/dating-dev/internal/cli/config"
    relayclient "github.com/vutran1710/dating-dev/internal/cli/relay"
    "github.com/vutran1710/dating-dev/internal/cli/tui/components"
    "github.com/vutran1710/dating-dev/internal/cli/tui/theme"
    "github.com/vutran1710/dating-dev/internal/crypto"
)

type ChatMessage struct {
    Sender string
    Body   string
    Time   string
    IsMe   bool
}

type ChatConnectedMsg struct {
    Client *relayclient.Client
    Err    error
}

type ChatIncomingMsg struct {
    SenderMatchHash string
    Plaintext       []byte
}

type ChatControlMsg struct {
    Message string
}

type ChatScreen struct {
    TargetID  string
    Messages  []ChatMessage
    Viewport  viewport.Model
    Width     int
    Height    int
    Ready     bool
    client    *relayclient.Client
    connected bool
    err       error
}

func NewChatScreen(targetID string, width, height int) ChatScreen {
    vp := viewport.New(width, height-6)
    vp.Style = lipgloss.NewStyle().Padding(0, 2)
    return ChatScreen{
        TargetID: targetID,
        Viewport: vp,
        Width:    width,
        Height:   height,
    }
}

func ConnectChatCmd(targetMatchHash string, peerPub ed25519.PublicKey) tea.Cmd {
    return func() tea.Msg {
        cfg, err := config.Load()
        if err != nil {
            return ChatConnectedMsg{Err: err}
        }
        pool := cfg.ActivePool()
        if pool == nil || pool.RelayURL == "" {
            return ChatConnectedMsg{Err: fmt.Errorf("no relay configured")}
        }

        pub, priv, err := crypto.LoadKeyPair(config.KeysDir())
        if err != nil {
            return ChatConnectedMsg{Err: err}
        }

        client := relayclient.NewClient(relayclient.Config{
            RelayURL:  pool.RelayURL,
            PoolURL:   pool.Repo,
            IDHash:    pool.IDHash,
            MatchHash: pool.MatchHash,
            Pub:       pub,
            Priv:      priv,
        })

        client.SetPeerKey(targetMatchHash, peerPub)

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        if err := client.Connect(ctx); err != nil {
            return ChatConnectedMsg{Err: err}
        }

        return ChatConnectedMsg{Client: client}
    }
}

func (s ChatScreen) Update(msg tea.Msg) (ChatScreen, tea.Cmd) {
    switch msg := msg.(type) {
    case ChatConnectedMsg:
        if msg.Err != nil {
            s.err = msg.Err
            return s, nil
        }
        s.client = msg.Client
        s.connected = true
        s.Ready = true
        return s, s.listenForMessages()

    case ChatIncomingMsg:
        sender := msg.SenderMatchHash
        if len(sender) > 12 {
            sender = sender[:12] + "..."
        }
        s.Messages = append(s.Messages, ChatMessage{
            Sender: sender,
            Body:   string(msg.Plaintext),
            Time:   time.Now().Format("15:04"),
            IsMe:   false,
        })
        s.Viewport.SetContent(s.renderMessages())
        s.Viewport.GotoBottom()
        return s, s.listenForMessages()

    case ChatControlMsg:
        s.Messages = append(s.Messages, ChatMessage{
            Body: "[relay] " + msg.Message,
            Time: time.Now().Format("15:04"),
        })
        s.Viewport.SetContent(s.renderMessages())
        s.Viewport.GotoBottom()
        return s, nil

    case components.SubmitMsg:
        if msg.IsCommand && msg.Value == "/exit" {
            if s.client != nil {
                s.client.Close()
            }
            return s, nil
        }
        if s.connected && msg.Value != "" {
            err := s.client.SendMessage(s.TargetID, msg.Value)
            if err != nil {
                s.Messages = append(s.Messages, ChatMessage{
                    Body: "(send failed: " + err.Error() + ")",
                    Time: time.Now().Format("15:04"),
                })
            } else {
                s.Messages = append(s.Messages, ChatMessage{
                    Sender: "you",
                    Body:   msg.Value,
                    Time:   time.Now().Format("15:04"),
                    IsMe:   true,
                })
            }
            s.Viewport.SetContent(s.renderMessages())
            s.Viewport.GotoBottom()
        }

    case tea.WindowSizeMsg:
        s.Width = msg.Width
        s.Height = msg.Height
        s.Viewport.Width = msg.Width
        s.Viewport.Height = msg.Height - 6
    }

    var cmd tea.Cmd
    s.Viewport, cmd = s.Viewport.Update(msg)
    return s, cmd
}

func (s ChatScreen) listenForMessages() tea.Cmd {
    if s.client == nil {
        return nil
    }
    msgCh := make(chan ChatIncomingMsg, 1)
    ctrlCh := make(chan ChatControlMsg, 1)

    s.client.OnMessage(func(senderMatchHash string, plaintext []byte) {
        msgCh <- ChatIncomingMsg{SenderMatchHash: senderMatchHash, Plaintext: plaintext}
    })
    s.client.OnControl(func(msg string) {
        ctrlCh <- ChatControlMsg{Message: msg}
    })

    return func() tea.Msg {
        select {
        case m := <-msgCh:
            return m
        case c := <-ctrlCh:
            return c
        }
    }
}

func (s ChatScreen) View() string {
    if s.err != nil {
        body := theme.RedStyle.Render("Connection failed: " + s.err.Error())
        return components.ScreenLayout("Chat", components.DimHints(s.TargetID), body)
    }
    if !s.connected {
        return components.ScreenLayout("Chat", components.DimHints(s.TargetID),
            theme.DimStyle.Render("Connecting to relay..."))
    }
    separator := lipgloss.NewStyle().
        Width(s.Width).
        Foreground(theme.Border).
        Render(components.Repeat("─", s.Width))
    body := separator + "\n" + s.Viewport.View()
    return components.ScreenLayout("Chat", components.DimHints(s.TargetID), body)
}

func (s ChatScreen) renderMessages() string {
    if len(s.Messages) == 0 {
        return theme.DimStyle.Render("  No messages yet. Say hello!")
    }
    out := ""
    for _, m := range s.Messages {
        ts := theme.DimStyle.Render("[" + m.Time + "]")
        var sender string
        if m.IsMe {
            sender = theme.BrandStyle.Render("you")
        } else if m.Sender != "" {
            sender = theme.AccentStyle.Render(m.Sender)
        } else {
            sender = theme.DimStyle.Render("system")
        }
        out += fmt.Sprintf("%s %s: %s\n", ts, sender, m.Body)
    }
    return out
}

func (s ChatScreen) HelpBindings() []components.KeyBind {
    return []components.KeyBind{
        {Key: "enter", Desc: "send"},
        {Key: "/exit", Desc: "leave"},
    }
}
```

- [ ] **Step 2: Fix callers — find where ConnectChatCmd and MatchChatMsg are used in app.go**

Run: `grep -n "ConnectChatCmd\|MatchChatMsg" internal/cli/tui/*.go internal/cli/tui/**/*.go`

Update `app.go` (or wherever `MatchChatMsg` is handled) to pass `msg.PubKey` to `ConnectChatCmd`.

- [ ] **Step 3: Verify the full TUI compiles**

Run: `go build ./internal/cli/...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/cli/tui/screens/chat.go
git commit -m "refactor: TUI chat screen — binary protocol, match_hash, peer pubkey"
```

---

### Task 14: Simplify cmd/relay/main.go

**Files:**
- Modify: `cmd/relay/main.go`

- [ ] **Step 1: Update main.go — remove DB_PATH**

```go
package main

import (
    "log"
    "os"

    "github.com/vutran1710/dating-dev/internal/relay"
)

func main() {
    port := os.Getenv("PORT")
    if port == "" {
        port = "8081"
    }

    poolURL := os.Getenv("POOL_URL")
    if poolURL == "" {
        log.Fatal("POOL_URL is required (e.g. owner/pool-name)")
    }

    salt := os.Getenv("POOL_SALT")
    if salt == "" {
        log.Fatal("POOL_SALT is required")
    }

    srv := relay.NewServer(relay.ServerConfig{
        PoolURL: poolURL,
        Salt:    salt,
    })

    if err := srv.ListenAndServe(relay.Addr(port)); err != nil {
        log.Fatal(err)
    }
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./cmd/relay/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add cmd/relay/main.go
git commit -m "simplify: relay main — no DB_PATH, stateless"
```

---

### Task 15: Fix remaining compile errors and run full build

**Files:**
- Various — any files still importing `protocol` or referencing old APIs

- [ ] **Step 1: Find broken imports**

Run: `go build ./...` and fix any remaining compile errors.

Common issues:
- `internal/cli/tui/app.go` — `MatchChatMsg` now has `PubKey` field, `ConnectChatCmd` takes extra arg
- Any file still importing `internal/protocol`
- Any reference to `client.BinHash()`, `client.SendMessagePlain()`, `client.AckMessage()`

- [ ] **Step 2: Run full build**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "fix: resolve all compile errors from protocol removal"
```

---

### Task 16: Rewrite relay e2e tests

**Files:**
- Modify: `internal/relay/relay_e2e_test.go`

- [ ] **Step 1: Write comprehensive test suite**

Test categories (see spec for details):
1. **Auth tests**: valid chain + sig, wrong sig, mismatched chain, missing params, expired sig
2. **Routing tests**: both online → binary frame routed, not matched → text frame error, sender match_hash prepended
3. **Queue tests**: target offline → queued + "queued" text frame, target connects → flushed, cap enforced (21st → "queue full")
4. **Session tests**: reconnect replaces old session (old conn closed), multiple concurrent users
5. **Cache tests**: pubkey cached (one fetch), match check cached with TTL, 404 negative cached, 5xx not cached
6. **Health test**: `/health` returns `{status, online, queued}`
7. **Encrypted roundtrip**: two clients with pre-shared pubkeys, send encrypted → decrypt → verify plaintext

Test setup helper:
```go
func setupTestRelay(t *testing.T) (*Server, *httptest.Server, string) {
    // Create httptest server that serves .bin files and match files
    // Return relay server, test HTTP server, and salt
}
```

Each test should:
- Use `httptest.NewServer` for mock GitHub
- Create relay server with `ServerConfig{RawBaseURL: mockServer.URL + "/owner/pool"}`
- Pre-populate caches via `srv.GitHubCache().SetPubkey()` and `srv.GitHubCache().SetMatch()` where appropriate
- Connect via real WebSocket (`gorilla/websocket.Dialer`)

- [ ] **Step 2: Run tests**

Run: `make test -- -run TestRelay -v ./internal/relay/ -race`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/relay/relay_e2e_test.go
git commit -m "test: comprehensive relay e2e tests — auth, routing, queue, cache"
```

---

### Task 17: Run full test suite and fix any failures

- [ ] **Step 1: Run all tests**

Run: `make test`
Expected: PASS (all packages)

- [ ] **Step 2: Fix any failures**

Common issues:
- Old TOTP tests calling `TOTPSign(binHash, matchHash, priv)` with old signature
- Config tests referencing `BinHash`
- Missing `go.sum` entries if `modernc.org/sqlite` is no longer needed — run `go mod tidy`

- [ ] **Step 3: Clean up go.mod**

Run: `go mod tidy` to remove unused dependencies (SQLite driver, msgpack).

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "fix: all tests pass, go mod tidy"
```

---

### Task 18: E2E test with real test pool (CLI)

- [ ] **Step 1: Set up two DATING_HOME dirs**

```bash
export DATING_HOME_A=$(mktemp -d)
export DATING_HOME_B=$(mktemp -d)
```

- [ ] **Step 2: Register two managed accounts**

Using the test pool repo, register two users with separate homes. Each will need key generation, profile creation, and `pool join`.

- [ ] **Step 3: Express mutual interest and wait for match**

Both users create interest PRs targeting each other's match_hash. Wait for the pool-interest Action to detect the mutual match and post encrypted notifications.

- [ ] **Step 4: Start relay and test chat**

Start relay with test pool salt. User A connects with `dating chat <B_match_hash>`, User B connects with `dating chat <A_match_hash>`. Exchange messages and verify decryption.

- [ ] **Step 5: Test offline queue**

Disconnect B, send from A, reconnect B, verify queued message arrives.

---

### Task 19: E2E test with TUI

- [ ] **Step 1: Launch two TUI instances**

```bash
DATING_HOME=$DATING_HOME_A dating tui &
DATING_HOME=$DATING_HOME_B dating tui &
```

- [ ] **Step 2: Navigate and test**

- Navigate to Matches screen → verify peer appears
- Select match → Chat screen
- Send messages both directions → verify display
- Close B, send from A, reopen B → verify queued message

---

### Task 20: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update CLAUDE.md to reflect stateless relay**

Key changes:
- Tech Stack: change "MessagePack over WebSocket" → "Binary frames over WebSocket"
- Architecture: remove `internal/protocol/` from tree
- Relay Server section: update to stateless design (no SQLite, no Store, hub keyed by match_hash, binary frames)
- Remove `DB_PATH` from env vars
- Update "Planned: Stateless Relay Refactor" section → mark as implemented
- Remove references to key exchange protocol

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for stateless relay"
```

---

### Task 21: Deploy to Railway and production E2E

- [ ] **Step 1: Update Dockerfile if needed**

Remove any SQLite-related build steps. The binary should be simpler now.

- [ ] **Step 2: Deploy**

Push to Railway. Set env vars: `POOL_URL`, `POOL_SALT`, `PORT`.

- [ ] **Step 3: Test with real relay**

Repeat E2E test (Task 17) against the production Railway relay URL.

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "deploy: stateless relay on Railway"
```
