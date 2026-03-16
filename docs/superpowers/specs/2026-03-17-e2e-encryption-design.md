# E2E Encryption for Relay Chat Messages

## Problem

After authentication, all chat messages flow through the relay in plaintext. The relay can read every message body. Each conversation between a match pair should be encrypted with a unique key so that:

1. The relay is zero-knowledge about message content
2. Each match pair has its own encryption key (compromising one conversation doesn't expose others)

## Approach: Static ECDH + HKDF + NaCl Secretbox

Use each user's existing ed25519 identity key (converted to curve25519) to perform X25519 Diffie-Hellman. Derive a unique symmetric key per conversation via HKDF. Encrypt message bodies with NaCl secretbox.

**Why this approach:**
- No online requirement — works with offline message queuing
- No extra handshake — both sides compute the same key independently
- Different key per conversation — different pubkey pairs + pool context = unique derived key
- All crypto primitives already exist in `internal/crypto/`
- Relay remains a dumb pipe

**Trade-off:** No forward secrecy. A compromised static key exposes all conversations for that user. Acceptable for v1; ratcheting can be added later.

## New Protocol Frames

### KeyRequest (client → relay, auth + match required)

```
{
  type: "key_request",
  target_hash: "8d419fa9098bdec3"
}
```

Client requests the ed25519 public key of a matched user. Relay verifies the requesting user is matched with the target before responding. If not matched, relay responds with `{type: "error", code: "not_matched", message: "..."}`. If target not found, relay responds with `{type: "error", code: "user_not_found", message: "..."}`.

### KeyResponse (relay → client)

```
{
  type: "key_response",
  target_hash: "8d419fa9098bdec3",
  pubkey: "hex-encoded 32-byte ed25519 pubkey"
}
```

### Message Frame Change

Existing `Message` struct gains one field:

```
encrypted: bool (msgpack:"encrypted,omitempty")
```

When `encrypted: true`, the `body` field contains base64-encoded `version_byte + nonce + ciphertext` instead of plaintext. The relay never inspects `body` — it's opaque either way. The relay MUST forward the `encrypted` field as-is when routing messages.

## Encryption Flow

### Key Derivation (both sides compute independently)

```
1. Convert keys:
   my_curve_priv    = ed25519PrivToCurve25519(my_ed25519_priv)
   their_curve_pub  = ed25519PubToCurve25519(their_ed25519_pub)

2. ECDH:
   shared_secret    = X25519(my_curve_priv, their_curve_pub)

3. Derive conversation key:
   salt             = nil (explicit: HKDF extract uses zero-filled 32-byte buffer)
   info             = "dating-e2e:" + sorted(hashA, hashB) + ":" + pool_url
   conversation_key = HKDF-SHA256(ikm=shared_secret, salt=nil, info=info, length=32)
```

Both sides arrive at the same 32-byte symmetric key because `X25519(a_priv, b_pub) == X25519(b_priv, a_pub)`.

The `info` string includes sorted hashes and pool_url, ensuring each match pair in each pool gets a distinct key even if the same two users are matched in multiple pools.

### Sending a Message

```
1. If conversation_key not cached:
   a. Send KeyRequest for target_hash
   b. Receive KeyResponse with target's ed25519 pubkey
   c. Compute shared_secret via ECDH
   d. Derive conversation_key via HKDF
   e. Cache: keys[target_hash] = conversation_key

2. Generate random 24-byte nonce
3. ciphertext = secretbox.Seal(conversation_key, nonce, []byte(plaintext))
4. body = base64(0x01 || nonce || ciphertext)
   - 0x01 = version byte (for future scheme upgrades)
5. Set encrypted = true
```

### Receiving a Message

```
1. If msg.Encrypted == false:
   Deliver body as-is (backward compat)

2. If conversation_key not cached for source_hash:
   a. Send KeyRequest for source_hash
   b. Receive KeyResponse, derive key, cache it

3. Decode base64 body
4. Check version byte (first byte must be 0x01)
5. Extract nonce (bytes 1-24), ciphertext (remainder)
6. plaintext = secretbox.Open(conversation_key, nonce, ciphertext)
7. Deliver decrypted plaintext to callback
```

### Key Request Lifecycle

The client manages inflight key requests via `keyReqs map[string]chan ed25519.PublicKey`:

- Channel buffer size: 1 (non-blocking send from readLoop)
- On `RequestKey(ctx, targetHash)`:
  - If a request for the same target is already inflight, reuse the existing channel (deduplication)
  - Send `KeyRequest` frame
  - Block on channel or `ctx.Done()`
  - On context cancellation: remove channel from map, return error (prevents goroutine leak)
- On `KeyResponse` in readLoop:
  - Look up channel in `keyReqs[targetHash]`
  - Send pubkey to channel (non-blocking)
  - If no channel exists (stale response), discard silently
  - Delete channel from map after send

### Key Caching

Client maintains `map[target_hash][]byte` in memory. Cache is lost on disconnect — re-derivation costs one `key_request` round-trip + cheap ECDH computation. No persistent key storage needed.

## File Changes

### `internal/crypto/encrypt.go` — New Functions

```go
// SharedSecret computes X25519 ECDH between own ed25519 private key
// and peer's ed25519 public key. Returns 32-byte shared secret.
func SharedSecret(myPriv ed25519.PrivateKey, theirPub ed25519.PublicKey) ([]byte, error)

// DeriveConversationKey derives a 32-byte symmetric key from a shared
// secret using HKDF-SHA256 with conversation context.
// Validates: shared must be 32 bytes, hashA/hashB/poolURL must be non-empty.
func DeriveConversationKey(shared []byte, hashA, hashB, poolURL string) ([]byte, error)

// SealMessage encrypts plaintext with a conversation key.
// Returns base64-encoded (version_byte || nonce || ciphertext).
// Version byte is 0x01 for this scheme.
func SealMessage(key []byte, plaintext string) (string, error)

// OpenMessage decrypts a base64-encoded (version_byte || nonce || ciphertext).
// Returns plaintext string. Validates version byte.
func OpenMessage(key []byte, encoded string) (string, error)
```

### `internal/protocol/types.go` — Additions

```go
const (
    TypeKeyRequest  = "key_request"
    TypeKeyResponse = "key_response"
)

type KeyRequest struct {
    Type       string `msgpack:"type"`
    TargetHash string `msgpack:"target_hash"`
}

type KeyResponse struct {
    Type       string `msgpack:"type"`
    TargetHash string `msgpack:"target_hash"`
    PubKey     string `msgpack:"pubkey"`
}

// Add to existing Message struct:
// Encrypted bool `msgpack:"encrypted,omitempty"`

// Add to existing QueuedMessage struct:
// Encrypted bool `msgpack:"encrypted"`
```

### `internal/protocol/codec.go`

Add `TypeKeyRequest` and `TypeKeyResponse` cases to `DecodeFrame` switch.

### `internal/relay/session.go` — Changes

```go
// New handler:
func (s *Session) handleKeyRequest(req *protocol.KeyRequest)
// - Requires auth
// - Verifies requester is matched with target (IsMatched check)
// - Looks up target_hash pubkey in store via PubKeyByHash
// - Returns KeyResponse with hex-encoded pubkey
// - Returns ErrNotMatched if not matched
// - Returns ErrUserNotFound if target not in index

// Modified: handleMessage must forward Encrypted field
// outMsg.Encrypted = msg.Encrypted

// Modified: handleFrame must route KeyRequest to handleKeyRequest
```

### `internal/relay/store.go` — New Method

```go
func (s *Store) PubKeyByHash(hashID string) ed25519.PublicKey
// Uses existing LookupByHash, returns .PubKey or nil.
```

### `internal/cli/relay/client.go` — Changes

```go
// New fields on Client struct:
keys     map[string][]byte                    // target_hash → conversation_key
keyReqs  map[string]chan ed25519.PublicKey     // pending key request channels
peerPubs map[string]ed25519.PublicKey          // cached peer pubkeys

// Modified:
func (c *Client) SendMessage(targetHash, body string) error
// - Derives key if not cached (RequestKey + ECDH + HKDF)
// - Encrypts body via SealMessage
// - Sets Encrypted: true on the Message frame

// New:
func (c *Client) RequestKey(ctx context.Context, targetHash string) (ed25519.PublicKey, error)
// - Sends KeyRequest, blocks until KeyResponse via channel
// - Deduplicates concurrent requests for same target
// - Cleans up channel on context cancellation

// Modified:
func (c *Client) readLoop()
// - Handles KeyResponse frames (sends pubkey to waiting channel)
// - Decrypts Message.Body when Encrypted == true before invoking onMsg
```

## Wire Format Impact

- KeyRequest: ~30 bytes
- KeyResponse: ~100 bytes (includes 64-char hex pubkey)
- Encrypted message overhead: 1 byte (version) + 24 bytes (nonce) + 16 bytes (Poly1305 tag) + base64 encoding (~33% expansion) = ~55 bytes over plaintext
- Key derivation: one-time per conversation, cached after

## Testing

### Unit Tests (`internal/crypto/`)

- `TestSharedSecret_BothSidesMatch` — Alice and Bob derive same secret
- `TestDeriveConversationKey_UniquePerPair` — different pairs -> different keys
- `TestDeriveConversationKey_UniquePerPool` — same pair, different pools -> different keys
- `TestDeriveConversationKey_InvalidInput` — empty/short inputs return error
- `TestSealMessage_OpenMessage_RoundTrip` — encrypt then decrypt recovers plaintext
- `TestSealMessage_UniqueNonce` — two seals produce different ciphertext
- `TestOpenMessage_WrongKey_Fails` — different key can't decrypt
- `TestOpenMessage_Tampered_Fails` — modified ciphertext rejected
- `TestOpenMessage_BadVersion_Fails` — wrong version byte rejected

### E2E Tests (`internal/relay/`)

- `TestE2E_EncryptedMessage_Delivered` — Alice sends encrypted, Bob receives decrypted plaintext
- `TestE2E_EncryptedMessage_RelayCannotRead` — intercept message at relay, verify body is not plaintext
- `TestE2E_EncryptedConversation_Bidirectional` — multi-turn encrypted chat
- `TestE2E_KeyRequest_NotMatched` — error frame for unmatched users
- `TestE2E_KeyRequest_TargetNotFound` — error frame for unknown hash
- `TestE2E_MixedEncryptedUnencrypted` — backward compat: unencrypted messages still work
