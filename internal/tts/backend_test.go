package tts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIBackendGenerateSpeech(t *testing.T) {
	var requestBody struct {
		Model          string `json:"model"`
		Voice          string `json:"voice"`
		Input          string `json:"input"`
		ResponseFormat string `json:"response_format"`
	}

	openAIAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/speech" {
			t.Fatalf("expected /v1/audio/speech path, got %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer openai-token" {
			t.Fatalf("expected bearer auth, got %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("mp3-bytes"))
	}))
	defer openAIAPI.Close()

	backend := NewOpenAIBackend(Config{
		OpenAIAPIKey:  "openai-token",
		OpenAIBaseURL: openAIAPI.URL + "/v1",
		OpenAIModel:   "custom-model",
		OpenAIVoice:   "nova",
	})

	input := strings.Repeat("a", openAISpeechInputLimit) + "extra"
	response, err := backend.GenerateSpeech(context.Background(), SpeechRequest{Input: input})
	if err != nil {
		t.Fatal(err)
	}

	if string(response.Audio) != "mp3-bytes" {
		t.Fatalf("expected response audio, got %q", string(response.Audio))
	}
	if response.ContentType != "audio/mpeg" {
		t.Fatalf("expected audio/mpeg content type, got %q", response.ContentType)
	}
	if requestBody.Model != "custom-model" {
		t.Fatalf("expected custom model, got %q", requestBody.Model)
	}
	if requestBody.Voice != "nova" {
		t.Fatalf("expected custom voice, got %q", requestBody.Voice)
	}
	if len([]rune(requestBody.Input)) != openAISpeechInputLimit {
		t.Fatalf("expected input truncated to %d characters, got %d", openAISpeechInputLimit, len([]rune(requestBody.Input)))
	}
	if requestBody.ResponseFormat != "mp3" {
		t.Fatalf("expected mp3 response format, got %q", requestBody.ResponseFormat)
	}
}
