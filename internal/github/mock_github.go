package github

import (
	"context"
	"fmt"
	"sync"
)

// MockGitHub is an in-memory GitHub API for testing.
type MockGitHub struct {
	mu     sync.Mutex
	User   *UserInfo
	Issues map[int]*Issue
	Files  map[string][]byte
	Repos  map[string]bool // name → private
	nextID int
}

func NewMockGitHub() *MockGitHub {
	return &MockGitHub{
		Issues: make(map[int]*Issue),
		Files:  make(map[string][]byte),
		Repos:  make(map[string]bool),
		nextID: 1,
	}
}

func (m *MockGitHub) GetUser(_ context.Context, _ string) (*UserInfo, error) {
	if m.User == nil {
		return nil, fmt.Errorf("no user configured")
	}
	return m.User, nil
}

func (m *MockGitHub) CreateIssue(_ context.Context, title, body string, labels []string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextID
	m.nextID++
	m.Issues[id] = &Issue{Number: id, State: "open"}
	return id, nil
}

func (m *MockGitHub) GetIssue(_ context.Context, number int) (*Issue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	issue, ok := m.Issues[number]
	if !ok {
		return nil, fmt.Errorf("issue %d not found", number)
	}
	return issue, nil
}

func (m *MockGitHub) StarRepo(_ context.Context, _ string) error {
	return nil
}

func (m *MockGitHub) CreateRepo(_ context.Context, _, name string, private bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Repos[name] = private
	return nil
}

func (m *MockGitHub) CommitFile(_ context.Context, _, _, path, _ string, content []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Files[path] = content
	return nil
}

func (m *MockGitHub) GetFileContent(_ context.Context, path string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.Files[path]
	if !ok {
		return nil, fmt.Errorf("not found: %s", path)
	}
	return data, nil
}

// CloseIssue is a test helper to simulate the GitHub Action closing an issue.
func (m *MockGitHub) CloseIssue(number int, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if issue, ok := m.Issues[number]; ok {
		issue.State = "closed"
		issue.StateReason = reason
	}
}
