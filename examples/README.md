# Examples

| Directory | What it shows |
|---|---|
| [`simple/`](./simple) | Minimal struct-routed pages with templ — no HTMX, no DI |
| [`htmx/`](./htmx) | HTMX navigation with `hx-target` + a small `urlFor` wrapper |
| [`htmx-render-target/`](./htmx-render-target) | Standalone-function components shared across pages, driven by `RenderTarget` for per-component data loading |
| [`todo/`](./todo) | Full TODO app: form actions via `ServeHTTP` returning `RenderComponent(...)` to re-render a sibling component |

## Running an example

Each example has its own `go.mod`. From the example directory:

```shell
# Generate templ files first (required)
templ generate -include-version=false

# Run the server (defaults to :8080)
go run .
```

Or use templ's watch mode for live-reloading during development:

```shell
templ generate --watch --proxy="http://localhost:8080" --cmd="go run ."
```

You'll need:
- Go 1.24+
- `templ` CLI: `go install github.com/a-h/templ/cmd/templ@latest`
