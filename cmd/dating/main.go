package main

import (
	"os"

	"github.com/vutran1710/dating-dev/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
