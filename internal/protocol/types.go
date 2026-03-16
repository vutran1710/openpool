// Package protocol defines the relay wire protocol types.
// These types are language-agnostic — any implementation must
// support these frame types over MessagePack-encoded WebSocket frames.
package protocol

// Frame types
const (
	TypeAuth             = "auth"
	TypeChallenge        = "challenge"
	TypeAuthResponse     = "auth_response"
	TypeAuthenticated    = "authenticated"
	TypeRefresh          = "refresh"
	TypeIdentity         = "identity"
	TypeIdentityResponse = "identity_response"
	TypeMsg              = "msg"
	TypeAck              = "ack"
	TypeError            = "error"
)

// Error codes
const (
	ErrUserNotFound = "user_not_found"
	ErrAuthFailed   = "auth_failed"
	ErrTokenExpired = "token_expired"
	ErrTokenInvalid = "token_invalid"
	ErrNotMatched   = "not_matched"
	ErrRateLimited  = "rate_limited"
	ErrPoolNotFound = "pool_not_found"
	ErrInternal     = "internal_error"
)

// --- Client → Relay ---

// AuthRequest initiates authentication.
type AuthRequest struct {
	Type     string `msgpack:"type"`
	UserID   string `msgpack:"user_id"`
	Provider string `msgpack:"provider"`
	PoolURL  string `msgpack:"pool_url"`
}

// AuthResponse sends the signed nonce back.
type AuthResponse struct {
	Type      string `msgpack:"type"`
	Signature string `msgpack:"signature"`
}

// RefreshRequest refreshes an expiring token.
type RefreshRequest struct {
	Type  string `msgpack:"type"`
	Token string `msgpack:"token"`
}

// IdentityRequest queries the user's hash_id for a pool.
type IdentityRequest struct {
	Type    string `msgpack:"type"`
	PoolURL string `msgpack:"pool_url"`
}

// Message is a chat message (bidirectional).
type Message struct {
	Type       string `msgpack:"type"`
	Token      string `msgpack:"token,omitempty"`      // only client→relay
	MsgID      string `msgpack:"msg_id,omitempty"`     // only relay→client
	SourceHash string `msgpack:"source_hash"`
	TargetHash string `msgpack:"target_hash"`
	PoolURL    string `msgpack:"pool_url"`
	Body       string `msgpack:"body"`
	Ts         int64  `msgpack:"ts"`
}

// Ack acknowledges message delivery.
type Ack struct {
	Type  string `msgpack:"type"`
	MsgID string `msgpack:"msg_id"`
}

// --- Relay → Client ---

// Challenge sends a nonce for the client to sign.
type Challenge struct {
	Type  string `msgpack:"type"`
	Nonce string `msgpack:"nonce"`
}

// Authenticated confirms auth and provides a token.
type Authenticated struct {
	Type      string `msgpack:"type"`
	Token     string `msgpack:"token"`
	ExpiresAt int64  `msgpack:"expires_at"`
	HashID    string `msgpack:"hash_id"`
}

// IdentityResponse returns the user's hash_id.
type IdentityResponse struct {
	Type    string `msgpack:"type"`
	PoolURL string `msgpack:"pool_url"`
	HashID  string `msgpack:"hash_id"`
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
	PoolURL  string `msgpack:"pool_url" json:"pool_url"`
	HashID1  string `msgpack:"hash_id_1" json:"hash_id_1"`
	HashID2  string `msgpack:"hash_id_2" json:"hash_id_2"`
}

// --- Data Model ---

// UserIndex represents a user in the relay's storage.
type UserIndex struct {
	PoolURL  string `msgpack:"pool_url"`
	PubKey   []byte `msgpack:"pubkey"`    // 32-byte ed25519 public key
	UserID   string `msgpack:"user_id"`
	Provider string `msgpack:"provider"`
	HashID   string `msgpack:"hash_id"`
}

// Match represents a matched pair.
type Match struct {
	PoolURL   string `msgpack:"pool_url"`
	HashID1   string `msgpack:"hash_id_1"`
	HashID2   string `msgpack:"hash_id_2"`
	CreatedAt int64  `msgpack:"created_at"`
}

// QueuedMessage is a message waiting for delivery.
type QueuedMessage struct {
	MsgID      string `msgpack:"msg_id"`
	PoolURL    string `msgpack:"pool_url"`
	SourceHash string `msgpack:"source_hash"`
	TargetHash string `msgpack:"target_hash"`
	Body       string `msgpack:"body"`
	Ts         int64  `msgpack:"ts"`
	Delivered  bool   `msgpack:"delivered"`
	Acked      bool   `msgpack:"acked"`
}
