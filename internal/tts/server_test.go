package tts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	miniflux "miniflux.app/v2/client"
)

func TestGenerateUpdatesEntryEnclosures(t *testing.T) {
	var updated miniflux.EntryModificationRequest
	minifluxAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Auth-Token") != "mf-token" {
			t.Fatalf("missing Miniflux API token")
		}

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/entries/123":
			writeJSON(w, &miniflux.Entry{
				ID:      123,
				Title:   "Entry title",
				Content: "<p>Hello <strong>world</strong></p>",
				Enclosures: miniflux.Enclosures{
					{URL: "https://example.org/original.mp3", MimeType: "audio/mpeg", Size: 100},
					{URL: "http://tts.test/audio/old.mp3", MimeType: "audio/mpeg", Size: 50},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/entries/123":
			if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
				t.Fatal(err)
			}
			writeJSON(w, &miniflux.Entry{ID: 123})
		default:
			http.NotFound(w, r)
		}
	}))
	defer minifluxAPI.Close()

	config := Config{
		MinifluxBaseURL:  minifluxAPI.URL,
		MinifluxAPIToken: "mf-token",
		PublicBaseURL:    "http://tts.test",
		StorageDir:       t.TempDir(),
		AllowedOrigin:    "http://miniflux.test",
		BrowserToken:     "browser-token",
	}
	server := NewServer(config, assertInputBackend{t: t, want: "Entry title Hello world", audio: []byte("mp3")})

	req := httptest.NewRequest(http.MethodPost, "/tts/123", nil)
	req.Header.Set("Origin", "http://miniflux.test")
	req.Header.Set("X-TTS-Token", "browser-token")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://miniflux.test" {
		t.Fatalf("missing CORS allow origin")
	}
	if updated.Enclosures == nil {
		t.Fatal("expected enclosures update")
	}
	if len(*updated.Enclosures) != 2 {
		t.Fatalf("expected original enclosure plus new TTS enclosure, got %d", len(*updated.Enclosures))
	}
	if (*updated.Enclosures)[0].URL != "https://example.org/original.mp3" {
		t.Fatalf("non-TTS enclosure was not preserved: %+v", (*updated.Enclosures)[0])
	}
	if !strings.HasPrefix((*updated.Enclosures)[1].URL, "http://tts.test/audio/") {
		t.Fatalf("new enclosure URL does not use public audio URL: %+v", (*updated.Enclosures)[1])
	}
	if (*updated.Enclosures)[1].MimeType != "audio/mpeg" {
		t.Fatalf("expected audio/mpeg enclosure: %+v", (*updated.Enclosures)[1])
	}
}

type assertInputBackend struct {
	t     *testing.T
	want  string
	audio []byte
}

func (b assertInputBackend) GenerateSpeech(_ context.Context, request SpeechRequest) (SpeechResponse, error) {
	b.t.Helper()
	if request.Input != b.want {
		b.t.Fatalf("expected speech input %q, got %q", b.want, request.Input)
	}
	return SpeechResponse{Audio: b.audio, ContentType: "audio/mpeg"}, nil
}

func TestGenerateRejectsInvalidBrowserToken(t *testing.T) {
	config := Config{
		MinifluxBaseURL:  "http://miniflux.test",
		MinifluxAPIToken: "mf-token",
		PublicBaseURL:    "http://tts.test",
		StorageDir:       t.TempDir(),
		BrowserToken:     "browser-token",
	}
	server := NewServer(config, FakeBackend{})

	req := httptest.NewRequest(http.MethodPost, "/tts/123", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAudioStoreServesSavedMP3(t *testing.T) {
	store := AudioStore{Dir: t.TempDir(), PublicBaseURL: "http://tts.test"}
	audioURL, _, err := store.Save(123, []byte("mp3"))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, strings.TrimPrefix(audioURL, "http://tts.test"), nil)
	rec := httptest.NewRecorder()
	store.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "mp3" {
		t.Fatalf("expected stored audio, got %q", rec.Body.String())
	}
	if filepath.Base(strings.TrimPrefix(audioURL, "http://tts.test/audio/")) == "" {
		t.Fatalf("expected generated filename in URL")
	}
}
