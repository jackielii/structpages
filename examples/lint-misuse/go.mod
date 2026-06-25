module github.com/jackielii/structpages/examples/lint-misuse

go 1.26.1

require github.com/jackielii/structpages v0.0.0-00010101000000-000000000000

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/tdewolff/parse/v2 v2.8.13 // indirect
	golang.org/x/mod v0.37.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/tools v0.46.0 // indirect
)

require (
	github.com/gsxhq/gsx v0.0.0
	github.com/jackielii/ctxkey v1.0.1 // indirect
)

replace github.com/jackielii/structpages => ../..

replace github.com/gsxhq/gsx => /Users/jackieli/personal/gsxhq/gsx

tool github.com/gsxhq/gsx/cmd/gsx
