package relay

import (
	"context"
	"testing"
	"time"
)

func TestClient_NewClient_Fields(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{
		RelayURL:  "http://localhost:8081",
		PoolURL:   "owner/pool",
		BinHash:   "test_bin_hash123",
		MatchHash: "test_match_hash1",
		Pub:       pub,
		Priv:      priv,
	})

	if c.url != "http://localhost:8081" {
		t.Errorf("url = %q", c.url)
	}
	if c.binHash != "test_bin_hash123" {
		t.Errorf("binHash = %q", c.binHash)
	}
	if c.matchHash != "test_match_hash1" {
		t.Errorf("matchHash = %q", c.matchHash)
	}
	if c.closed {
		t.Error("new client should not be closed")
	}
}

func TestClient_Close_BeforeConnect(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{Pub: pub, Priv: priv, RelayURL: "http://localhost:1"})
	c.Close()
	if !c.closed {
		t.Error("should be closed")
	}
}

func TestClient_Close_Idempotent(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{Pub: pub, Priv: priv, RelayURL: "http://localhost:1"})
	c.Close()
	c.Close()
	c.Close()
}

func TestClient_SendMessage_NotConnected(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{
		Pub: pub, Priv: priv, RelayURL: "http://localhost:1",
		BinHash: "test", MatchHash: "test",
	})
	err := c.SendMessagePlain("target", "hello")
	if err == nil {
		t.Fatal("expected error when not connected")
	}
}

func TestClient_Connect_Unreachable(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{
		RelayURL:  "http://127.0.0.1:1",
		BinHash:   "test_bin",
		MatchHash: "test_match",
		Pub:       pub, Priv: priv,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := c.Connect(ctx)
	if err == nil {
		c.Close()
		t.Fatal("expected error for unreachable server")
	}
}

func TestClient_BinHash_EmptyBeforeConnect(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{Pub: pub, Priv: priv, RelayURL: "http://localhost:1"})
	if c.BinHash() != "" {
		t.Errorf("binHash should be empty, got %q", c.BinHash())
	}
}

func TestClient_WsURL_Conversion(t *testing.T) {
	pub, priv := genTestKeys()
	tests := []struct {
		input string
		want  string
	}{
		{"ws://localhost:8081", "ws://localhost:8081"},
		{"wss://relay.example.com", "wss://relay.example.com"},
		{"http://localhost:8081", "ws://localhost:8081"},
	}

	for _, tt := range tests {
		c := NewClient(Config{RelayURL: tt.input, Pub: pub, Priv: priv})
		got := c.wsURL()
		if got != tt.want {
			t.Errorf("wsURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
