package protocol

import (
	"fmt"

	"github.com/vmihailenco/msgpack/v5"
)

// Encode serializes a frame to MessagePack bytes.
func Encode(v any) ([]byte, error) {
	return msgpack.Marshal(v)
}

// DecodeType peeks at the "type" field without fully decoding.
func DecodeType(data []byte) (string, error) {
	var raw map[string]any
	if err := msgpack.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("decode type: %w", err)
	}
	t, ok := raw["type"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid type field")
	}
	return t, nil
}

// Decode deserializes MessagePack bytes into the target struct.
func Decode(data []byte, v any) error {
	return msgpack.Unmarshal(data, v)
}

// DecodeFrame decodes a raw frame into the appropriate typed struct.
func DecodeFrame(data []byte) (any, error) {
	typ, err := DecodeType(data)
	if err != nil {
		return nil, err
	}

	switch typ {
	case TypeAuth:
		var f AuthRequest
		return &f, Decode(data, &f)
	case TypeChallenge:
		var f Challenge
		return &f, Decode(data, &f)
	case TypeAuthResponse:
		var f AuthResponse
		return &f, Decode(data, &f)
	case TypeAuthenticated:
		var f Authenticated
		return &f, Decode(data, &f)
	case TypeRefresh:
		var f RefreshRequest
		return &f, Decode(data, &f)
	case TypeIdentity:
		var f IdentityRequest
		return &f, Decode(data, &f)
	case TypeIdentityResponse:
		var f IdentityResponse
		return &f, Decode(data, &f)
	case TypeMsg:
		var f Message
		return &f, Decode(data, &f)
	case TypeAck:
		var f Ack
		return &f, Decode(data, &f)
	case TypeError:
		var f Error
		return &f, Decode(data, &f)
	default:
		return nil, fmt.Errorf("unknown frame type: %s", typ)
	}
}
