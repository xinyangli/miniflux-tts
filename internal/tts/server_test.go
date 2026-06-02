package tts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	miniflux "miniflux.app/v2/client"
)

func TestProcessJobUpdatesEntryEnclosures(t *testing.T) {
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

	if err := server.processJob(context.Background(), 123); err != nil {
		t.Fatal(err)
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

func TestGenerateEnqueuesWithoutSynchronousGeneration(t *testing.T) {
	minifluxAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("POST should not call Miniflux synchronously: %s %s", r.Method, r.URL.Path)
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
	server := NewServer(config, failBackend{t: t})

	req := httptest.NewRequest(http.MethodPost, "/tts/123", nil)
	req.Header.Set("Origin", "http://miniflux.test")
	req.Header.Set("X-TTS-Token", "browser-token")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://miniflux.test" {
		t.Fatalf("missing CORS allow origin")
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Methods"), "DELETE") {
		t.Fatalf("expected DELETE in CORS methods, got %q", rec.Header().Get("Access-Control-Allow-Methods"))
	}
	var response JobResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.EntryID != 123 || response.Status != string(JobStatusQueued) {
		t.Fatalf("unexpected enqueue response: %+v", response)
	}
}

func TestGenerateReturnsExistingQueuedAndRunningStatus(t *testing.T) {
	config := Config{
		MinifluxBaseURL:  "http://miniflux.test",
		MinifluxAPIToken: "mf-token",
		PublicBaseURL:    "http://tts.test",
		StorageDir:       t.TempDir(),
	}
	server := NewServer(config, failBackend{t: t})

	for _, want := range []string{string(JobStatusQueued), string(JobStatusQueued)} {
		req := httptest.NewRequest(http.MethodPost, "/tts/123", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
		}
		var response JobResponse
		if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
			t.Fatal(err)
		}
		if response.Status != want {
			t.Fatalf("expected %q status, got %+v", want, response)
		}
	}

	if _, ok := server.queue.Next(context.Background()); !ok {
		t.Fatal("expected queued job")
	}
	req := httptest.NewRequest(http.MethodPost, "/tts/123", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	var response JobResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Status != string(JobStatusRunning) {
		t.Fatalf("expected running status, got %+v", response)
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

type failBackend struct {
	t *testing.T
}

func (b failBackend) GenerateSpeech(_ context.Context, _ SpeechRequest) (SpeechResponse, error) {
	b.t.Helper()
	b.t.Fatal("backend should not be called")
	return SpeechResponse{}, nil
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

func TestProcessJobSkipsExistingTTSEnclosure(t *testing.T) {
	minifluxAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/entries/123":
			writeJSON(w, &miniflux.Entry{
				ID:    123,
				Title: "Entry title",
				Enclosures: miniflux.Enclosures{
					{URL: "http://tts.test/audio/existing.mp3", MimeType: "audio/mpeg", Size: 50},
				},
			})
		default:
			t.Fatalf("unexpected Miniflux request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer minifluxAPI.Close()

	config := Config{
		MinifluxBaseURL:  minifluxAPI.URL,
		MinifluxAPIToken: "mf-token",
		PublicBaseURL:    "http://tts.test",
		StorageDir:       t.TempDir(),
	}
	server := NewServer(config, failBackend{t: t})

	if err := server.processJob(context.Background(), 123); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteRemovesTTSEnclosuresAndLocalAudio(t *testing.T) {
	storageDir := t.TempDir()
	audioPath := filepath.Join(storageDir, "123-test.mp3")
	if err := os.WriteFile(audioPath, []byte("mp3"), 0o644); err != nil {
		t.Fatal(err)
	}

	var updated miniflux.EntryModificationRequest
	minifluxAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/entries/123":
			writeJSON(w, &miniflux.Entry{
				ID: 123,
				Enclosures: miniflux.Enclosures{
					{URL: "https://example.org/original.mp3", MimeType: "audio/mpeg", Size: 100},
					{URL: "http://tts.test/audio/123-test.mp3", MimeType: "audio/mpeg", Size: 50},
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
		StorageDir:       storageDir,
		BrowserToken:     "browser-token",
	}
	server := NewServer(config, FakeBackend{})
	server.queue.Enqueue(123)

	req := httptest.NewRequest(http.MethodDelete, "/tts/123", nil)
	req.Header.Set("X-TTS-Token", "browser-token")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response DeleteResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.EntryID != 123 || response.Removed != 1 {
		t.Fatalf("unexpected delete response: %+v", response)
	}
	if _, err := os.Stat(audioPath); !os.IsNotExist(err) {
		t.Fatalf("expected local audio file to be deleted, stat error: %v", err)
	}
	if updated.Enclosures == nil || len(*updated.Enclosures) != 1 {
		t.Fatalf("expected one preserved enclosure, got %+v", updated.Enclosures)
	}
	if (*updated.Enclosures)[0].URL != "https://example.org/original.mp3" {
		t.Fatalf("expected original enclosure to be preserved, got %+v", (*updated.Enclosures)[0])
	}
	if status, ok := server.queue.Status(123); !ok || status != JobStatusQueued {
		t.Fatalf("expected delete to leave queued job unchanged, got status=%q ok=%v", status, ok)
	}
}

func TestAudioStoreServesSavedMP3(t *testing.T) {
	store := AudioStore{Dir: t.TempDir(), PublicBaseURL: "http://tts.test"}
	audioURL, _, err := store.Save(123, []byte("mp3"), "audio/mpeg")
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

func TestAudioStoreServesSavedWAV(t *testing.T) {
	store := AudioStore{Dir: t.TempDir(), PublicBaseURL: "http://tts.test"}
	audioURL, _, err := store.Save(123, []byte("wav"), "audio/wav")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, strings.TrimPrefix(audioURL, "http://tts.test"), nil)
	rec := httptest.NewRecorder()
	store.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "audio/wav" {
		t.Fatalf("expected audio/wav content type, got %q", rec.Header().Get("Content-Type"))
	}
	if !strings.HasSuffix(audioURL, ".wav") {
		t.Fatalf("expected wav URL, got %q", audioURL)
	}
}
