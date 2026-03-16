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

type Issue struct {
	Number      int    `json:"number"`
	State       string `json:"state"`
	StateReason string `json:"state_reason"`
}

type PoolManifest struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	Version          int    `json:"version"`
	CreatedAt        string `json:"created_at"`
	OperatorPubKey   string `json:"operator_public_key"`
	RelayURL         string `json:"relay_url,omitempty"`
}

func decodeBase64(encoded string) ([]byte, error) {
	cleaned := strings.ReplaceAll(encoded, "\n", "")
	return base64.StdEncoding.DecodeString(cleaned)
}

func decodeJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
