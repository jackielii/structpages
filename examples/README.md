# Examples

| Directory | What it shows |
|---|---|
| [`simple/`](./simple) | Minimal struct-routed pages with gsx — no HTMX, no DI |
| [`html-template/`](./html-template) | Standard library `html/template` in an atomic-design layout (atoms / molecules / organisms), htmx 4 partial swaps, and a no-Clone `urlFor` template func wired up in user code |
| [`htmx/`](./htmx) | HTMX navigation with `hx-target` + a small `urlFor` wrapper |
| [`htmx-render-target/`](./htmx-render-target) | Standalone-function components shared across pages, driven by `RenderTarget` for per-component data loading |
| [`todo/`](./todo) | Full TODO app: form actions via `ServeHTTP` returning `RenderComponent(...)` to re-render a sibling component |
| [`blog/`](./blog) | Comprehensive blog + admin CMS with React-style per-feature packages, DI, page-level `Middlewares`, `Props` + `RenderTarget` widgets, custom error handler, cross-package component composition |

## Running an example

Each example has its own `go.mod`, and the generated `.x.go` files are committed — so from
the example directory:

```shell
go run .   # serves on :8080
```

If you edit `.gsx` sources, regenerate before running (uses the gsx version pinned in the
example's `go.mod`):

```shell
go run github.com/gsxhq/gsx/cmd/gsx generate .
```

You'll need Go 1.26+. `html-template/` and `url-validation/` use only the standard library —
no gsx involved.
