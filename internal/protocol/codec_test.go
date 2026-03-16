package protocol

import (
	"testing"
)

func TestEncodeDecode_AuthRequest(t *testing.T) {
	orig := AuthRequest{
		Type:     TypeAuth,
		UserID:   "vutran1710",
		Provider: "github",
		PoolURL:  "vutran1710/dating-test-pool",
	}

	data, err := Encode(orig)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded AuthRequest
	if err := Decode(data, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.UserID != "vutran1710" {
		t.Errorf("expected vutran1710, got %s", decoded.UserID)
	}
	if decoded.Provider != "github" {
		t.Errorf("expected github, got %s", decoded.Provider)
	}
}

func TestEncodeDecode_Message(t *testing.T) {
	orig := Message{
		Type:       TypeMsg,
		SourceHash: "abc123",
		TargetHash: "def456",
		PoolURL:    "owner/pool",
		Body:       "hello world",
		Ts:         1710720000,
	}

	data, err := Encode(orig)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded Message
	Decode(data, &decoded)

	if decoded.Body != "hello world" {
		t.Errorf("expected hello world, got %s", decoded.Body)
	}
	if decoded.Ts != 1710720000 {
		t.Errorf("expected ts 1710720000, got %d", decoded.Ts)
	}
}

func TestEncodeDecode_Error(t *testing.T) {
	orig := Error{
		Type:    TypeError,
		Code:    ErrAuthFailed,
		Message: "bad signature",
	}

	data, _ := Encode(orig)
	var decoded Error
	Decode(data, &decoded)

	if decoded.Code != ErrAuthFailed {
		t.Errorf("expected auth_failed, got %s", decoded.Code)
	}
}

func TestDecodeType(t *testing.T) {
	data, _ := Encode(AuthRequest{Type: TypeAuth, UserID: "test"})
	typ, err := DecodeType(data)
	if err != nil {
		t.Fatalf("decode type: %v", err)
	}
	if typ != TypeAuth {
		t.Errorf("expected auth, got %s", typ)
	}
}

func TestDecodeType_Invalid(t *testing.T) {
	_, err := DecodeType([]byte("not msgpack"))
	if err == nil {
		t.Error("expected error for invalid data")
	}
}

func TestDecodeFrame_AllTypes(t *testing.T) {
	frames := []any{
		&AuthRequest{Type: TypeAuth, UserID: "test"},
		&Challenge{Type: TypeChallenge, Nonce: "abc"},
		&AuthResponse{Type: TypeAuthResponse, Signature: "sig"},
		&Authenticated{Type: TypeAuthenticated, Token: "tok", HashID: "hash"},
		&RefreshRequest{Type: TypeRefresh, Token: "tok"},
		&IdentityRequest{Type: TypeIdentity, PoolURL: "pool"},
		&IdentityResponse{Type: TypeIdentityResponse, HashID: "hash"},
		&Message{Type: TypeMsg, Body: "hi"},
		&Ack{Type: TypeAck, MsgID: "id"},
		&Error{Type: TypeError, Code: ErrInternal},
	}

	for _, orig := range frames {
		data, err := Encode(orig)
		if err != nil {
			t.Fatalf("encode %T: %v", orig, err)
		}

		decoded, err := DecodeFrame(data)
		if err != nil {
			t.Fatalf("decode %T: %v", orig, err)
		}

		if decoded == nil {
			t.Errorf("decoded %T is nil", orig)
		}
	}
}

func TestDecodeFrame_UnknownType(t *testing.T) {
	data, _ := Encode(map[string]string{"type": "unknown"})
	_, err := DecodeFrame(data)
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestMessagePack_SmallerThanJSON(t *testing.T) {
	msg := Message{
		Type:       TypeMsg,
		SourceHash: "fef9b374b0d6f4ad",
		TargetHash: "8d419fa9098bdec3",
		PoolURL:    "vutran1710/dating-test-pool",
		Body:       "hello!",
		Ts:         1710720000,
	}

	msgpackData, _ := Encode(msg)

	// JSON equivalent would be ~150 bytes
	// MessagePack should be notably smaller
	if len(msgpackData) >= 150 {
		t.Errorf("msgpack should be smaller than JSON (~150 bytes), got %d", len(msgpackData))
	}
	t.Logf("MessagePack size: %d bytes", len(msgpackData))
}

func TestMatchWebhook(t *testing.T) {
	orig := MatchWebhook{
		PoolURL: "owner/pool",
		HashID1: "abc",
		HashID2: "def",
	}

	data, _ := Encode(orig)
	var decoded MatchWebhook
	Decode(data, &decoded)

	if decoded.HashID1 != "abc" || decoded.HashID2 != "def" {
		t.Error("match webhook decode failed")
	}
}
