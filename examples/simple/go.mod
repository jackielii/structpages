module github.com/jackielii/structpages/examples/simple

go 1.26.1

require github.com/jackielii/structpages v0.0.0-00010101000000-000000000000

require github.com/a-h/templ v0.3.1020 // indirect

require (
	github.com/a-h/parse v0.0.0-20250122154542-74294addb73e // indirect
	github.com/andybalholm/brotli v1.1.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cli/browser v1.3.0 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/gsxhq/gsx v0.0.0
	github.com/jackielii/ctxkey v1.0.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/natefinch/atomic v1.0.1 // indirect
	golang.org/x/mod v0.37.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/tools v0.46.0 // indirect
)

replace github.com/jackielii/structpages => ../..

tool github.com/a-h/templ/cmd/templ

replace github.com/gsxhq/gsx => ../../../gsxhq/gsx
