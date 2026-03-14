package main

import (
	"context"
	"log"

	"github.com/vutran1710/dating-dev/internal/server"
)

func main() {
	if err := server.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
