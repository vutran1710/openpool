# E2E Encryption Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Encrypt all chat message bodies end-to-end so the relay is zero-knowledge, with a unique key per match pair.

**Architecture:** Static ECDH (X25519 from ed25519 keys) + HKDF-SHA256 key derivation + NaCl secretbox encryption. Client requests peer pubkey from relay via new `key_request`/`key_response` frames, derives a per-conversation symmetric key, encrypts message body before sending. Relay forwards encrypted body opaquely.

**Tech Stack:** Go, `golang.org/x/crypto/curve25519`, `golang.org/x/crypto/hkdf`, `golang.org/x/crypto/nacl/secretbox`, `filippo.io/edwards25519`, `encoding/base64`

**Spec:** `docs/superpowers/specs/2026-03-17-e2e-encryption-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/crypto/conversation.go` | Create | SharedSecret, DeriveConversationKey, SealMessage, OpenMessage |
| `internal/crypto/conversation_test.go` | Create | Unit tests for all conversation crypto functions |
| `internal/protocol/types.go` | Modify | Add KeyRequest, KeyResponse structs + Encrypted field on Message/QueuedMessage |
| `internal/protocol/codec.go` | Modify | Add key_request/key_response to DecodeFrame switch |
| `internal/protocol/codec_test.go` | Modify | Add encode/decode tests for new frame types |
| `internal/relay/store.go` | Modify | Add PubKeyByHash method |
| `internal/relay/session.go` | Modify | Add handleKeyRequest, forward Encrypted field in handleMessage |
| `internal/cli/relay/client.go` | Modify | Add OnRawMessage, key caching, RequestKey, encrypt in SendMessage, decrypt in readLoop |
| `internal/relay/relay_e2e_test.go` | Modify | Add E2E encryption tests + update existing tests for encrypted SendMessage |
| `internal/cli/relay/client_test.go` | Modify | Update unit tests for encrypted SendMessage |
| `internal/cli/relay/client_integration_test.go` | Modify | Update integration tests for encrypted SendMessage |

---

## Chunk 1: Crypto Primitives

### Task 1: Conversation crypto functions

**Files:**
- Create: `internal/crypto/conversation.go`
- Create: `internal/crypto/conversation_test.go`

- [ ] **Step 1: Write failing tests for SharedSecret**

In `internal/crypto/conversation_test.go`:

```go
package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

func genKeys(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func TestSharedSecret_BothSidesMatch(t *testing.T) {
	pubA, privA := genKeys(t)
	pubB, privB := genKeys(t)

	secretAB, err := SharedSecret(privA, pubB)
	if err != nil {
		t.Fatalf("A→B: %v", err)
	}
	secretBA, err := SharedSecret(privB, pubA)
	if err != nil {
		t.Fatalf("B→A: %v", err)
	}

	if len(secretAB) != 32 {
		t.Errorf("secret length = %d, want 32", len(secretAB))
	}
	for i := range secretAB {
		if secretAB[i] != secretBA[i] {
			t.Fatal("shared secrets differ")
		}
	}
}

func TestSharedSecret_DifferentPairs_DifferentSecrets(t *testing.T) {
	_, privA := genKeys(t)
	pubB, _ := genKeys(t)
	pubC, _ := genKeys(t)

	ab, _ := SharedSecret(privA, pubB)
	ac, _ := SharedSecret(privA, pubC)

	for i := range ab {
		if ab[i] != ac[i] {
			return // different — pass
		}
	}
	t.Fatal("different peers should produce different secrets")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `DATING_HOME=$(mktemp -d) go test ./internal/crypto/ -run TestSharedSecret -v`
Expected: FAIL — `SharedSecret` undefined

- [ ] **Step 3: Implement SharedSecret**

In `internal/crypto/conversation.go` (imports added incrementally — only what this function needs):

```go
package crypto

import (
	"crypto/ed25519"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

// SharedSecret computes X25519 ECDH between own ed25519 private key
// and peer's ed25519 public key. Returns 32-byte shared secret.
func SharedSecret(myPriv ed25519.PrivateKey, theirPub ed25519.PublicKey) ([]byte, error) {
	myCurve := ed25519PrivToCurve25519(myPriv)
	theirCurve, err := ed25519PubToCurve25519(theirPub)
	if err != nil {
		return nil, fmt.Errorf("converting peer pubkey: %w", err)
	}
	shared, err := curve25519.X25519(myCurve[:], theirCurve[:])
	if err != nil {
		return nil, fmt.Errorf("X25519: %w", err)
	}
	return shared, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `DATING_HOME=$(mktemp -d) go test ./internal/crypto/ -run TestSharedSecret -v`
Expected: PASS

- [ ] **Step 5: Write failing tests for DeriveConversationKey**

Append to `internal/crypto/conversation_test.go`:

```go
func TestDeriveConversationKey_UniquePerPair(t *testing.T) {
	_, privA := genKeys(t)
	pubB, _ := genKeys(t)
	pubC, _ := genKeys(t)

	abSecret, _ := SharedSecret(privA, pubB)
	acSecret, _ := SharedSecret(privA, pubC)

	keyAB, err := DeriveConversationKey(abSecret, "hashA", "hashB", "owner/pool")
	if err != nil {
		t.Fatal(err)
	}
	keyAC, err := DeriveConversationKey(acSecret, "hashA", "hashC", "owner/pool")
	if err != nil {
		t.Fatal(err)
	}

	for i := range keyAB {
		if keyAB[i] != keyAC[i] {
			return
		}
	}
	t.Fatal("different pairs should produce different keys")
}

func TestDeriveConversationKey_UniquePerPool(t *testing.T) {
	_, privA := genKeys(t)
	pubB, _ := genKeys(t)

	secret, _ := SharedSecret(privA, pubB)

	key1, _ := DeriveConversationKey(secret, "hashA", "hashB", "owner/pool-1")
	key2, _ := DeriveConversationKey(secret, "hashA", "hashB", "owner/pool-2")

	for i := range key1 {
		if key1[i] != key2[i] {
			return
		}
	}
	t.Fatal("different pools should produce different keys")
}

func TestDeriveConversationKey_SortedHashes(t *testing.T) {
	_, privA := genKeys(t)
	pubB, _ := genKeys(t)
	secret, _ := SharedSecret(privA, pubB)

	key1, _ := DeriveConversationKey(secret, "aaa", "zzz", "pool")
	key2, _ := DeriveConversationKey(secret, "zzz", "aaa", "pool")

	for i := range key1 {
		if key1[i] != key2[i] {
			t.Fatal("hash order should not matter (sorted internally)")
		}
	}
}

func TestDeriveConversationKey_InvalidInput(t *testing.T) {
	_, err := DeriveConversationKey([]byte("short"), "a", "b", "pool")
	if err == nil {
		t.Fatal("expected error for short shared secret")
	}
	_, err = DeriveConversationKey(make([]byte, 32), "", "b", "pool")
	if err == nil {
		t.Fatal("expected error for empty hashA")
	}
	_, err = DeriveConversationKey(make([]byte, 32), "a", "", "pool")
	if err == nil {
		t.Fatal("expected error for empty hashB")
	}
	_, err = DeriveConversationKey(make([]byte, 32), "a", "b", "")
	if err == nil {
		t.Fatal("expected error for empty poolURL")
	}
}
```

- [ ] **Step 6: Run tests to verify they fail**

Run: `DATING_HOME=$(mktemp -d) go test ./internal/crypto/ -run TestDeriveConversationKey -v`
Expected: FAIL — `DeriveConversationKey` undefined

- [ ] **Step 7: Implement DeriveConversationKey**

Add to `internal/crypto/conversation.go` (also add `"crypto/sha256"`, `"io"`, and `"golang.org/x/crypto/hkdf"` to imports):

```go
// DeriveConversationKey derives a 32-byte symmetric key from a shared
// secret using HKDF-SHA256. The info string includes sorted hashes and
// pool URL for domain separation. Salt is nil (HKDF uses zero-filled buffer).
func DeriveConversationKey(shared []byte, hashA, hashB, poolURL string) ([]byte, error) {
	if len(shared) != 32 {
		return nil, fmt.Errorf("shared secret must be 32 bytes, got %d", len(shared))
	}
	if hashA == "" || hashB == "" || poolURL == "" {
		return nil, fmt.Errorf("hashA, hashB, and poolURL must be non-empty")
	}

	// Sort hashes so both sides produce the same info string
	if hashA > hashB {
		hashA, hashB = hashB, hashA
	}
	info := "dating-e2e:" + hashA + ":" + hashB + ":" + poolURL

	hk := hkdf.New(sha256.New, shared, nil, []byte(info))
	key := make([]byte, 32)
	if _, err := io.ReadFull(hk, key); err != nil {
		return nil, fmt.Errorf("HKDF expand: %w", err)
	}
	return key, nil
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `DATING_HOME=$(mktemp -d) go test ./internal/crypto/ -run TestDeriveConversationKey -v`
Expected: PASS

- [ ] **Step 9: Write failing tests for SealMessage/OpenMessage**

Append to `internal/crypto/conversation_test.go` (add `"encoding/base64"` to imports):

```go
func TestSealMessage_OpenMessage_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	sealed, err := SealMessage(key, "hello world")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}

	opened, err := OpenMessage(key, sealed)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	if opened != "hello world" {
		t.Errorf("got %q, want 'hello world'", opened)
	}
}

func TestSealMessage_UniqueNonce(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	s1, _ := SealMessage(key, "same message")
	s2, _ := SealMessage(key, "same message")

	if s1 == s2 {
		t.Fatal("two seals of same plaintext should produce different ciphertext (unique nonce)")
	}
}

func TestOpenMessage_WrongKey_Fails(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	rand.Read(key1)
	rand.Read(key2)

	sealed, _ := SealMessage(key1, "secret")
	_, err := OpenMessage(key2, sealed)
	if err == nil {
		t.Fatal("expected error with wrong key")
	}
}

func TestOpenMessage_Tampered_Fails(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	sealed, _ := SealMessage(key, "secret")

	// Decode, flip a byte, re-encode
	raw, _ := base64.RawStdEncoding.DecodeString(sealed)
	raw[len(raw)-1] ^= 0xff
	tampered := base64.RawStdEncoding.EncodeToString(raw)

	_, err := OpenMessage(key, tampered)
	if err == nil {
		t.Fatal("expected error for tampered ciphertext")
	}
}

func TestOpenMessage_BadVersion_Fails(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	sealed, _ := SealMessage(key, "secret")
	raw, _ := base64.RawStdEncoding.DecodeString(sealed)
	raw[0] = 0x99 // wrong version
	bad := base64.RawStdEncoding.EncodeToString(raw)

	_, err := OpenMessage(key, bad)
	if err == nil {
		t.Fatal("expected error for bad version byte")
	}
}

func TestSealMessage_EmptyPlaintext(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	sealed, err := SealMessage(key, "")
	if err != nil {
		t.Fatalf("seal empty: %v", err)
	}
	opened, err := OpenMessage(key, sealed)
	if err != nil {
		t.Fatalf("open empty: %v", err)
	}
	if opened != "" {
		t.Errorf("got %q, want empty", opened)
	}
}
```

- [ ] **Step 10: Run tests to verify they fail**

Run: `DATING_HOME=$(mktemp -d) go test ./internal/crypto/ -run "TestSealMessage|TestOpenMessage" -v`
Expected: FAIL — `SealMessage`/`OpenMessage` undefined

- [ ] **Step 11: Implement SealMessage and OpenMessage**

Add to `internal/crypto/conversation.go` (also add `"crypto/rand"`, `"encoding/base64"`, and `"golang.org/x/crypto/nacl/secretbox"` to imports):

```go
const e2eVersion byte = 0x01

// SealMessage encrypts plaintext with a 32-byte conversation key.
// Returns base64-encoded (version_byte || nonce || ciphertext).
func SealMessage(key []byte, plaintext string) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("key must be 32 bytes")
	}

	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	var k [32]byte
	copy(k[:], key)

	sealed := secretbox.Seal(nil, []byte(plaintext), &nonce, &k)

	// version(1) + nonce(24) + ciphertext
	buf := make([]byte, 0, 1+24+len(sealed))
	buf = append(buf, e2eVersion)
	buf = append(buf, nonce[:]...)
	buf = append(buf, sealed...)

	return base64.RawStdEncoding.EncodeToString(buf), nil
}

// OpenMessage decrypts a base64-encoded (version_byte || nonce || ciphertext)
// with a 32-byte conversation key. Returns plaintext string.
func OpenMessage(key []byte, encoded string) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("key must be 32 bytes")
	}

	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	if len(raw) < 1+24+secretbox.Overhead {
		return "", fmt.Errorf("ciphertext too short")
	}

	if raw[0] != e2eVersion {
		return "", fmt.Errorf("unsupported encryption version: 0x%02x", raw[0])
	}

	var nonce [24]byte
	copy(nonce[:], raw[1:25])
	ciphertext := raw[25:]

	var k [32]byte
	copy(k[:], key)

	plaintext, ok := secretbox.Open(nil, ciphertext, &nonce, &k)
	if !ok {
		return "", fmt.Errorf("decryption failed (wrong key or tampered)")
	}

	return string(plaintext), nil
}
```

- [ ] **Step 12: Run all crypto tests to verify they pass**

Run: `DATING_HOME=$(mktemp -d) go test ./internal/crypto/ -v`
Expected: ALL PASS

- [ ] **Step 13: Commit**

```bash
git add internal/crypto/conversation.go internal/crypto/conversation_test.go
git commit -m "feat: add conversation crypto (SharedSecret, DeriveConversationKey, SealMessage, OpenMessage)"
```

---

## Chunk 2: Protocol Types + Codec

### Task 2: Add KeyRequest/KeyResponse frames and Encrypted field

**Files:**
- Modify: `internal/protocol/types.go:7-18` (constants), `:60-70` (Message), `:136-146` (QueuedMessage)
- Modify: `internal/protocol/codec.go:39-72` (DecodeFrame switch)
- Modify: `internal/protocol/codec_test.go`

- [ ] **Step 1: Add new types and constants to types.go**

In `internal/protocol/types.go`, add to the frame types const block after `TypeError`:

```go
TypeKeyRequest  = "key_request"
TypeKeyResponse = "key_response"
```

Add new structs after the `Ack` struct:

```go
// KeyRequest asks the relay for a matched user's public key.
type KeyRequest struct {
	Type       string `msgpack:"type"`
	TargetHash string `msgpack:"target_hash"`
}

// KeyResponse returns a user's ed25519 public key.
type KeyResponse struct {
	Type       string `msgpack:"type"`
	TargetHash string `msgpack:"target_hash"`
	PubKey     string `msgpack:"pubkey"`
}
```

Add `Encrypted` field to `Message` struct:

```go
Encrypted  bool   `msgpack:"encrypted,omitempty"`
```

Add `Encrypted` field to `QueuedMessage` struct:

```go
Encrypted  bool   `msgpack:"encrypted"`
```

- [ ] **Step 2: Add DecodeFrame cases in codec.go**

In `internal/protocol/codec.go`, add two cases before the `default:` in DecodeFrame:

```go
case TypeKeyRequest:
	var f KeyRequest
	return &f, Decode(data, &f)
case TypeKeyResponse:
	var f KeyResponse
	return &f, Decode(data, &f)
```

- [ ] **Step 3: Add tests for new frame types in codec_test.go**

Add to the `frames` slice in `TestDecodeFrame_AllTypes`:

```go
&KeyRequest{Type: TypeKeyRequest, TargetHash: "abc"},
&KeyResponse{Type: TypeKeyResponse, TargetHash: "abc", PubKey: "deadbeef"},
```

Add a new test:

```go
func TestEncodeDecode_KeyRequest(t *testing.T) {
	orig := KeyRequest{Type: TypeKeyRequest, TargetHash: "abc123"}
	data, err := Encode(orig)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var decoded KeyRequest
	if err := Decode(data, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.TargetHash != "abc123" {
		t.Errorf("target_hash = %q, want abc123", decoded.TargetHash)
	}
}

func TestEncodeDecode_KeyResponse(t *testing.T) {
	orig := KeyResponse{Type: TypeKeyResponse, TargetHash: "abc", PubKey: "deadbeef"}
	data, _ := Encode(orig)
	var decoded KeyResponse
	Decode(data, &decoded)
	if decoded.PubKey != "deadbeef" {
		t.Errorf("pubkey = %q, want deadbeef", decoded.PubKey)
	}
}

func TestEncodeDecode_MessageEncrypted(t *testing.T) {
	orig := Message{Type: TypeMsg, Body: "cipher", Encrypted: true}
	data, _ := Encode(orig)
	var decoded Message
	Decode(data, &decoded)
	if !decoded.Encrypted {
		t.Error("expected Encrypted=true")
	}
}
```

- [ ] **Step 4: Run protocol tests**

Run: `DATING_HOME=$(mktemp -d) go test ./internal/protocol/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/protocol/types.go internal/protocol/codec.go internal/protocol/codec_test.go
git commit -m "feat: add key_request/key_response frames and Encrypted field to Message"
```

---

## Chunk 3: Relay Server — KeyRequest Handler

### Task 3: Add PubKeyByHash to store and handleKeyRequest to session

**Files:**
- Modify: `internal/relay/store.go`
- Modify: `internal/relay/session.go:65-82` (handleFrame switch), `:198-244` (handleMessage)

- [ ] **Step 1: Add PubKeyByHash to store.go**

Add to `internal/relay/store.go`:

```go
// PubKeyByHash returns the public key for a user identified by hash_id.
func (s *Store) PubKeyByHash(hashID string) ed25519.PublicKey {
	entry := s.LookupByHash(hashID)
	if entry == nil {
		return nil
	}
	return entry.PubKey
}
```

- [ ] **Step 2: Add handleKeyRequest to session.go**

Add to `internal/relay/session.go`:

```go
func (s *Session) handleKeyRequest(req *protocol.KeyRequest) {
	if !s.authed {
		s.sendError(protocol.ErrTokenInvalid, "not authenticated")
		return
	}

	if req.TargetHash == "" {
		s.sendError(protocol.ErrInternal, "missing target_hash")
		return
	}

	// Verify requester is matched with target
	if !s.server.store.IsMatched(s.hashID, req.TargetHash) {
		s.sendError(protocol.ErrNotMatched, "not matched with target")
		return
	}

	pubkey := s.server.store.PubKeyByHash(req.TargetHash)
	if pubkey == nil {
		s.sendError(protocol.ErrUserNotFound, "target not found")
		return
	}

	s.sendFrame(protocol.KeyResponse{
		Type:       protocol.TypeKeyResponse,
		TargetHash: req.TargetHash,
		PubKey:     hex.EncodeToString(pubkey),
	})
}
```

Add `"encoding/hex"` to imports if not already present.

- [ ] **Step 3: Route KeyRequest in handleFrame**

In `internal/relay/session.go`, add to the `handleFrame` switch before `default`:

```go
case *protocol.KeyRequest:
	s.handleKeyRequest(f)
```

- [ ] **Step 4: Forward Encrypted field in handleMessage**

In `internal/relay/session.go`, in `handleMessage`, add to the `outMsg` construction:

```go
Encrypted:  msg.Encrypted,
```

So the outMsg block becomes:

```go
outMsg := protocol.Message{
	Type:       protocol.TypeMsg,
	MsgID:      msgID,
	SourceHash: msg.SourceHash,
	TargetHash: msg.TargetHash,
	PoolURL:    msg.PoolURL,
	Body:       msg.Body,
	Ts:         msg.Ts,
	Encrypted:  msg.Encrypted,
}
```

- [ ] **Step 5: Build to verify compilation**

Run: `go build ./internal/relay/`
Expected: success (no output)

- [ ] **Step 6: Commit**

```bash
git add internal/relay/store.go internal/relay/session.go
git commit -m "feat: add handleKeyRequest with match verification, forward Encrypted field"
```

---

## Chunk 4: Client — Encryption Integration

### Task 4: Add key caching, RequestKey, encrypt/decrypt to relay client

**Files:**
- Modify: `internal/cli/relay/client.go`

- [ ] **Step 1: Add new fields and imports to Client**

In `internal/cli/relay/client.go`, update the `Client` struct to add:

```go
keys     map[string][]byte                // target_hash → conversation_key
keyReqs  map[string]chan ed25519.PublicKey // pending key request response channels
```

Update `NewClient` to initialize them:

```go
keys:    make(map[string][]byte),
keyReqs: make(map[string]chan ed25519.PublicKey),
```

- [ ] **Step 2: Implement RequestKey**

Add to `internal/cli/relay/client.go`:

```go
// RequestKey requests and returns a peer's ed25519 public key from the relay.
// Blocks until the response arrives or ctx is cancelled.
func (c *Client) RequestKey(ctx context.Context, targetHash string) (ed25519.PublicKey, error) {
	c.mu.Lock()
	// Dedup: reuse existing inflight request
	ch, exists := c.keyReqs[targetHash]
	if !exists {
		ch = make(chan ed25519.PublicKey, 1)
		c.keyReqs[targetHash] = ch
	}
	c.mu.Unlock()

	// Only send the frame if we created the channel
	if !exists {
		if err := c.send(protocol.KeyRequest{
			Type:       protocol.TypeKeyRequest,
			TargetHash: targetHash,
		}); err != nil {
			c.mu.Lock()
			delete(c.keyReqs, targetHash)
			c.mu.Unlock()
			return nil, fmt.Errorf("sending key request: %w", err)
		}
	}

	select {
	case pub := <-ch:
		return pub, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.keyReqs, targetHash)
		c.mu.Unlock()
		return nil, ctx.Err()
	}
}
```

- [ ] **Step 3: Add getOrDeriveKey helper**

```go
// getOrDeriveKey returns the cached conversation key or derives a new one.
func (c *Client) getOrDeriveKey(ctx context.Context, targetHash string) ([]byte, error) {
	c.mu.Lock()
	key, ok := c.keys[targetHash]
	c.mu.Unlock()
	if ok {
		return key, nil
	}

	peerPub, err := c.RequestKey(ctx, targetHash)
	if err != nil {
		return nil, fmt.Errorf("requesting peer key: %w", err)
	}

	shared, err := crypto.SharedSecret(c.priv, peerPub)
	if err != nil {
		return nil, fmt.Errorf("computing shared secret: %w", err)
	}

	key, err = crypto.DeriveConversationKey(shared, c.hashID, targetHash, c.poolURL)
	if err != nil {
		return nil, fmt.Errorf("deriving conversation key: %w", err)
	}

	c.mu.Lock()
	c.keys[targetHash] = key
	c.mu.Unlock()

	return key, nil
}
```

- [ ] **Step 4: Modify SendMessage to encrypt**

Replace the existing `SendMessage` method:

```go
// SendMessage sends an encrypted chat message to a target user.
func (c *Client) SendMessage(targetHash, body string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	key, err := c.getOrDeriveKey(ctx, targetHash)
	if err != nil {
		return fmt.Errorf("deriving key: %w", err)
	}

	encBody, err := crypto.SealMessage(key, body)
	if err != nil {
		return fmt.Errorf("encrypting message: %w", err)
	}

	msg := protocol.Message{
		Type:       protocol.TypeMsg,
		Token:      c.Token(),
		SourceHash: c.hashID,
		TargetHash: targetHash,
		PoolURL:    c.poolURL,
		Body:       encBody,
		Encrypted:  true,
		Ts:         time.Now().Unix(),
	}
	return c.send(msg)
}
```

- [ ] **Step 5: Modify readLoop to handle KeyResponse and decrypt messages**

In `internal/cli/relay/client.go`, update the `readLoop` switch to add `KeyResponse` handling and message decryption:

```go
case *protocol.KeyResponse:
	pubBytes, err := hex.DecodeString(f.PubKey)
	if err == nil && len(pubBytes) == ed25519.PublicKeySize {
		c.mu.Lock()
		ch, ok := c.keyReqs[f.TargetHash]
		if ok {
			select {
			case ch <- ed25519.PublicKey(pubBytes):
			default:
			}
			delete(c.keyReqs, f.TargetHash)
		}
		c.mu.Unlock()
	}
case *protocol.Message:
	if f.Encrypted {
		// Derive key and decrypt
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		key, err := c.getOrDeriveKey(ctx, f.SourceHash)
		cancel()
		if err == nil {
			if plain, err := crypto.OpenMessage(key, f.Body); err == nil {
				f.Body = plain
				f.Encrypted = false
			}
		}
	}
	if c.onMsg != nil {
		c.onMsg(*f)
	}
```

Add `"context"` to the import if not already there (it already is).

- [ ] **Step 6: Build to verify compilation**

Run: `go build ./internal/cli/relay/`
Expected: success

- [ ] **Step 7: Commit**

```bash
git add internal/cli/relay/client.go
git commit -m "feat: add E2E encryption to relay client (RequestKey, encrypt send, decrypt receive)"
```

---

## Chunk 5: Fix Existing Tests for Encrypted SendMessage

### Task 5: Update existing tests that use SendMessage

After Task 4, `SendMessage` always encrypts. This triggers a `KeyRequest` round-trip before sending. Existing tests that call `SendMessage` will break because:
- The mock relay in `client_integration_test.go` doesn't handle `KeyRequest` frames
- The E2E tests in `relay_e2e_test.go` don't set up pubkeys for key derivation correctly

**Files:**
- Modify: `internal/cli/relay/client.go` — add `SendMessagePlain` for backward compat / testing
- Modify: `internal/relay/relay_e2e_test.go`

- [ ] **Step 1: Verify existing tests break**

Run: `DATING_HOME=$(mktemp -d) go test ./internal/relay/ ./internal/cli/relay/ -timeout 30s -count=1 2>&1 | tail -20`
Expected: Several FAIL (existing message tests timeout waiting for KeyResponse)

- [ ] **Step 2: Fix existing E2E tests**

The existing E2E tests (`TestE2E_MessageRouting_BothOnline`, etc.) should now pass because:
- Both users are in the store with their pubkeys
- The relay handles `KeyRequest` frames (added in Task 3)
- The client auto-derives keys via `getOrDeriveKey` in the new `SendMessage`

If any test still fails, the issue is timing — the `KeyRequest`/`KeyResponse` round-trip adds latency. Increase timeouts if needed from 3s to 5s.

Run: `DATING_HOME=$(mktemp -d) go test ./internal/relay/ -v -timeout 60s -count=1 2>&1`

Verify all existing `TestE2E_*` tests still pass. If they do, no changes needed.

- [ ] **Step 3: Fix client integration tests**

The mock relay in `client_integration_test.go` needs to handle `KeyRequest` frames. Update the `doAuth` helper or individual test handlers to respond to `KeyRequest` with a `KeyResponse`. Alternatively, add a `SendMessagePlain` method to the client that sends without encryption (for tests that only care about the transport layer, not crypto).

Add to `internal/cli/relay/client.go`:

```go
// SendMessagePlain sends a plaintext (unencrypted) chat message.
// Used for testing transport-layer behavior without encryption overhead.
func (c *Client) SendMessagePlain(targetHash, body string) error {
	msg := protocol.Message{
		Type:       protocol.TypeMsg,
		Token:      c.Token(),
		SourceHash: c.hashID,
		TargetHash: targetHash,
		PoolURL:    c.poolURL,
		Body:       body,
		Ts:         time.Now().Unix(),
	}
	return c.send(msg)
}
```

Update existing integration tests that test transport (not crypto) to use `SendMessagePlain` instead of `SendMessage`.

- [ ] **Step 4: Run all tests to verify**

Run: `DATING_HOME=$(mktemp -d) go test ./... -timeout 60s -count=1`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/relay/client.go internal/cli/relay/client_integration_test.go internal/relay/relay_e2e_test.go
git commit -m "fix: update existing tests for encrypted SendMessage, add SendMessagePlain"
```

---

## Chunk 6: New E2E Encryption Tests

### Task 6: Add E2E encryption tests

**Files:**
- Modify: `internal/relay/relay_e2e_test.go`

- [ ] **Step 1: Add encrypted message delivery test**

Append to `internal/relay/relay_e2e_test.go`:

```go
func TestE2E_EncryptedMessage_Delivered(t *testing.T) {
	env := newTestEnv(t)

	pubA, privA := genKeys(t)
	pubB, privB := genKeys(t)
	hidA := env.addUser("owner/pool", "alice", "github", pubA)
	hidB := env.addUser("owner/pool", "bob", "github", pubB)
	env.addMatch(hidA, hidB)

	clientA := connectClient(t, env, "owner/pool", "alice", "github", pubA, privA)
	clientB := connectClient(t, env, "owner/pool", "bob", "github", pubB, privB)

	msgCh := make(chan protocol.Message, 1)
	clientB.OnMessage(func(msg protocol.Message) { msgCh <- msg })

	if err := clientA.SendMessage(hidB, "secret hello!"); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case msg := <-msgCh:
		// Client should have decrypted it
		if msg.Body != "secret hello!" {
			t.Errorf("body = %q, want 'secret hello!'", msg.Body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for encrypted message")
	}
}
```

- [ ] **Step 2: Add relay-cannot-read test**

This test needs access to the raw message body as seen by the relay. We'll add a hook to intercept. Instead, we verify the Message on the wire has `Encrypted: true` and body is not plaintext:

```go
func TestE2E_EncryptedMessage_RelayCannotRead(t *testing.T) {
	env := newTestEnv(t)

	pubA, privA := genKeys(t)
	pubB, privB := genKeys(t)
	hidA := env.addUser("owner/pool", "alice", "github", pubA)
	hidB := env.addUser("owner/pool", "bob", "github", pubB)
	env.addMatch(hidA, hidB)

	clientA := connectClient(t, env, "owner/pool", "alice", "github", pubA, privA)
	clientB := connectClient(t, env, "owner/pool", "bob", "github", pubB, privB)

	// Capture raw message at Bob's side before decryption
	rawCh := make(chan protocol.Message, 1)
	clientB.OnRawMessage(func(msg protocol.Message) { rawCh <- msg })

	clientA.SendMessage(hidB, "top secret")

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
```

Note: This test requires adding an `OnRawMessage` callback to the client that fires before decryption. Add to `Client`:

```go
onRawMsg func(protocol.Message)
```

And a setter:

```go
func (c *Client) OnRawMessage(fn func(protocol.Message)) {
	c.onRawMsg = fn
}
```

In `readLoop`, invoke before decryption:

```go
case *protocol.Message:
	if c.onRawMsg != nil {
		c.onRawMsg(*f)
	}
	// ... then decrypt ...
```

- [ ] **Step 3: Add bidirectional encrypted conversation test**

```go
func TestE2E_EncryptedConversation_Bidirectional(t *testing.T) {
	env := newTestEnv(t)

	pubA, privA := genKeys(t)
	pubB, privB := genKeys(t)
	hidA := env.addUser("owner/pool", "alice", "github", pubA)
	hidB := env.addUser("owner/pool", "bob", "github", pubB)
	env.addMatch(hidA, hidB)

	clientA := connectClient(t, env, "owner/pool", "alice", "github", pubA, privA)
	clientB := connectClient(t, env, "owner/pool", "bob", "github", pubB, privB)

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
```

- [ ] **Step 4: Add key request error tests**

```go
func TestE2E_KeyRequest_NotMatched(t *testing.T) {
	env := newTestEnv(t)

	pubA, privA := genKeys(t)
	pubB, _ := genKeys(t)
	env.addUser("owner/pool", "alice", "github", pubA)
	hidB := env.addUser("owner/pool", "bob", "github", pubB)
	// No match!

	clientA := connectClient(t, env, "owner/pool", "alice", "github", pubA, privA)

	errCh := make(chan protocol.Error, 1)
	clientA.OnError(func(e protocol.Error) { errCh <- e })

	// SendMessage will trigger a KeyRequest which should fail
	clientA.SendMessage(hidB, "hello")

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
	hidA := env.addUser("owner/pool", "alice", "github", pubA)
	fakeHash := "nonexistent12345"
	env.addMatch(hidA, fakeHash)

	clientA := connectClient(t, env, "owner/pool", "alice", "github", pubA, privA)

	errCh := make(chan protocol.Error, 1)
	clientA.OnError(func(e protocol.Error) { errCh <- e })

	clientA.SendMessage(fakeHash, "hello")

	select {
	case e := <-errCh:
		if e.Code != protocol.ErrUserNotFound {
			t.Errorf("code = %q, want user_not_found", e.Code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for user_not_found error")
	}
}
```

- [ ] **Step 5: Add mixed encrypted/unencrypted backward compat test**

```go
func TestE2E_MixedEncryptedUnencrypted(t *testing.T) {
	env := newTestEnv(t)

	pubA, privA := genKeys(t)
	pubB, privB := genKeys(t)
	hidA := env.addUser("owner/pool", "alice", "github", pubA)
	hidB := env.addUser("owner/pool", "bob", "github", pubB)
	env.addMatch(hidA, hidB)

	clientA := connectClient(t, env, "owner/pool", "alice", "github", pubA, privA)
	clientB := connectClient(t, env, "owner/pool", "bob", "github", pubB, privB)

	msgCh := make(chan protocol.Message, 2)
	clientB.OnMessage(func(msg protocol.Message) { msgCh <- msg })

	// Send plaintext (using SendMessagePlain)
	clientA.SendMessagePlain(hidB, "plain hello")
	select {
	case msg := <-msgCh:
		if msg.Body != "plain hello" {
			t.Errorf("plain body = %q", msg.Body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout for plain message")
	}

	// Send encrypted (using SendMessage)
	clientA.SendMessage(hidB, "encrypted hello")
	select {
	case msg := <-msgCh:
		if msg.Body != "encrypted hello" {
			t.Errorf("encrypted body = %q", msg.Body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout for encrypted message")
	}
}
```

- [ ] **Step 6: Run all tests**

Run: `DATING_HOME=$(mktemp -d) go test ./... -timeout 60s -count=1`
Expected: ALL PASS

- [ ] **Step 7: Run with race detector**

Run: `DATING_HOME=$(mktemp -d) go test ./internal/relay/ ./internal/cli/relay/ ./internal/crypto/ -race -timeout 60s`
Expected: PASS, no races

- [ ] **Step 8: Commit**

```bash
git add internal/relay/relay_e2e_test.go internal/cli/relay/client.go
git commit -m "feat: add E2E encryption tests (delivery, relay-opaque, bidirectional, mixed, error cases)"
```

- [ ] **Step 9: Build binaries**

Run: `make build`
Expected: builds `bin/dating` + `bin/relay` successfully
