package relay

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
)

func GenerateNonce() ([]byte, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return nonce, nil
}

func VerifySignature(pubKeyHex string, nonce []byte, signatureHex string) (bool, error) {
	pubBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return false, fmt.Errorf("decoding public key: %w", err)
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		return false, fmt.Errorf("invalid public key length")
	}

	sig, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false, fmt.Errorf("decoding signature: %w", err)
	}

	return ed25519.Verify(ed25519.PublicKey(pubBytes), nonce, sig), nil
}

func UserExistsInPool(poolRepo, poolToken, userHash string) bool {
	url := fmt.Sprintf("https://api.github.com/repos/%s/contents/users/%s.bin", poolRepo, userHash)

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

func MatchExists(poolRepo, poolToken, hashA, hashB string) bool {
	ph := PairHash(hashA, hashB)
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
