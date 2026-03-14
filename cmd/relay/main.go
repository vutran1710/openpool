package main

import (
	"log"
	"net/http"
	"os"

	"github.com/vutran1710/dating-dev/internal/relay"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	srv := relay.NewServer()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", srv.HandleWS)
	mux.HandleFunc("GET /health", srv.HandleHealth)

	log.Printf("relay server listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
