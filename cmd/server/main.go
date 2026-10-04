package main

import (
	"log"

	"github.com/grok-agent/backend/internal/api"
)

func main() {
	srv, err := api.NewServer()
	if err != nil {
		log.Fatalf("failed to init server: %v", err)
	}
	if err := srv.Run(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
