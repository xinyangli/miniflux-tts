module miniflux-tts

go 1.26.0

require (
	github.com/openai/openai-go/v3 v3.37.0
	miniflux.app/v2 v2.0.0
)

require (
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
)

replace miniflux.app/v2 => ./v2
