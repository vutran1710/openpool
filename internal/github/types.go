package github

import (
	"encoding/base64"
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

type PoolManifest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     int    `json:"version"`
	CreatedAt   string `json:"created_at"`
}

type UserProfile struct {
	PublicID    string   `json:"public_id"`
	DisplayName string  `json:"display_name"`
	Bio         string  `json:"bio,omitempty"`
	City        string  `json:"city,omitempty"`
	Interests   []string `json:"interests,omitempty"`
	LookingFor  string  `json:"looking_for,omitempty"`
	PublicKey   string  `json:"public_key"`
	Status      string  `json:"status"`
	JoinedAt    string  `json:"joined_at"`
}

func decodeBase64(encoded string) ([]byte, error) {
	cleaned := strings.ReplaceAll(encoded, "\n", "")
	return base64.StdEncoding.DecodeString(cleaned)
}
