package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/vutran1710/dating-dev/internal/crypto"
)

func cmdDecrypt() {
	operatorKeyHex := envOrArg("--operator-key", "OPERATOR_PRIVATE_KEY")
	body := envOrArg("--body", "")

	operatorKey, err := hex.DecodeString(operatorKeyHex)
	if err != nil || len(operatorKey) != ed25519.PrivateKeySize {
		log.Fatal("invalid operator key")
	}

	blobBytes, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		log.Fatalf("base64 decode: %v", err)
	}

	plaintext, err := crypto.Decrypt(ed25519.PrivateKey(operatorKey), blobBytes)
	if err != nil {
		log.Fatalf("decrypt: %v", err)
	}

	fmt.Print(string(plaintext))
}
