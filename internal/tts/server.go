package tts

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	miniflux "miniflux.app/v2/client"
)

var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

type Server struct {
	config  Config
	client  *miniflux.Client
	backend Backend
	store   AudioStore
	queue   *JobQueue
}

type JobResponse struct {
	EntryID int64  `json:"entry_id"`
	Status  string `json:"status"`
}

type DeleteResponse struct {
	EntryID int64 `json:"entry_id"`
	Removed int   `json:"removed"`
}

func NewServer(config Config, backend Backend) *Server {
	return &Server{
		config:  config,
		client:  miniflux.NewClient(config.MinifluxBaseURL, config.MinifluxAPIToken),
		backend: backend,
		store: AudioStore{
			Dir:           config.StorageDir,
			PublicBaseURL: config.PublicBaseURL,
		},
		queue: NewJobQueue(),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/audio/", s.store)
	mux.HandleFunc("POST /tts/{entryID}", s.handleGenerate)
	mux.HandleFunc("DELETE /tts/{entryID}", s.handleDelete)
	mux.HandleFunc("OPTIONS /tts/{entryID}", s.handleOptions)
	return s.withCORS(mux)
}

func (s *Server) StartWorkers(ctx context.Context) {
	go func() {
		<-ctx.Done()
		s.queue.Wake()
	}()
	for i := 0; i < s.config.WorkerCount; i++ {
		workerID := i + 1
		go s.runWorker(ctx, workerID)
	}
}

func (s *Server) handleOptions(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if s.config.BrowserToken != "" && r.Header.Get("X-TTS-Token") != s.config.BrowserToken {
		s.writeError(w, r, http.StatusUnauthorized, "invalid TTS token", nil, 0)
		return
	}

	entryID, err := strconv.ParseInt(r.PathValue("entryID"), 10, 64)
	if err != nil || entryID <= 0 {
		s.writeError(w, r, http.StatusBadRequest, "invalid entry ID", err, 0)
		return
	}

	status := s.queue.Enqueue(entryID)
	writeJSONStatus(w, http.StatusAccepted, JobResponse{EntryID: entryID, Status: string(status)})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if s.config.BrowserToken != "" && r.Header.Get("X-TTS-Token") != s.config.BrowserToken {
		s.writeError(w, r, http.StatusUnauthorized, "invalid TTS token", nil, 0)
		return
	}

	entryID, err := strconv.ParseInt(r.PathValue("entryID"), 10, 64)
	if err != nil || entryID <= 0 {
		s.writeError(w, r, http.StatusBadRequest, "invalid entry ID", err, 0)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	entry, err := s.client.EntryContext(ctx, entryID)
	if err != nil {
		s.writeError(w, r, http.StatusBadGateway, "fetch entry", err, entryID)
		return
	}

	kept, removed := splitTTSEnclosures(entry.Enclosures, s.config.PublicBaseURL)
	for _, enclosure := range removed {
		if err := s.store.DeleteURL(enclosure.URL); err != nil {
			s.writeError(w, r, http.StatusInternalServerError, "delete audio", err, entryID)
			return
		}
	}

	if len(removed) > 0 {
		if _, err := s.client.UpdateEntryContext(ctx, entryID, &miniflux.EntryModificationRequest{Enclosures: &kept}); err != nil {
			s.writeError(w, r, http.StatusBadGateway, "update entry enclosures", err, entryID)
			return
		}
	}

	writeJSON(w, DeleteResponse{EntryID: entryID, Removed: len(removed)})
}

func (s *Server) runWorker(ctx context.Context, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		entryID, ok := s.queue.Next(ctx)
		if !ok {
			return
		}
		jobCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
		err := s.processJob(jobCtx, entryID)
		cancel()
		s.queue.Complete(entryID)
		if err != nil {
			log.Printf("tts worker failed worker_id=%d entry_id=%d error=%v", workerID, entryID, err)
			continue
		}
		log.Printf("tts worker completed worker_id=%d entry_id=%d", workerID, entryID)
	}
}

func (s *Server) processJob(ctx context.Context, entryID int64) error {
	entry, err := s.client.EntryContext(ctx, entryID)
	if err != nil {
		return fmt.Errorf("fetch entry: %w", err)
	}

	if hasTTSEnclosure(entry.Enclosures, s.config.PublicBaseURL) {
		log.Printf("tts skip existing enclosure entry_id=%d", entryID)
		return nil
	}

	input := entryText(entry)
	speech, err := s.backend.GenerateSpeech(ctx, SpeechRequest{Input: input})
	if err != nil {
		return fmt.Errorf("generate speech: %w", err)
	}

	audioURL, size, err := s.store.Save(entryID, speech.Audio, speech.ContentType)
	if err != nil {
		return fmt.Errorf("store audio: %w", err)
	}

	enclosures := replaceTTSEnclosure(entry.Enclosures, audioURL, size, s.config.PublicBaseURL, speech.ContentType)
	if _, err := s.client.UpdateEntryContext(ctx, entryID, &miniflux.EntryModificationRequest{Enclosures: &enclosures}); err != nil {
		return fmt.Errorf("update entry enclosures: %w", err)
	}

	log.Printf("tts generated entry_id=%d input_chars=%d audio_bytes=%d audio_url=%q", entryID, len([]rune(input)), len(speech.Audio), audioURL)
	return nil
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, status int, message string, err error, entryID int64) {
	if err != nil {
		log.Printf("tts request failed status=%d method=%s path=%q entry_id=%d message=%q error=%v", status, r.Method, r.URL.Path, entryID, message, err)
		http.Error(w, fmt.Sprintf("%s: %v", message, err), status)
		return
	}

	log.Printf("tts request failed status=%d method=%s path=%q entry_id=%d message=%q", status, r.Method, r.URL.Path, entryID, message)
	http.Error(w, message, status)
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if s.config.AllowedOrigin != "" && origin == s.config.AllowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-TTS-Token")
		}

		next.ServeHTTP(w, r)
	})
}

func replaceTTSEnclosure(existing miniflux.Enclosures, audioURL string, size int64, publicBaseURL string, contentType string) miniflux.Enclosures {
	enclosures, _ := splitTTSEnclosures(existing, publicBaseURL)

	enclosures = append(enclosures, &miniflux.Enclosure{
		URL:      audioURL,
		MimeType: enclosureMimeType(contentType),
		Size:     int(size),
	})
	return enclosures
}

func splitTTSEnclosures(existing miniflux.Enclosures, publicBaseURL string) (miniflux.Enclosures, miniflux.Enclosures) {
	kept := make(miniflux.Enclosures, 0, len(existing))
	removed := make(miniflux.Enclosures, 0)
	for _, enclosure := range existing {
		if enclosure == nil {
			continue
		}
		if isTTSEnclosureURL(enclosure.URL, publicBaseURL) {
			removed = append(removed, enclosure)
			continue
		}
		kept = append(kept, enclosure)
	}
	return kept, removed
}

func hasTTSEnclosure(existing miniflux.Enclosures, publicBaseURL string) bool {
	for _, enclosure := range existing {
		if enclosure != nil && isTTSEnclosureURL(enclosure.URL, publicBaseURL) {
			return true
		}
	}
	return false
}

func isTTSEnclosureURL(url string, publicBaseURL string) bool {
	publicPrefix := strings.TrimRight(publicBaseURL, "/") + "/audio/"
	return strings.HasPrefix(url, publicPrefix)
}

func enclosureMimeType(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil && mediaType != "" {
		return mediaType
	}
	return "audio/mpeg"
}

func entryText(entry *miniflux.Entry) string {
	if entry == nil {
		return ""
	}
	parts := []string{entry.Title, entry.Content}
	text := strings.Join(parts, "\n\n")
	text = htmlTagPattern.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	return strings.Join(strings.Fields(text), " ")
}

func writeJSON(w http.ResponseWriter, value any) {
	writeJSONStatus(w, http.StatusOK, value)
}

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
