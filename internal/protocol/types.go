// Package protocol defines the relay wire protocol types.
// These types are language-agnostic — any implementation must
// support these frame types over MessagePack-encoded WebSocket frames.
//
// Auth is handled via TOTP signature at WebSocket upgrade (query params).
// WebSocket only carries message frames after authentication.
package protocol

// Frame types (WebSocket only — no auth frames)
const (
	TypeMsg         = "msg"
	TypeAck         = "ack"
	TypeError       = "error"
	TypeKeyRequest  = "key_request"
	TypeKeyResponse = "key_response"
)

// Error codes
const (
	ErrUserNotFound = "user_not_found"
	ErrAuthFailed   = "auth_failed"
	ErrNotMatched   = "not_matched"
	ErrRateLimited  = "rate_limited"
	ErrInternal     = "internal_error"
)

// --- Chat Frames (WebSocket) ---

// Message is a chat message (bidirectional).
type Message struct {
	Type       string `msgpack:"type"`
	MsgID      string `msgpack:"msg_id,omitempty"` // relay→client only
	SourceHash string `msgpack:"source_hash"`
	TargetHash string `msgpack:"target_hash"`
	Body       string `msgpack:"body"`
	Ts         int64  `msgpack:"ts"`
	Encrypted  bool   `msgpack:"encrypted,omitempty"`
}

// Ack acknowledges message delivery.
type Ack struct {
	Type  string `msgpack:"type"`
	MsgID string `msgpack:"msg_id"`
}

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

// Error is sent for any protocol error.
type Error struct {
	Type    string `msgpack:"type"`
	Code    string `msgpack:"code"`
	Message string `msgpack:"message"`
}

// --- Webhook ---

// MatchWebhook is sent by the pool repo when a match PR is merged.
type MatchWebhook struct {
	PoolURL string `msgpack:"pool_url" json:"pool_url"`
	HashID1 string `msgpack:"hash_id_1" json:"hash_id_1"`
	HashID2 string `msgpack:"hash_id_2" json:"hash_id_2"`
}

// --- Data Model ---

// UserIndex represents a user in the relay's storage.
type UserIndex struct {
	PubKey   []byte `msgpack:"pubkey"` // 32-byte ed25519 public key
	UserID   string `msgpack:"user_id"`
	Provider string `msgpack:"provider"`
	BinHash  string `msgpack:"bin_hash"`
}

// Match represents a matched pair.
type Match struct {
	HashID1   string `msgpack:"hash_id_1"`
	HashID2   string `msgpack:"hash_id_2"`
	CreatedAt int64  `msgpack:"created_at"`
}

// QueuedMessage is a message waiting for delivery.
type QueuedMessage struct {
	MsgID      string `msgpack:"msg_id"`
	SourceHash string `msgpack:"source_hash"`
	TargetHash string `msgpack:"target_hash"`
	Body       string `msgpack:"body"`
	Ts         int64  `msgpack:"ts"`
	Delivered  bool   `msgpack:"delivered"`
	Acked      bool   `msgpack:"acked"`
	Encrypted  bool   `msgpack:"encrypted"`
}
