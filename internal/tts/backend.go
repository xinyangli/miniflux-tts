package tts

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

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
	client       openai.Client
	model        string
	voice        string
	format       string
	instructions string
}

func NewOpenAIBackend(config Config, opts ...option.RequestOption) OpenAIBackend {
	model := config.OpenAIModel
	if model == "" {
		model = "gpt-audio-1.5"
	}
	voice := config.OpenAIVoice
	if voice == "" {
		voice = "alloy"
	}
	format := config.OpenAIFormat
	if format == "" {
		format = "wav"
	}

	clientOptions := []option.RequestOption{
		option.WithAPIKey(config.OpenAIAPIKey),
		option.WithBaseURL(config.OpenAIBaseURL),
	}
	clientOptions = append(clientOptions, opts...)

	return OpenAIBackend{
		client:       openai.NewClient(clientOptions...),
		model:        model,
		voice:        voice,
		format:       format,
		instructions: config.OpenAIInstructions,
	}
}

func (b OpenAIBackend) GenerateSpeech(ctx context.Context, request SpeechRequest) (SpeechResponse, error) {
	completion, err := b.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModel(b.model),
		Audio: openai.ChatCompletionAudioParam{
			Format: openai.ChatCompletionAudioParamFormat(b.format),
			Voice: openai.ChatCompletionAudioParamVoiceUnion{
				OfString: openai.String(b.voice),
			},
		},
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(b.instructions),
			openai.AssistantMessage(request.Input),
		},
		Modalities: []string{"text", "audio"},
	})
	if err != nil {
		return SpeechResponse{}, fmt.Errorf("openai chat completion speech request failed: %w", err)
	}
	if len(completion.Choices) == 0 || completion.Choices[0].Message.Audio.Data == "" {
		return SpeechResponse{}, fmt.Errorf("openai chat completion response did not include audio data")
	}

	audio, err := base64.StdEncoding.DecodeString(completion.Choices[0].Message.Audio.Data)
	if err != nil {
		return SpeechResponse{}, fmt.Errorf("decode chat completion audio data: %w", err)
	}
	if len(audio) == 0 {
		return SpeechResponse{}, fmt.Errorf("openai chat completion returned empty audio")
	}

	return SpeechResponse{Audio: audio, ContentType: audioContentTypeFromFormat(b.format)}, nil
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

func audioContentTypeFromFormat(format string) string {
	switch format {
	case "aac":
		return "audio/aac"
	case "flac":
		return "audio/flac"
	case "mp3":
		return "audio/mpeg"
	case "opus":
		return "audio/opus"
	case "pcm", "pcm16":
		return "audio/pcm"
	case "wav":
		return "audio/wav"
	default:
		return "audio/mpeg"
	}
}
