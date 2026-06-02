package tts

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Addr               string
	MinifluxBaseURL    string
	MinifluxAPIToken   string
	PublicBaseURL      string
	StorageDir         string
	AllowedOrigin      string
	BrowserToken       string
	Provider           string
	OpenAIAPIKey       string
	OpenAIBaseURL      string
	OpenAIModel        string
	OpenAIVoice        string
	OpenAIFormat       string
	OpenAIInstructions string
	WorkerCount        int
}

func ConfigFromEnv() Config {
	return Config{
		Addr:               envOrDefault("TTS_ADDR", ":8090"),
		MinifluxBaseURL:    os.Getenv("MINIFLUX_BASE_URL"),
		MinifluxAPIToken:   os.Getenv("MINIFLUX_API_TOKEN"),
		PublicBaseURL:      strings.TrimRight(envOrDefault("PUBLIC_BASE_URL", "http://localhost:8090"), "/"),
		StorageDir:         envOrDefault("STORAGE_DIR", "data/audio"),
		AllowedOrigin:      os.Getenv("ALLOWED_MINIFLUX_ORIGIN"),
		BrowserToken:       os.Getenv("TTS_BROWSER_TOKEN"),
		Provider:           envOrDefault("MINIFLUX_TTS_PROVIDER", "openai"),
		OpenAIAPIKey:       os.Getenv("MINIFLUX_TTS_OPENAI_API_KEY"),
		OpenAIBaseURL:      strings.TrimRight(envOrDefault("MINIFLUX_TTS_OPENAI_BASE_URL", "https://api.openai.com/v1"), "/"),
		OpenAIModel:        envOrDefault("MINIFLUX_TTS_OPENAI_MODEL", "gpt-audio-1.5"),
		OpenAIVoice:        envOrDefault("MINIFLUX_TTS_OPENAI_VOICE", "alloy"),
		OpenAIFormat:       envOrDefault("MINIFLUX_TTS_OPENAI_FORMAT", "wav"),
		OpenAIInstructions: envOrDefault("MINIFLUX_TTS_OPENAI_INSTRUCTIONS", "Read this article clearly and naturally."),
		WorkerCount:        envIntOrDefault("MINIFLUX_TTS_WORKER_COUNT", 1),
	}
}

func (c Config) Validate() error {
	if c.MinifluxBaseURL == "" {
		return errors.New("MINIFLUX_BASE_URL is required")
	}
	if c.MinifluxAPIToken == "" {
		return errors.New("MINIFLUX_API_TOKEN is required")
	}
	if c.PublicBaseURL == "" {
		return errors.New("PUBLIC_BASE_URL is required")
	}
	if c.StorageDir == "" {
		return errors.New("STORAGE_DIR is required")
	}
	if c.WorkerCount < 1 {
		return errors.New("MINIFLUX_TTS_WORKER_COUNT must be >= 1")
	}
	switch c.Provider {
	case "fake":
		return nil
	case "openai":
		if c.OpenAIAPIKey == "" {
			return errors.New("MINIFLUX_TTS_OPENAI_API_KEY is required when MINIFLUX_TTS_PROVIDER=openai")
		}
		if c.OpenAIBaseURL == "" {
			return errors.New("MINIFLUX_TTS_OPENAI_BASE_URL is required when MINIFLUX_TTS_PROVIDER=openai")
		}
		if c.OpenAIFormat == "" {
			return errors.New("MINIFLUX_TTS_OPENAI_FORMAT is required when MINIFLUX_TTS_PROVIDER=openai")
		}
	default:
		return errors.New("MINIFLUX_TTS_PROVIDER must be openai or fake")
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	if parsed < 1 {
		return parsed
	}
	return parsed
}
