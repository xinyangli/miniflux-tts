package main

import (
	"context"
	"log"
	"net/http"

	"miniflux-tts/internal/tts"
)

func main() {
	config := tts.ConfigFromEnv()
	if err := config.Validate(); err != nil {
		log.Fatal(err)
	}
	backend, err := tts.NewBackend(config)
	if err != nil {
		log.Fatal(err)
	}

	server := tts.NewServer(config, backend)
	server.StartWorkers(context.Background())
	log.Printf("listening on %s", config.Addr)
	log.Fatal(http.ListenAndServe(config.Addr, server.Handler()))
}
