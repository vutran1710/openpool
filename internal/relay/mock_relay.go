package relay

import (
	"context"
	"fmt"
)

// MockRelay is an in-memory relay API for testing.
type MockRelay struct {
	Identities map[string]string // pubkey → user_hash
}

func NewMockRelay() *MockRelay {
	return &MockRelay{
		Identities: make(map[string]string),
	}
}

func (m *MockRelay) GetIdentity(_ context.Context, _ string, req IdentityRequest) (*IdentityResponse, error) {
	hash, ok := m.Identities[req.PubKey]
	if !ok {
		return nil, fmt.Errorf("identity not found for pubkey %s", req.PubKey[:12])
	}
	return &IdentityResponse{UserHash: hash}, nil
}
