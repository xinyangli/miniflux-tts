package tts

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	Addr             string
	MinifluxBaseURL  string
	MinifluxAPIToken string
	PublicBaseURL    string
	StorageDir       string
	AllowedOrigin    string
	BrowserToken     string
	Provider         string
	OpenAIAPIKey     string
	OpenAIBaseURL    string
	OpenAIModel      string
	OpenAIVoice      string
}

func ConfigFromEnv() Config {
	return Config{
		Addr:             envOrDefault("TTS_ADDR", ":8090"),
		MinifluxBaseURL:  os.Getenv("MINIFLUX_BASE_URL"),
		MinifluxAPIToken: os.Getenv("MINIFLUX_API_TOKEN"),
		PublicBaseURL:    strings.TrimRight(envOrDefault("PUBLIC_BASE_URL", "http://localhost:8090"), "/"),
		StorageDir:       envOrDefault("STORAGE_DIR", "data/audio"),
		AllowedOrigin:    os.Getenv("ALLOWED_MINIFLUX_ORIGIN"),
		BrowserToken:     os.Getenv("TTS_BROWSER_TOKEN"),
		Provider:         envOrDefault("MINIFLUX_TTS_PROVIDER", "openai"),
		OpenAIAPIKey:     os.Getenv("MINIFLUX_TTS_OPENAI_API_KEY"),
		OpenAIBaseURL:    strings.TrimRight(envOrDefault("MINIFLUX_TTS_OPENAI_BASE_URL", "https://api.openai.com/v1"), "/"),
		OpenAIModel:      envOrDefault("MINIFLUX_TTS_OPENAI_MODEL", "gpt-4o-mini-tts"),
		OpenAIVoice:      envOrDefault("MINIFLUX_TTS_OPENAI_VOICE", "alloy"),
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
