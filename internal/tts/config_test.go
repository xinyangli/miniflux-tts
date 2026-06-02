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
	t.Setenv("MINIFLUX_TTS_OPENAI_FORMAT", "")

	config := ConfigFromEnv()
	if config.Provider != "openai" {
		t.Fatalf("expected openai provider, got %q", config.Provider)
	}
	if config.OpenAIBaseURL != "https://api.openai.com/v1" {
		t.Fatalf("expected default OpenAI base URL, got %q", config.OpenAIBaseURL)
	}
	if config.OpenAIModel != "gpt-audio-1.5" {
		t.Fatalf("expected default OpenAI model, got %q", config.OpenAIModel)
	}
	if config.OpenAIVoice != "alloy" {
		t.Fatalf("expected default OpenAI voice, got %q", config.OpenAIVoice)
	}
	if config.OpenAIFormat != "wav" {
		t.Fatalf("expected default OpenAI format, got %q", config.OpenAIFormat)
	}
	if config.WorkerCount != 1 {
		t.Fatalf("expected default worker count 1, got %d", config.WorkerCount)
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}
}

func TestConfigFromEnvWorkerCount(t *testing.T) {
	t.Setenv("MINIFLUX_TTS_WORKER_COUNT", "3")

	config := ConfigFromEnv()
	if config.WorkerCount != 3 {
		t.Fatalf("expected worker count 3, got %d", config.WorkerCount)
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
		OpenAIModel:      "gpt-audio-1.5",
		OpenAIVoice:      "alloy",
		OpenAIFormat:     "wav",
		WorkerCount:      1,
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
		WorkerCount:      1,
	}

	if err := config.Validate(); err != nil {
		t.Fatalf("expected fake provider config to bypass OpenAI key, got %v", err)
	}
}

func TestConfigValidateInvalidWorkerCount(t *testing.T) {
	config := Config{
		MinifluxBaseURL:  "http://miniflux.test",
		MinifluxAPIToken: "mf-token",
		PublicBaseURL:    "http://tts.test",
		StorageDir:       "data/audio",
		Provider:         "fake",
		WorkerCount:      0,
	}

	err := config.Validate()
	if err == nil {
		t.Fatal("expected invalid worker count error")
	}
	if !strings.Contains(err.Error(), "MINIFLUX_TTS_WORKER_COUNT") {
		t.Fatalf("expected worker count error, got %v", err)
	}
}

func TestConfigValidateOpenAICompatibleChatCompletions(t *testing.T) {
	config := Config{
		MinifluxBaseURL:  "http://miniflux.test",
		MinifluxAPIToken: "mf-token",
		PublicBaseURL:    "http://tts.test",
		StorageDir:       "data/audio",
		Provider:         "openai",
		OpenAIAPIKey:     "openai-token",
		OpenAIBaseURL:    "https://api.xiaomimimo.com/v1",
		OpenAIModel:      "mimo-v2.5-tts",
		OpenAIFormat:     "wav",
		WorkerCount:      1,
	}

	if err := config.Validate(); err != nil {
		t.Fatalf("expected valid OpenAI-compatible chat completions config, got %v", err)
	}
}
