module github.com/jackielii/structpages/examples/blog-admin

go 1.24.0

toolchain go1.24.3

replace github.com/jackielii/structpages => ../..

require (
	github.com/alexedwards/scs/v2 v2.8.0
	github.com/go-playground/form/v4 v4.2.1
	github.com/jackielii/structpages v0.0.0-00010101000000-000000000000
	github.com/mattn/go-sqlite3 v1.14.24
	golang.org/x/crypto v0.31.0
)

require (
	github.com/a-h/templ v0.3.898 // indirect
	github.com/jackielii/ctxkey v1.0.1 // indirect
)
