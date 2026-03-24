package main

import (
	"os"

	"github.com/vutran1710/openpool/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
