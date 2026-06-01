#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
CGO_ENABLED=0 MINIFLUX_TTS_PROVIDER=fake go test ./internal/tts -run TestGenerateUpdatesEntryEnclosures -count=1
