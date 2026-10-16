# Examples

Read them in this order; each builds on the one before. The entry file is where the pages are defined.

| Directory | What it shows | Start in |
|---|---|---|
| [`simple/`](./simple) | Route-tagged page structs, one `Page` component each, and links built with the `url` filter. No HTMX, no DI. | `pages.gsx` |
| [`htmx/`](./htmx) | A `Page` and a `Content` component per page: mounted with `WithTargetSelector(structpages.HTMXRenderTarget)`, so an htmx request renders just `Content` when `hx-target` names it. | `pages.gsx` |
| [`todo/`](./todo) | Form actions as handler methods (`ServeHTTP`) that mutate, then return `RenderComponent(...)` to re-render the list. | `pages.gsx` |
| [`htmx-render-target/`](./htmx-render-target) | Standalone component functions as HTMX targets: `Props` asks `target.Is(...)`, loads only that widget's data and renders it with `RenderComponent`. | `pages.gsx` |
| [`blog/`](./blog) | A larger app: per-feature packages, `WithArgs` dependency injection, page `Middlewares` for auth, a custom `WithErrorHandler`, components shared across packages. | `main.go`, then `blog/` and `admin/` |
| [`url-validation/`](./url-validation) | Checking every generated URL at boot, plus an integration test that asserts rendered `href`s. Standard library only. | `validate.go` |
| [`html-template/`](./html-template) | Rendering with `html/template` instead of gsx, with a `urlFor` template func and htmx 4 via `HTMXv4RenderTarget`. | `main.go` |

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

You'll need Go 1.26+. The repository-root `gsx.toml` registers structpages' `url`, `id` and
`target` filters for every example. `html-template/` and `url-validation/` use only the standard
library — no gsx involved.

---

[`lint-misuse/`](./lint-misuse) isn't an app to learn from: it's the test fixture for `structpages-lint`, a set of deliberate misuses whose findings `lint_test.go` pins.
