package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"miniflux-tts/internal/tts"
)

func main() {
	config := tts.ConfigFromEnv()
	if err := config.Validate(); err != nil {
		slog.Error("invalid config", slog.Any("error", err))
		os.Exit(1)
	}
	backend, err := tts.NewBackend(config)
	if err != nil {
		slog.Error("create backend failed", slog.Any("error", err))
		os.Exit(1)
	}

	server := tts.NewServer(config, backend)
	slog.Info("starting tts service", slog.String("addr", config.Addr), slog.String("provider", config.Provider), slog.Int("worker_count", config.WorkerCount), slog.String("storage_dir", config.StorageDir))
	server.StartWorkers(context.Background())
	slog.Info("listening", slog.String("addr", config.Addr))
	if err := http.ListenAndServe(config.Addr, server.Handler()); err != nil {
		slog.Error("http server stopped", slog.Any("error", err))
		os.Exit(1)
	}
}
