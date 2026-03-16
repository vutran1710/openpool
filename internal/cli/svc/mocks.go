package svc

import (
	"context"
	"fmt"
	"sync"

	"github.com/vutran1710/dating-dev/internal/cli/config"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

// --- Mock Profile ---

type MockProfile struct {
	mu      sync.Mutex
	Global  *gh.DatingProfile
	Pools   map[string]*gh.DatingProfile
	SaveErr error
}

func NewMockProfile() *MockProfile {
	return &MockProfile{Pools: make(map[string]*gh.DatingProfile)}
}

func (m *MockProfile) LoadGlobal() (*gh.DatingProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Global == nil {
		return nil, fmt.Errorf("no global profile")
	}
	return m.Global, nil
}
func (m *MockProfile) SaveGlobal(p *gh.DatingProfile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Global = p
	return m.SaveErr
}
func (m *MockProfile) LoadPool(name string) (*gh.DatingProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.Pools[name]
	if !ok {
		return nil, fmt.Errorf("no pool profile: %s", name)
	}
	return p, nil
}
func (m *MockProfile) SavePool(name string, p *gh.DatingProfile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Pools[name] = p
	return m.SaveErr
}
func (m *MockProfile) GlobalPath() string          { return "/mock/profile.json" }
func (m *MockProfile) PoolPath(name string) string { return "/mock/pools/" + name + "/profile.json" }

// --- Mock Persistence ---

type MockPersistence struct {
	mu     sync.Mutex
	Pools  map[string]config.PoolConfig
	Token  string
	User   struct{ DisplayName, Username, Provider, ProviderUserID string }
	Registries []string
}

func NewMockPersistence() *MockPersistence {
	return &MockPersistence{Pools: make(map[string]config.PoolConfig)}
}

func (m *MockPersistence) SavePendingPool(pool config.PoolConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	pool.Status = "pending"
	m.Pools[pool.Name] = pool
	return nil
}
func (m *MockPersistence) MarkPoolActive(name, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.Pools[name]; ok {
		p.Status = "active"
		p.UserHash = hash
		p.PendingIssue = 0
		m.Pools[name] = p
	}
	return nil
}
func (m *MockPersistence) MarkPoolRejected(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.Pools[name]; ok {
		p.Status = "rejected"
		p.PendingIssue = 0
		m.Pools[name] = p
	}
	return nil
}
func (m *MockPersistence) SaveEncryptedToken(hex string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Token = hex
	return nil
}
func (m *MockPersistence) DecryptToken() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Token == "" {
		return "", fmt.Errorf("no token")
	}
	return m.Token, nil
}
func (m *MockPersistence) SaveUserIdentity(dn, un, pr, pid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.User.DisplayName = dn
	m.User.Username = un
	m.User.Provider = pr
	m.User.ProviderUserID = pid
	return nil
}
func (m *MockPersistence) AddRegistry(url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Registries = append(m.Registries, url)
	return nil
}

// --- Mock Polling ---

type MockPolling struct {
	Updates []StatusUpdate
	idx     int
}

func NewMockPolling() *MockPolling {
	return &MockPolling{}
}

func (m *MockPolling) Start(_ context.Context, _ chan<- StatusUpdate) {}
func (m *MockPolling) Stop()                                          {}
func (m *MockPolling) PollOnce(_ context.Context) *StatusUpdate {
	if m.idx >= len(m.Updates) {
		return nil
	}
	u := m.Updates[m.idx]
	m.idx++
	return &u
}
