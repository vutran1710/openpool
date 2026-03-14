package relay

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
)

const PubKeySize = ed25519.PublicKeySize

func GenerateNonce() ([]byte, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return nonce, nil
}

func VerifySignature(pubKey ed25519.PublicKey, nonce []byte, signatureHex string) bool {
	sig, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false
	}
	return ed25519.Verify(pubKey, nonce, sig)
}

func FetchUserBin(poolRepo, poolToken, userHash string) ([]byte, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/contents/users/%s.bin", poolRepo, userHash)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if poolToken != "" {
		req.Header.Set("Authorization", "Bearer "+poolToken)
	}
	req.Header.Set("Accept", "application/vnd.github.raw+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching user bin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("user not found")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func ExtractPubKey(bin []byte) (ed25519.PublicKey, error) {
	if len(bin) < PubKeySize {
		return nil, fmt.Errorf("bin too short")
	}
	return ed25519.PublicKey(bin[:PubKeySize]), nil
}

func MatchExists(poolRepo, poolToken, rootA, rootB string) bool {
	ph := PairHash(rootA, rootB)
	url := fmt.Sprintf("https://api.github.com/repos/%s/contents/matches/%s", poolRepo, ph)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false
	}
	if poolToken != "" {
		req.Header.Set("Authorization", "Bearer "+poolToken)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func PairHash(a, b string) string {
	combined := a + ":" + b
	if a > b {
		combined = b + ":" + a
	}
	h := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(h[:])[:12]
}
