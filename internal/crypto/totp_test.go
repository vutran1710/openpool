package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"testing"
	"time"
)

const testRelayHost = "relay.example.com"

func TestTOTPSign_Valid(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig := TOTPSign(priv, testRelayHost)
	if len(sig) != 128 {
		t.Errorf("sig length = %d, want 128 hex chars", len(sig))
	}
	if _, err := hex.DecodeString(sig); err != nil {
		t.Errorf("sig is not valid hex: %v", err)
	}
	if !TOTPVerify(sig, pub, testRelayHost) {
		t.Error("valid signature should verify")
	}
}

func TestTOTPVerify_WrongKey(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	pub2, _, _ := ed25519.GenerateKey(rand.Reader)
	sig := TOTPSign(priv, testRelayHost)
	if TOTPVerify(sig, pub2, testRelayHost) {
		t.Error("wrong key should not verify")
	}
}

func TestTOTPVerify_DriftWindow(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now().Unix() / 300
	sig := TOTPSignAt(priv, testRelayHost, now-1)
	if !TOTPVerify(sig, pub, testRelayHost) {
		t.Error("previous time window should verify (drift tolerance)")
	}
}

func TestTOTPVerify_Expired(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now().Unix() / 300
	sig := TOTPSignAt(priv, testRelayHost, now-2)
	if TOTPVerify(sig, pub, testRelayHost) {
		t.Error("2 windows ago should NOT verify")
	}
}

func TestTOTPVerify_MalformedSig(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	if TOTPVerify("not-hex-zzzz", pub, testRelayHost) {
		t.Error("invalid hex should not verify")
	}
}

func TestTOTPSignAt_Deterministic(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig1 := TOTPSignAt(priv, testRelayHost, 100)
	sig2 := TOTPSignAt(priv, testRelayHost, 100)
	if sig1 != sig2 {
		t.Error("same key + same time window should produce same signature")
	}
}

func TestTOTPVerify_NextWindow(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now().Unix() / 300
	sig := TOTPSignAt(priv, testRelayHost, now+1)
	if !TOTPVerify(sig, pub, testRelayHost) {
		t.Error("next time window should verify (drift tolerance)")
	}
}

func TestTOTPVerify_FarFutureWindow(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now().Unix() / 300
	sig := TOTPSignAt(priv, testRelayHost, now+2)
	if TOTPVerify(sig, pub, testRelayHost) {
		t.Error("2 windows in future should NOT verify")
	}
}

func TestTOTPVerify_TruncatedSignature(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig := TOTPSign(priv, testRelayHost)
	if TOTPVerify(sig[:64], pub, testRelayHost) {
		t.Error("truncated signature should not verify")
	}
}

func TestTOTPVerify_WrongRelayHost(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig := TOTPSign(priv, "relay-a.example.com")
	if TOTPVerify(sig, pub, "relay-b.example.com") {
		t.Error("signature for relay-a should not verify on relay-b")
	}
}

func TestTOTPVerify_ConcurrentSafe(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig := TOTPSign(priv, testRelayHost)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !TOTPVerify(sig, pub, testRelayHost) {
				t.Error("concurrent verify should succeed")
			}
		}()
	}
	wg.Wait()
}

func BenchmarkTOTPSign(b *testing.B) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	for i := 0; i < b.N; i++ {
		TOTPSign(priv, testRelayHost)
	}
}

func BenchmarkTOTPVerify(b *testing.B) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig := TOTPSign(priv, testRelayHost)
	for i := 0; i < b.N; i++ {
		TOTPVerify(sig, pub, testRelayHost)
	}
}
