# Examples

| Directory | What it shows |
|---|---|
| [`simple/`](./simple) | Minimal struct-routed pages with templ — no HTMX, no DI |
| [`html-template/`](./html-template) | Standard library `html/template` in an atomic-design layout (atoms / molecules / organisms), htmx 4 partial swaps, and a no-Clone `urlFor` template func wired up in user code |
| [`htmx/`](./htmx) | HTMX navigation with `hx-target` + a small `urlFor` wrapper |
| [`htmx-render-target/`](./htmx-render-target) | Standalone-function components shared across pages, driven by `RenderTarget` for per-component data loading |
| [`todo/`](./todo) | Full TODO app: form actions via `ServeHTTP` returning `RenderComponent(...)` to re-render a sibling component |
| [`blog/`](./blog) | Comprehensive blog + admin CMS with React-style per-feature packages, DI, page-level `Middlewares`, `Props` + `RenderTarget` widgets, custom error handler, cross-package component composition |

## Running an example

Each example has its own `go.mod`. From the example directory:

```shell
# Generate the .x.go files from .gsx sources first (required)
gsx generate .

# Run the server (defaults to :8080)
go run .
```

You'll need:
- Go 1.24+
- `gsx` CLI: `go install github.com/gsxhq/gsx/cmd/gsx@latest` (not required for `html-template/`, which uses only the standard library)
