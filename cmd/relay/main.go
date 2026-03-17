package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/vutran1710/dating-dev/internal/relay"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	ttl := 15 * time.Minute
	if s := os.Getenv("TOKEN_TTL"); s != "" {
		if sec, err := strconv.Atoi(s); err == nil {
			ttl = time.Duration(sec) * time.Second
		}
	}

	srv := relay.NewServer(relay.ServerConfig{
		TokenTTL: ttl,
	})

	if err := srv.ListenAndServe(relay.Addr(port)); err != nil {
		log.Fatal(err)
	}
}
