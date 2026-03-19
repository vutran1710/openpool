package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"testing"
	"time"
)

func TestTOTPSign_Valid(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig := TOTPSign(priv)
	if len(sig) != 128 {
		t.Errorf("sig length = %d, want 128 hex chars", len(sig))
	}
	if _, err := hex.DecodeString(sig); err != nil {
		t.Errorf("sig is not valid hex: %v", err)
	}
	if !TOTPVerify(sig, pub) {
		t.Error("valid signature should verify")
	}
}

func TestTOTPVerify_WrongKey(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	pub2, _, _ := ed25519.GenerateKey(rand.Reader)
	sig := TOTPSign(priv)
	if TOTPVerify(sig, pub2) {
		t.Error("wrong key should not verify")
	}
}

func TestTOTPVerify_DriftWindow(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now().Unix() / 300
	sig := TOTPSignAt(priv, now-1)
	if !TOTPVerify(sig, pub) {
		t.Error("previous time window should verify (drift tolerance)")
	}
}

func TestTOTPVerify_Expired(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now().Unix() / 300
	sig := TOTPSignAt(priv, now-2)
	if TOTPVerify(sig, pub) {
		t.Error("2 windows ago should NOT verify")
	}
}

func TestTOTPVerify_MalformedSig(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	if TOTPVerify("not-hex-zzzz", pub) {
		t.Error("invalid hex should not verify")
	}
}

func TestTOTPSignAt_Deterministic(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig1 := TOTPSignAt(priv, 100)
	sig2 := TOTPSignAt(priv, 100)
	if sig1 != sig2 {
		t.Error("same key + same time window should produce same signature")
	}
}

func TestTOTPVerify_NextWindow(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now().Unix() / 300
	sig := TOTPSignAt(priv, now+1)
	if !TOTPVerify(sig, pub) {
		t.Error("next time window should verify (drift tolerance)")
	}
}

func TestTOTPVerify_FarFutureWindow(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now().Unix() / 300
	sig := TOTPSignAt(priv, now+2)
	if TOTPVerify(sig, pub) {
		t.Error("2 windows in future should NOT verify")
	}
}

func TestTOTPVerify_TruncatedSignature(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig := TOTPSign(priv)
	if TOTPVerify(sig[:64], pub) {
		t.Error("truncated signature should not verify")
	}
}

func TestTOTPVerify_ConcurrentSafe(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig := TOTPSign(priv)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !TOTPVerify(sig, pub) {
				t.Error("concurrent verify should succeed")
			}
		}()
	}
	wg.Wait()
}

func BenchmarkTOTPSign(b *testing.B) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	for i := 0; i < b.N; i++ {
		TOTPSign(priv)
	}
}

func BenchmarkTOTPVerify(b *testing.B) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig := TOTPSign(priv)
	for i := 0; i < b.N; i++ {
		TOTPVerify(sig, pub)
	}
}
