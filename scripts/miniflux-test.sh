#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TMPDIR="$(mktemp -d)"
PGDATA="$TMPDIR/postgres"
PGRUN="$TMPDIR/run"
ADMIN_USERNAME="admin"
ADMIN_PASSWORD="miniflux-password"
DB_NAME="miniflux2"

cleanup() {
    set +e
    if [[ -n "${MINIFLUX_PID:-}" ]]; then kill "$MINIFLUX_PID" 2>/dev/null; fi
    if [[ -n "${FIXTURE_PID:-}" ]]; then kill "$FIXTURE_PID" 2>/dev/null; fi
    if [[ -d "$PGDATA" ]]; then pg_ctl -D "$PGDATA" -m fast stop >/dev/null 2>&1; fi
    rm -rf "$TMPDIR"
}
trap cleanup EXIT

free_port() {
    for _ in $(seq 1 50); do
        port="$(shuf -i 20000-45000 -n 1)"
        if ! nc -z 127.0.0.1 "$port" >/dev/null 2>&1; then
            echo "$port"
            return 0
        fi
    done
    echo "unable to find a free port" >&2
    return 1
}

PGPORT="$(free_port)"
MINIFLUX_PORT="$(free_port)"
FIXTURE_PORT="$(free_port)"
mkdir -p "$PGRUN"

initdb -D "$PGDATA" -A trust --username "$USER" >/dev/null
pg_ctl -D "$PGDATA" -o "-k $PGRUN -p $PGPORT -h 127.0.0.1" -w start >/dev/null
createdb -h "$PGRUN" -p "$PGPORT" "$DB_NAME"

cat > "$TMPDIR/fixture.go" <<'GO'
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	addr := os.Args[1]
	baseURL := "http://" + addr

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Fixture</title><link rel="alternate" type="application/rss+xml" title="Miniflux Releases" href="%s/feed.xml"></head><body>Fixture</body></html>`, baseURL)
	})
	http.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Miniflux</title>
    <link>%s/</link>
    <description>Fixture feed</description>
    <item>
      <title>Fixture entry</title>
      <link>%s/articles/1</link>
      <guid>fixture-entry-1</guid>
      <description><![CDATA[<p>Hello from the fixture feed with version 2.0.8.</p>]]></description>
      <pubDate>Mon, 01 Jun 2026 00:00:00 GMT</pubDate>
      <enclosure url="%s/audio/original.mp3" type="audio/mpeg" length="12345"/>
    </item>
    <item>
      <title>Another fixture entry</title>
      <link>%s/articles/2</link>
      <guid>fixture-entry-2</guid>
      <description><![CDATA[<p>More local fixture content.</p>]]></description>
      <pubDate>Sun, 31 May 2026 00:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`, baseURL, baseURL, baseURL, baseURL)
	})
	http.HandleFunc("/audio/original.mp3", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("fixture mp3"))
	})

	log.Fatal(http.ListenAndServe(addr, nil))
}
GO

CGO_ENABLED=0 go run "$TMPDIR/fixture.go" "127.0.0.1:$FIXTURE_PORT" > "$TMPDIR/fixture.log" 2>&1 &
FIXTURE_PID=$!

for _ in $(seq 1 50); do
    if curl -fsS "http://127.0.0.1:$FIXTURE_PORT/feed.xml" >/dev/null; then break; fi
    sleep 0.1
done
if ! curl -fsS "http://127.0.0.1:$FIXTURE_PORT/feed.xml" >/dev/null; then
    cat "$TMPDIR/fixture.log" >&2
    echo "fixture server did not start" >&2
    exit 1
fi

cd "$ROOT_DIR/v2"
DATABASE_URL="user=$USER dbname=$DB_NAME host=$PGRUN port=$PGPORT sslmode=disable" \
RUN_MIGRATIONS=1 \
CREATE_ADMIN=1 \
FETCHER_ALLOW_PRIVATE_NETWORKS=1 \
ADMIN_USERNAME="$ADMIN_USERNAME" \
ADMIN_PASSWORD="$ADMIN_PASSWORD" \
LISTEN_ADDR="127.0.0.1:$MINIFLUX_PORT" \
BASE_URL="http://127.0.0.1:$MINIFLUX_PORT" \
CGO_ENABLED=0 \
go run . > "$TMPDIR/miniflux.log" 2>&1 &
MINIFLUX_PID=$!

for _ in $(seq 1 120); do
    if curl -fsS "http://127.0.0.1:$MINIFLUX_PORT/healthcheck" >/dev/null; then break; fi
    sleep 0.25
done
if ! curl -fsS "http://127.0.0.1:$MINIFLUX_PORT/healthcheck" >/dev/null; then
    cat "$TMPDIR/miniflux.log" >&2
    echo "miniflux server did not start" >&2
    exit 1
fi

TEST_MINIFLUX_BASE_URL="http://127.0.0.1:$MINIFLUX_PORT" \
TEST_MINIFLUX_ADMIN_USERNAME="$ADMIN_USERNAME" \
TEST_MINIFLUX_ADMIN_PASSWORD="$ADMIN_PASSWORD" \
TEST_MINIFLUX_FEED_URL="http://127.0.0.1:$FIXTURE_PORT/feed.xml" \
TEST_MINIFLUX_FEED_TITLE="Miniflux" \
TEST_MINIFLUX_SUBSCRIPTION_TITLE="Miniflux Releases" \
TEST_MINIFLUX_WEBSITE_URL="http://127.0.0.1:$FIXTURE_PORT/" \
CGO_ENABLED=0 \
go test ./internal/api
