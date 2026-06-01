package tts

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
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

func (s AudioStore) Save(entryID int64, audio []byte) (string, int64, error) {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return "", 0, err
	}

	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", 0, err
	}

	name := strconv.FormatInt(entryID, 10) + "-" + hex.EncodeToString(random) + ".mp3"
	path := filepath.Join(s.Dir, name)
	if err := os.WriteFile(path, audio, 0o644); err != nil {
		return "", 0, err
	}

	return strings.TrimRight(s.PublicBaseURL, "/") + "/audio/" + name, int64(len(audio)), nil
}

func (s AudioStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/audio/")
	if name == "" || name != filepath.Base(name) || !strings.HasSuffix(name, ".mp3") {
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

	w.Header().Set("Content-Type", "audio/mpeg")
	http.ServeContent(w, r, name, stat.ModTime(), file)
}
