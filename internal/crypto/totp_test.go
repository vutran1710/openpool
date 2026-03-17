package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"testing"
	"time"
)

func TestTOTPSign_ProducesHexSignature(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig := TOTPSign("abcd1234abcd1234", "efgh5678efgh5678", priv)
	if len(sig) != 128 {
		t.Errorf("sig length = %d, want 128 hex chars", len(sig))
	}
	if _, err := hex.DecodeString(sig); err != nil {
		t.Errorf("sig is not valid hex: %v", err)
	}
}

func TestTOTPSign_Deterministic(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	bin, match := "abcd1234abcd1234", "efgh5678efgh5678"
	sig1 := TOTPSignAt(bin, match, priv, 100)
	sig2 := TOTPSignAt(bin, match, priv, 100)
	if sig1 != sig2 {
		t.Error("same inputs + same time window should produce same signature")
	}
}

func TestTOTPVerify_ValidSignature(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	bin, match := "abcd1234abcd1234", "efgh5678efgh5678"
	sig := TOTPSign(bin, match, priv)
	if !TOTPVerify(bin, match, sig, pub) {
		t.Error("valid signature should verify")
	}
}

func TestTOTPVerify_WrongKey(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	pub2, _, _ := ed25519.GenerateKey(rand.Reader)
	sig := TOTPSign("abcd1234abcd1234", "efgh5678efgh5678", priv)
	if TOTPVerify("abcd1234abcd1234", "efgh5678efgh5678", sig, pub2) {
		t.Error("wrong key should not verify")
	}
}

func TestTOTPVerify_WrongMatchHash(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig := TOTPSign("abcd1234abcd1234", "efgh5678efgh5678", priv)
	if TOTPVerify("abcd1234abcd1234", "wrong_match_hash", sig, pub) {
		t.Error("wrong match_hash should not verify")
	}
}

func TestTOTPVerify_WrongBinHash(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig := TOTPSign("abcd1234abcd1234", "efgh5678efgh5678", priv)
	if TOTPVerify("wrong_bin_hash00", "efgh5678efgh5678", sig, pub) {
		t.Error("wrong bin_hash should not verify")
	}
}

func TestTOTPVerify_InvalidHex(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	if TOTPVerify("abcd1234abcd1234", "efgh5678efgh5678", "not-hex-zzzz", pub) {
		t.Error("invalid hex should not verify")
	}
}

func TestTOTPVerify_TruncatedSignature(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig := TOTPSign("abcd1234abcd1234", "efgh5678efgh5678", priv)
	if TOTPVerify("abcd1234abcd1234", "efgh5678efgh5678", sig[:64], pub) {
		t.Error("truncated signature should not verify")
	}
}

func TestTOTPVerify_EmptyInputs(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig := TOTPSign("", "", priv)
	if !TOTPVerify("", "", sig, pub) {
		t.Error("empty inputs should still produce a valid verifiable signature")
	}
	if TOTPVerify("x", "", sig, pub) {
		t.Error("different bin_hash should fail")
	}
}

func TestTOTPVerify_PreviousWindow(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	bin, match := "abcd1234abcd1234", "efgh5678efgh5678"
	now := time.Now().Unix() / 300
	sig := TOTPSignAt(bin, match, priv, now-1)
	if !TOTPVerify(bin, match, sig, pub) {
		t.Error("previous time window should verify (drift tolerance)")
	}
}

func TestTOTPVerify_NextWindow(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	bin, match := "abcd1234abcd1234", "efgh5678efgh5678"
	now := time.Now().Unix() / 300
	sig := TOTPSignAt(bin, match, priv, now+1)
	if !TOTPVerify(bin, match, sig, pub) {
		t.Error("next time window should verify (drift tolerance)")
	}
}

func TestTOTPVerify_ExpiredWindow(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	bin, match := "abcd1234abcd1234", "efgh5678efgh5678"
	now := time.Now().Unix() / 300
	sig := TOTPSignAt(bin, match, priv, now-2)
	if TOTPVerify(bin, match, sig, pub) {
		t.Error("2 windows ago should NOT verify")
	}
}

func TestTOTPVerify_FarFutureWindow(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	bin, match := "abcd1234abcd1234", "efgh5678efgh5678"
	now := time.Now().Unix() / 300
	sig := TOTPSignAt(bin, match, priv, now+2)
	if TOTPVerify(bin, match, sig, pub) {
		t.Error("2 windows in future should NOT verify")
	}
}

func TestTOTPVerify_ConcurrentSafe(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	bin, match := "abcd1234abcd1234", "efgh5678efgh5678"
	sig := TOTPSign(bin, match, priv)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !TOTPVerify(bin, match, sig, pub) {
				t.Error("concurrent verify should succeed")
			}
		}()
	}
	wg.Wait()
}

func BenchmarkTOTPSign(b *testing.B) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	for i := 0; i < b.N; i++ {
		TOTPSign("abcd1234abcd1234", "efgh5678efgh5678", priv)
	}
}

func BenchmarkTOTPVerify(b *testing.B) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig := TOTPSign("abcd1234abcd1234", "efgh5678efgh5678", priv)
	for i := 0; i < b.N; i++ {
		TOTPVerify("abcd1234abcd1234", "efgh5678efgh5678", sig, pub)
	}
}
