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
	Name           string      `json:"name"`
	Description    string      `json:"description"`
	Version        int         `json:"version"`
	CreatedAt      string      `json:"created_at"`
	OperatorPubKey string      `json:"operator_public_key"`
	RelayURL       string      `json:"relay_url,omitempty"`
	Schema         *PoolSchema `json:"schema,omitempty"`
}

// PoolSchema defines profile fields and vector encoding for a pool.
type PoolSchema struct {
	Version int           `json:"version"`
	Fields  []SchemaField `json:"fields"`
}

// SchemaField defines a single profile field and how it maps to vector dimensions.
type SchemaField struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`            // "enum", "multi", "range"
	Values []string `json:"values,omitempty"` // for enum/multi
	Min    *int     `json:"min,omitempty"`    // for range
	Max    *int     `json:"max,omitempty"`    // for range
}

// Dimensions returns the vector dimensions for this field.
func (f SchemaField) Dimensions() int {
	switch f.Type {
	case "enum", "multi":
		return len(f.Values)
	case "range":
		return 1
	}
	return 0
}

// Dimensions returns the total vector dimensions across all fields.
func (s *PoolSchema) Dimensions() int {
	d := 0
	for _, f := range s.Fields {
		d += f.Dimensions()
	}
	return d
}

func decodeBase64(encoded string) ([]byte, error) {
	cleaned := strings.ReplaceAll(encoded, "\n", "")
	return base64.StdEncoding.DecodeString(cleaned)
}

func decodeJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
