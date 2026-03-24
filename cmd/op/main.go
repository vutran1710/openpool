package main

import (
	"os"

	"github.com/vutran1710/openpool/internal/cli"
)

var version = "dev"

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
