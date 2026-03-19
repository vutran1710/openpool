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

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "/data/relay.db"
	}

	srv := relay.NewServer(relay.ServerConfig{
		PoolURL: poolURL,
		Salt:    salt,
		DBPath:  dbPath,
	})

	log.Printf("relay: users=%d matches=%d", srv.Store().UserCount(), srv.Store().MatchCount())

	if err := srv.ListenAndServe(relay.Addr(port)); err != nil {
		log.Fatal(err)
	}
}
