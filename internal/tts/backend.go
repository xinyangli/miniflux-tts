package tts

import (
	"context"
	"fmt"
	"io"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

const openAISpeechInputLimit = 4096

type SpeechRequest struct {
	Input string
}

type SpeechResponse struct {
	Audio       []byte
	ContentType string
}

type Backend interface {
	GenerateSpeech(ctx context.Context, request SpeechRequest) (SpeechResponse, error)
}

type FakeBackend struct {
	Audio []byte
}

func (b FakeBackend) GenerateSpeech(_ context.Context, _ SpeechRequest) (SpeechResponse, error) {
	audio := b.Audio
	if len(audio) == 0 {
		audio = []byte("fake mp3 data")
	}
	return SpeechResponse{Audio: audio, ContentType: "audio/mpeg"}, nil
}

type OpenAIBackend struct {
	client openai.Client
	model  string
	voice  string
}

func NewOpenAIBackend(config Config, opts ...option.RequestOption) OpenAIBackend {
	clientOptions := []option.RequestOption{
		option.WithAPIKey(config.OpenAIAPIKey),
		option.WithBaseURL(config.OpenAIBaseURL),
	}
	clientOptions = append(clientOptions, opts...)

	return OpenAIBackend{
		client: openai.NewClient(clientOptions...),
		model:  config.OpenAIModel,
		voice:  config.OpenAIVoice,
	}
}

func (b OpenAIBackend) GenerateSpeech(ctx context.Context, request SpeechRequest) (SpeechResponse, error) {
	resp, err := b.client.Audio.Speech.New(ctx, openai.AudioSpeechNewParams{
		Input: truncateRunes(request.Input, openAISpeechInputLimit),
		Model: openai.SpeechModel(b.model),
		Voice: openai.AudioSpeechNewParamsVoiceUnion{
			OfString: openai.String(b.voice),
		},
		ResponseFormat: openai.AudioSpeechNewParamsResponseFormatMP3,
	})
	if err != nil {
		return SpeechResponse{}, fmt.Errorf("openai speech request failed: %w", err)
	}
	defer resp.Body.Close()

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return SpeechResponse{}, err
	}
	if len(audio) == 0 {
		return SpeechResponse{}, fmt.Errorf("openai speech request returned empty audio")
	}

	return SpeechResponse{Audio: audio, ContentType: "audio/mpeg"}, nil
}

func NewBackend(config Config) (Backend, error) {
	switch config.Provider {
	case "fake":
		return FakeBackend{}, nil
	case "openai":
		return NewOpenAIBackend(config), nil
	default:
		return nil, fmt.Errorf("unknown TTS provider %q", config.Provider)
	}
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
