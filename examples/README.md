# Examples

| Directory | What it shows |
|---|---|
| [`simple/`](./simple) | Minimal struct-routed pages with gsx — no HTMX, no DI |
| [`html-template/`](./html-template) | Standard library `html/template` in an atomic-design layout (atoms / molecules / organisms), htmx 4 partial swaps, and a no-Clone `urlFor` template func wired up in user code |
| [`htmx/`](./htmx) | HTMX navigation with `hx-target` + a small `urlFor` wrapper |
| [`htmx-render-target/`](./htmx-render-target) | Standalone-function components shared across pages, driven by `RenderTarget` for per-component data loading |
| [`todo/`](./todo) | Full TODO app: form actions via `ServeHTTP` returning `RenderComponent(...)` to re-render a sibling component |
| [`lint-misuse/`](./lint-misuse) | Deliberate structpages misuse patterns pinned as targets for the structpages-lint rules (see lint_test.go) |
| [`url-validation/`](./url-validation) | Route/URLFor validation exercised by an integration test — standard library only, no gsx |
| [`blog/`](./blog) | Comprehensive blog + admin CMS with React-style per-feature packages, DI, page-level `Middlewares`, `Props` + `RenderTarget` widgets, custom error handler, cross-package component composition |

## Running an example

Each example has its own `go.mod`, and the generated `.x.go` files are committed — so from
the example directory:

```shell
go run .   # serves on :8080
```

If you edit `.gsx` sources, regenerate before running. Each example pins the gsx CLI as a
`tool` dependency in its `go.mod`, so `go tool` resolves the right version automatically:

```shell
go tool gsx generate .
go tool gsx fmt -w .   # canonical formatting
```

You'll need Go 1.26+. `html-template/` and `url-validation/` use only the standard library —
no gsx involved.
