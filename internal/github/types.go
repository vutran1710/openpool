package github

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

type PullRequest struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	Labels    []Label   `json:"labels"`
	Head      Branch    `json:"head"`
}

type Label struct {
	Name string `json:"name"`
}

type Branch struct {
	Ref string `json:"ref"`
}

type User struct {
	Login string `json:"login"`
	ID    int    `json:"id"`
}

type Issue struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	State       string    `json:"state"`
	StateReason string    `json:"state_reason"`
	Labels      []Label   `json:"labels"`
	User        User      `json:"user"`
	CreatedAt   time.Time `json:"created_at"`
}

func decodeBase64(encoded string) ([]byte, error) {
	cleaned := strings.ReplaceAll(encoded, "\n", "")
	return base64.StdEncoding.DecodeString(cleaned)
}

func decodeJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
