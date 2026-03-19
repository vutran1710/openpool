package main

import (
	"log"
	"os"

	"github.com/vutran1710/dating-dev/internal/relay"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	poolURL := os.Getenv("POOL_URL")
	if poolURL == "" {
		log.Fatal("POOL_URL is required (e.g. owner/pool-name)")
	}

	salt := os.Getenv("POOL_SALT")
	if salt == "" {
		log.Fatal("POOL_SALT is required")
	}

	srv := relay.NewServer(relay.ServerConfig{
		PoolURL: poolURL,
		Salt:    salt,
	})

	if err := srv.ListenAndServe(relay.Addr(port)); err != nil {
		log.Fatal(err)
	}
}
