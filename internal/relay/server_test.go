package relay

import "testing"

func TestBinHash_Deterministic(t *testing.T) {
	srv := &Server{salt: "test-salt"}
	h1 := srv.BinHash("abc123")
	h2 := srv.BinHash("abc123")
	if h1 != h2 {
		t.Fatal("should be deterministic")
	}
	if len(h1) != 16 {
		t.Fatalf("should be 16 hex chars, got %d", len(h1))
	}
}

func TestMatchHash_Deterministic(t *testing.T) {
	srv := &Server{salt: "test-salt"}
	h := srv.MatchHash("abc123")
	if len(h) != 16 {
		t.Fatalf("should be 16 hex chars, got %d", len(h))
	}
}

func TestPairHash_Order(t *testing.T) {
	h1 := PairHash("aaa", "bbb")
	h2 := PairHash("bbb", "aaa")
	if h1 != h2 {
		t.Fatal("should be order-independent")
	}
	if len(h1) != 12 {
		t.Fatalf("should be 12 hex chars, got %d", len(h1))
	}
}

func TestChain_Derivation(t *testing.T) {
	srv := &Server{salt: "test-salt"}
	bin := srv.BinHash("some-id-hash")
	match := srv.MatchHash(bin)
	if bin == match {
		t.Fatal("bin_hash and match_hash should differ")
	}
}
