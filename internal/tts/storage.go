package tts

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type AudioStore struct {
	Dir           string
	PublicBaseURL string
}

func (s AudioStore) Save(entryID int64, audio []byte, contentType string) (string, int64, error) {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return "", 0, err
	}

	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", 0, err
	}

	name := strconv.FormatInt(entryID, 10) + "-" + hex.EncodeToString(random) + audioExtension(contentType)
	path := filepath.Join(s.Dir, name)
	if err := os.WriteFile(path, audio, 0o644); err != nil {
		return "", 0, err
	}

	return strings.TrimRight(s.PublicBaseURL, "/") + "/audio/" + name, int64(len(audio)), nil
}

func (s AudioStore) DeleteURL(audioURL string) error {
	prefix := strings.TrimRight(s.PublicBaseURL, "/") + "/audio/"
	if !strings.HasPrefix(audioURL, prefix) {
		return nil
	}

	name := strings.TrimPrefix(audioURL, prefix)
	if name == "" || name != filepath.Base(name) || audioContentTypeForName(name) == "" {
		return nil
	}

	err := os.Remove(filepath.Join(s.Dir, name))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s AudioStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/audio/")
	if name == "" || name != filepath.Base(name) || audioContentTypeForName(name) == "" {
		http.NotFound(w, r)
		return
	}

	path := filepath.Join(s.Dir, name)
	file, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		http.Error(w, fmt.Sprintf("stat audio: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", audioContentTypeForName(name))
	http.ServeContent(w, r, name, stat.ModTime(), file)
}

func audioExtension(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ".mp3"
	}
	switch mediaType {
	case "audio/aac":
		return ".aac"
	case "audio/flac":
		return ".flac"
	case "audio/mpeg":
		return ".mp3"
	case "audio/opus":
		return ".opus"
	case "audio/pcm":
		return ".pcm"
	case "audio/wav", "audio/wave", "audio/x-wav":
		return ".wav"
	default:
		return ".mp3"
	}
}

func audioContentTypeForName(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".aac":
		return "audio/aac"
	case ".flac":
		return "audio/flac"
	case ".mp3":
		return "audio/mpeg"
	case ".opus":
		return "audio/opus"
	case ".pcm":
		return "audio/pcm"
	case ".wav":
		return "audio/wav"
	default:
		return ""
	}
}
