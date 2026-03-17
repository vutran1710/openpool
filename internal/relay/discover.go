package relay

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"

	"github.com/vutran1710/dating-dev/internal/crypto"
)

type DiscoverRequest struct {
	UserHash  string `json:"user_hash"`
	PubKey    string `json:"pub_key"`
	Nonce     string `json:"nonce"`
	Signature string `json:"signature"`
}

type DiscoverResponse struct {
	UserHash         string `json:"user_hash"`
	EncryptedProfile string `json:"encrypted_profile"`
}

// HandleDiscover picks a random profile, decrypts it with the operator key,
// re-encrypts it for the requesting user, and returns it.
func (s *Server) HandleDiscover(w http.ResponseWriter, r *http.Request) {
	if s.operatorPrivKey == nil {
		http.Error(w, `{"error":"operator key not configured"}`, http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}

	var req DiscoverRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if req.UserHash == "" || req.PubKey == "" || req.Signature == "" {
		http.Error(w, `{"error":"missing required fields"}`, http.StatusBadRequest)
		return
	}

	// Verify the requester owns the pubkey by checking signature on the nonce
	requesterPubBytes, err := hex.DecodeString(req.PubKey)
	if err != nil || len(requesterPubBytes) != ed25519.PublicKeySize {
		http.Error(w, `{"error":"invalid public key"}`, http.StatusBadRequest)
		return
	}
	requesterPub := ed25519.PublicKey(requesterPubBytes)

	// Verify signature over "discover:{user_hash}" to authenticate the request
	message := []byte("discover:" + req.UserHash)
	if !crypto.Verify(requesterPub, message, req.Signature) {
		http.Error(w, `{"error":"invalid signature"}`, http.StatusUnauthorized)
		return
	}

	// List all users from the pool repo
	hashes, err := listPoolUsers(s.poolRepo, s.poolToken)
	if err != nil {
		http.Error(w, `{"error":"failed to list users"}`, http.StatusInternalServerError)
		return
	}

	// Filter out the requester
	var candidates []string
	for _, h := range hashes {
		if h != req.UserHash {
			candidates = append(candidates, h)
		}
	}

	if len(candidates) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"user_hash":"","encrypted_profile":""}`))
		return
	}

	// Pick a random profile
	targetHash := candidates[rand.Intn(len(candidates))]

	// Fetch the .bin file
	bin, err := FetchUserBin(s.poolRepo, s.poolToken, targetHash)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch profile"}`, http.StatusInternalServerError)
		return
	}

	// Re-encrypt for the requester
	reEncrypted, err := crypto.ReEncryptForRecipient(s.operatorPrivKey, bin, requesterPub)
	if err != nil {
		http.Error(w, `{"error":"failed to re-encrypt profile"}`, http.StatusInternalServerError)
		return
	}

	resp := DiscoverResponse{
		UserHash:         targetHash,
		EncryptedProfile: hex.EncodeToString(reEncrypted),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// listPoolUsers lists all user hashes from the pool repo via GitHub API.
func listPoolUsers(poolRepo, poolToken string) ([]string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/contents/users", poolRepo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if poolToken != "" {
		req.Header.Set("Authorization", "Bearer "+poolToken)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, nil
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var entries []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}

	var hashes []string
	for _, e := range entries {
		hash := strings.TrimSuffix(e.Name, ".bin")
		if hash != e.Name {
			hashes = append(hashes, hash)
		}
	}
	return hashes, nil
}
