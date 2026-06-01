# Miniflux TTS Integration

This workspace tracks Miniflux as the `v2` submodule from `https://github.com/miniflux/v2.git`.
The older `github.com/miniflux/miniflux` repository path redirects to the same project and is treated as an alias.

## Service

Run the TTS integration service with:

```sh
MINIFLUX_BASE_URL=http://localhost:8080 \
MINIFLUX_API_TOKEN=... \
PUBLIC_BASE_URL=http://localhost:8090 \
ALLOWED_MINIFLUX_ORIGIN=http://localhost:8080 \
TTS_BROWSER_TOKEN=... \
MINIFLUX_TTS_OPENAI_API_KEY=... \
go run ./cmd/miniflux-tts
```

Or build/run the Nix package:

```sh
nix build .#miniflux-tts
nix run .#miniflux-tts
```

For local tests, use the fake provider:

```sh
MINIFLUX_TTS_PROVIDER=fake go test ./...
```

The Custom JS snippet in `custom-js/miniflux-tts.js` calls `POST /tts/{entryID}` directly and sends `X-TTS-Token` when configured.
