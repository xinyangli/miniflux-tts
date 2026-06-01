package tts

import (
	"strings"
	"testing"
)

func TestConfigFromEnvOpenAIDefaults(t *testing.T) {
	t.Setenv("MINIFLUX_BASE_URL", "http://miniflux.test")
	t.Setenv("MINIFLUX_API_TOKEN", "mf-token")
	t.Setenv("MINIFLUX_TTS_PROVIDER", "")
	t.Setenv("MINIFLUX_TTS_OPENAI_API_KEY", "openai-token")
	t.Setenv("MINIFLUX_TTS_OPENAI_BASE_URL", "")
	t.Setenv("MINIFLUX_TTS_OPENAI_MODEL", "")
	t.Setenv("MINIFLUX_TTS_OPENAI_VOICE", "")

	config := ConfigFromEnv()
	if config.Provider != "openai" {
		t.Fatalf("expected openai provider, got %q", config.Provider)
	}
	if config.OpenAIBaseURL != "https://api.openai.com/v1" {
		t.Fatalf("expected default OpenAI base URL, got %q", config.OpenAIBaseURL)
	}
	if config.OpenAIModel != "gpt-4o-mini-tts" {
		t.Fatalf("expected default OpenAI model, got %q", config.OpenAIModel)
	}
	if config.OpenAIVoice != "alloy" {
		t.Fatalf("expected default OpenAI voice, got %q", config.OpenAIVoice)
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}
}

func TestConfigValidateMissingOpenAIAPIKey(t *testing.T) {
	config := Config{
		MinifluxBaseURL:  "http://miniflux.test",
		MinifluxAPIToken: "mf-token",
		PublicBaseURL:    "http://tts.test",
		StorageDir:       "data/audio",
		Provider:         "openai",
		OpenAIBaseURL:    "https://api.openai.com/v1",
		OpenAIModel:      "gpt-4o-mini-tts",
		OpenAIVoice:      "alloy",
	}

	err := config.Validate()
	if err == nil {
		t.Fatal("expected missing OpenAI API key error")
	}
	if !strings.Contains(err.Error(), "MINIFLUX_TTS_OPENAI_API_KEY") {
		t.Fatalf("expected OpenAI API key error, got %v", err)
	}
}

func TestConfigValidateFakeProviderBypassesOpenAIAPIKey(t *testing.T) {
	config := Config{
		MinifluxBaseURL:  "http://miniflux.test",
		MinifluxAPIToken: "mf-token",
		PublicBaseURL:    "http://tts.test",
		StorageDir:       "data/audio",
		Provider:         "fake",
	}

	if err := config.Validate(); err != nil {
		t.Fatalf("expected fake provider config to bypass OpenAI key, got %v", err)
	}
}
