package tts

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
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
		http.Error(w, "invalid TTS token", http.StatusUnauthorized)
		return
	}

	entryID, err := strconv.ParseInt(r.PathValue("entryID"), 10, 64)
	if err != nil || entryID <= 0 {
		http.Error(w, "invalid entry ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	entry, err := s.client.EntryContext(ctx, entryID)
	if err != nil {
		http.Error(w, fmt.Sprintf("fetch entry: %v", err), http.StatusBadGateway)
		return
	}

	speech, err := s.backend.GenerateSpeech(ctx, SpeechRequest{Input: entryText(entry)})
	if err != nil {
		http.Error(w, fmt.Sprintf("generate speech: %v", err), http.StatusBadGateway)
		return
	}

	audioURL, size, err := s.store.Save(entryID, speech.Audio)
	if err != nil {
		http.Error(w, fmt.Sprintf("store audio: %v", err), http.StatusInternalServerError)
		return
	}

	enclosures := replaceTTSEnclosure(entry.Enclosures, audioURL, size, s.config.PublicBaseURL, speech.ContentType)
	if _, err := s.client.UpdateEntryContext(ctx, entryID, &miniflux.EntryModificationRequest{Enclosures: &enclosures}); err != nil {
		http.Error(w, fmt.Sprintf("update entry enclosures: %v", err), http.StatusBadGateway)
		return
	}

	writeJSON(w, GenerateResponse{EntryID: entryID, URL: audioURL, Size: size})
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
