package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"log"
	"os"
)

func cmdPubkey() {
	operatorKeyHex := envOrArg("--operator-key", "OPERATOR_PRIVATE_KEY")
	binFile := envOrArg("--bin-file", "")

	operatorKey, err := hex.DecodeString(operatorKeyHex)
	if err != nil || len(operatorKey) != ed25519.PrivateKeySize {
		log.Fatal("invalid operator key")
	}
	_ = operatorKey // kept for interface compatibility

	binData, err := os.ReadFile(binFile)
	if err != nil {
		log.Fatalf("reading bin file: %v", err)
	}

	// First 32 bytes of .bin file = user's ed25519 pubkey
	if len(binData) < 32 {
		log.Fatal("bin file too short")
	}
	pubkey := binData[:32]

	fmt.Print(hex.EncodeToString(pubkey))
}
