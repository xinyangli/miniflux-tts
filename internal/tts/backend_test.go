package tts

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIBackendGenerateSpeech(t *testing.T) {
	var requestBody struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Modalities []string `json:"modalities"`
		Audio      struct {
			Format string `json:"format"`
			Voice  string `json:"voice"`
		} `json:"audio"`
	}

	openAIAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("expected /v1/chat/completions path, got %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer openai-token" {
			t.Fatalf("expected bearer auth, got %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatal(err)
		}
		writeJSON(w, map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"audio": map[string]string{
							"data": base64.StdEncoding.EncodeToString([]byte("RIFFxxxxWAVEwav-bytes")),
						},
					},
				},
			},
		})
	}))
	defer openAIAPI.Close()

	backend := NewOpenAIBackend(Config{
		OpenAIAPIKey:       "openai-token",
		OpenAIBaseURL:      openAIAPI.URL + "/v1",
		OpenAIModel:        "mimo-v2.5-tts",
		OpenAIVoice:        "Chloe",
		OpenAIFormat:       "wav",
		OpenAIInstructions: "Bright and clear.",
	})

	response, err := backend.GenerateSpeech(context.Background(), SpeechRequest{Input: "Read me."})
	if err != nil {
		t.Fatal(err)
	}

	if string(response.Audio) != "RIFFxxxxWAVEwav-bytes" {
		t.Fatalf("expected decoded wav bytes, got %q", string(response.Audio))
	}
	if response.ContentType != "audio/wav" {
		t.Fatalf("expected audio/wav content type, got %q", response.ContentType)
	}
	if requestBody.Model != "mimo-v2.5-tts" {
		t.Fatalf("expected chat completion model, got %q", requestBody.Model)
	}
	if len(requestBody.Modalities) != 2 || requestBody.Modalities[0] != "text" || requestBody.Modalities[1] != "audio" {
		t.Fatalf("expected text/audio modalities, got %+v", requestBody.Modalities)
	}
	if requestBody.Audio.Format != "wav" || requestBody.Audio.Voice != "Chloe" {
		t.Fatalf("expected wav Chloe audio field, got %+v", requestBody.Audio)
	}
	if len(requestBody.Messages) != 2 {
		t.Fatalf("expected style and text messages, got %d", len(requestBody.Messages))
	}
	if requestBody.Messages[0].Role != "user" || requestBody.Messages[0].Content != "Bright and clear." {
		t.Fatalf("unexpected instruction message: %+v", requestBody.Messages[0])
	}
	if requestBody.Messages[1].Role != "assistant" || requestBody.Messages[1].Content != "Read me." {
		t.Fatalf("unexpected speech text message: %+v", requestBody.Messages[1])
	}
}
