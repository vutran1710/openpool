package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
)

func cmdSign() {
	operatorKeyHex := envOrArg("--operator-key", "OPERATOR_PRIVATE_KEY")
	operatorKey, err := hex.DecodeString(operatorKeyHex)
	if err != nil || len(operatorKey) != ed25519.PrivateKeySize {
		log.Fatal("invalid operator key (expected 128 hex chars / 64 bytes)")
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("reading stdin: %v", err)
	}
	sig := ed25519.Sign(ed25519.PrivateKey(operatorKey), data)
	fmt.Print(hex.EncodeToString(sig))
}
