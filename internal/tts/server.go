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
}

type GenerateResponse struct {
	EntryID int64  `json:"entry_id"`
	URL     string `json:"url"`
	Size    int64  `json:"size"`
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
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/audio/", s.store)
	mux.HandleFunc("POST /tts/{entryID}", s.handleGenerate)
	mux.HandleFunc("OPTIONS /tts/{entryID}", s.handleOptions)
	return s.withCORS(mux)
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

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	entry, err := s.client.EntryContext(ctx, entryID)
	if err != nil {
		s.writeError(w, r, http.StatusBadGateway, "fetch entry", err, entryID)
		return
	}

	input := entryText(entry)
	speech, err := s.backend.GenerateSpeech(ctx, SpeechRequest{Input: input})
	if err != nil {
		s.writeError(w, r, http.StatusBadGateway, "generate speech", err, entryID)
		return
	}

	audioURL, size, err := s.store.Save(entryID, speech.Audio, speech.ContentType)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "store audio", err, entryID)
		return
	}

	enclosures := replaceTTSEnclosure(entry.Enclosures, audioURL, size, s.config.PublicBaseURL, speech.ContentType)
	if _, err := s.client.UpdateEntryContext(ctx, entryID, &miniflux.EntryModificationRequest{Enclosures: &enclosures}); err != nil {
		s.writeError(w, r, http.StatusBadGateway, "update entry enclosures", err, entryID)
		return
	}

	log.Printf("tts generated entry_id=%d input_chars=%d audio_bytes=%d audio_url=%q", entryID, len([]rune(input)), len(speech.Audio), audioURL)
	writeJSON(w, GenerateResponse{EntryID: entryID, URL: audioURL, Size: size})
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
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-TTS-Token")
		}

		next.ServeHTTP(w, r)
	})
}

func replaceTTSEnclosure(existing miniflux.Enclosures, audioURL string, size int64, publicBaseURL string, contentType string) miniflux.Enclosures {
	publicPrefix := strings.TrimRight(publicBaseURL, "/") + "/audio/"
	enclosures := make(miniflux.Enclosures, 0, len(existing)+1)
	for _, enclosure := range existing {
		if enclosure == nil || strings.HasPrefix(enclosure.URL, publicPrefix) {
			continue
		}
		enclosures = append(enclosures, enclosure)
	}

	enclosures = append(enclosures, &miniflux.Enclosure{
		URL:      audioURL,
		MimeType: enclosureMimeType(contentType),
		Size:     int(size),
	})
	return enclosures
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
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
