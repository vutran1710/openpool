package github

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/message"
)

type mockCommentsClient struct {
	comments []IssueComment
}

func (m *mockCommentsClient) ListIssueComments(_ context.Context, _ int) ([]IssueComment, error) {
	return m.comments, nil
}

// Stub all other GitHubClient methods — panic if called.
func (m *mockCommentsClient) GetFile(_ context.Context, _ string) ([]byte, error)            { panic("not implemented") }
func (m *mockCommentsClient) ListDir(_ context.Context, _ string) ([]string, error)          { panic("not implemented") }
func (m *mockCommentsClient) FileExists(_ context.Context, _ string) bool                    { panic("not implemented") }
func (m *mockCommentsClient) CreateIssue(_ context.Context, _, _ string, _ []string) (int, error) { panic("not implemented") }
func (m *mockCommentsClient) GetIssue(_ context.Context, _ int) (*Issue, error)              { panic("not implemented") }
func (m *mockCommentsClient) CloseIssue(_ context.Context, _ int, _ string) error            { panic("not implemented") }
func (m *mockCommentsClient) LockIssue(_ context.Context, _ int, _ string) error             { panic("not implemented") }
func (m *mockCommentsClient) ListIssues(_ context.Context, _ string, _ ...string) ([]Issue, error) { panic("not implemented") }
func (m *mockCommentsClient) CommentIssue(_ context.Context, _ int, _ string) error          { panic("not implemented") }
func (m *mockCommentsClient) ListPullRequests(_ context.Context, _ string) ([]PullRequest, error) { panic("not implemented") }
func (m *mockCommentsClient) CreatePullRequest(_ context.Context, _ PRRequest) (int, error)  { panic("not implemented") }
func (m *mockCommentsClient) MergePullRequest(_ context.Context, _ int) error                { panic("not implemented") }
func (m *mockCommentsClient) ClosePullRequest(_ context.Context, _ int) error                { panic("not implemented") }
func (m *mockCommentsClient) CommentPR(_ context.Context, _ int, _ string) error             { panic("not implemented") }
func (m *mockCommentsClient) GetDefaultBranch(_ context.Context) (string, error)             { panic("not implemented") }
func (m *mockCommentsClient) TriggerWorkflow(_ context.Context, _ string, _ map[string]string) error { panic("not implemented") }
func (m *mockCommentsClient) StarRepo(_ context.Context) error                               { panic("not implemented") }

func makeSignedComment(t *testing.T, blockType string, operatorPriv ed25519.PrivateKey, userPub ed25519.PublicKey) string {
	t.Helper()
	plaintext := []byte("test-payload")
	encrypted, err := crypto.Encrypt(userPub, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	sig := ed25519.Sign(operatorPriv, encrypted)
	signedBlob := base64.StdEncoding.EncodeToString(encrypted) + "." + hex.EncodeToString(sig)
	return message.Format(blockType, signedBlob)
}

func TestFindOperatorReply_Valid(t *testing.T) {
	operatorPub, operatorPriv, _ := ed25519.GenerateKey(rand.Reader)
	userPub, _, _ := ed25519.GenerateKey(rand.Reader)

	commentBody := makeSignedComment(t, "registration", operatorPriv, userPub)
	mock := &mockCommentsClient{
		comments: []IssueComment{
			{Body: "random noise"},
			{Body: commentBody},
		},
	}

	content, err := FindOperatorReplyInIssue(context.Background(), mock, 1, "registration", operatorPub)
	if err != nil {
		t.Fatal(err)
	}
	if content == "" {
		t.Fatal("expected non-empty content")
	}
}

func TestFindOperatorReply_WrongBlockType(t *testing.T) {
	operatorPub, operatorPriv, _ := ed25519.GenerateKey(rand.Reader)
	userPub, _, _ := ed25519.GenerateKey(rand.Reader)

	commentBody := makeSignedComment(t, "match", operatorPriv, userPub)
	mock := &mockCommentsClient{
		comments: []IssueComment{{Body: commentBody}},
	}

	_, err := FindOperatorReplyInIssue(context.Background(), mock, 1, "registration", operatorPub)
	if err == nil {
		t.Fatal("expected error for wrong block type")
	}
}

func TestFindOperatorReply_ForgedSignature(t *testing.T) {
	operatorPub, _, _ := ed25519.GenerateKey(rand.Reader)
	_, attackerPriv, _ := ed25519.GenerateKey(rand.Reader)
	userPub, _, _ := ed25519.GenerateKey(rand.Reader)

	commentBody := makeSignedComment(t, "registration", attackerPriv, userPub)
	mock := &mockCommentsClient{
		comments: []IssueComment{{Body: commentBody}},
	}

	_, err := FindOperatorReplyInIssue(context.Background(), mock, 1, "registration", operatorPub)
	if err == nil {
		t.Fatal("expected error for forged signature")
	}
}

func TestFindOperatorReply_NewestFirst(t *testing.T) {
	operatorPub, operatorPriv, _ := ed25519.GenerateKey(rand.Reader)
	userPub, _, _ := ed25519.GenerateKey(rand.Reader)

	old := makeSignedComment(t, "registration", operatorPriv, userPub)
	new_ := makeSignedComment(t, "registration", operatorPriv, userPub)

	mock := &mockCommentsClient{
		comments: []IssueComment{
			{Body: old},
			{Body: "noise"},
			{Body: new_},
		},
	}

	content, err := FindOperatorReplyInIssue(context.Background(), mock, 1, "registration", operatorPub)
	if err != nil {
		t.Fatal(err)
	}
	if content == "" {
		t.Fatal("expected content")
	}
}

func TestFindOperatorReply_NoComments(t *testing.T) {
	operatorPub, _, _ := ed25519.GenerateKey(rand.Reader)
	mock := &mockCommentsClient{comments: nil}

	_, err := FindOperatorReplyInIssue(context.Background(), mock, 1, "registration", operatorPub)
	if err == nil {
		t.Fatal("expected error for no comments")
	}
}
