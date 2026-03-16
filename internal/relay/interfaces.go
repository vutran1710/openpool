package relay

import "context"

// IdentityRequest is sent by the CLI to get the user's hash for a pool.
type IdentityRequest struct {
	PoolRepo  string `json:"pool_repo"`
	PubKey    string `json:"pub_key"`
	Signature string `json:"signature"`
}

// IdentityResponse contains the user's hash computed by the relay.
type IdentityResponse struct {
	UserHash string `json:"user_hash"`
}

// RelayAPI defines the interface for relay server operations.
// Implementations: relayClient (real), MockRelay (test).
type RelayAPI interface {
	GetIdentity(ctx context.Context, relayURL string, req IdentityRequest) (*IdentityResponse, error)
}
