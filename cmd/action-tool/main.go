package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: action-tool <register|match|squash|index|sign|decrypt|pubkey|managed-register>")
		os.Exit(1)
	}
	switch os.Args[1] {
	case "register":
		cmdRegister()
	case "match":
		cmdMatch()
	case "squash":
		cmdSquash()
	case "index":
		cmdIndex()
	case "sign":
		cmdSign()
	case "decrypt":
		cmdDecrypt()
	case "pubkey":
		cmdPubkey()
	case "managed-register":
		cmdManagedRegister()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
