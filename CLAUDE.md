# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Structpages is a Go web framework library that provides struct-based routing. It integrates with Go's standard `http.ServeMux` and provides a declarative way to define routes using struct tags. The framework is designed to reduce boilerplate code when building web pages and components. It renders any value with `Render(context.Context, io.Writer) error`; the examples use gsx, and templ and `html/template` also work.

**Status**: Beta (API settled; battle-tested in production by medium-to-large applications)

## Development Commands

### Testing
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run a specific test
go test -run TestName ./...

# Run tests with verbose output
go test -v ./...
```

### Working with Examples
```bash
# Navigate to an example directory
cd examples/simple  # or examples/htmx, examples/todo, examples/blog, ...

# Generated .x.go files are committed, so this runs as is (typically on :8080)
go run .

# After editing .gsx sources: regenerate and format. Each example pins the gsx
# CLI as a `tool` dependency; the repo-root gsx.toml registers the
# structpages url/id/target filters for every example.
go tool gsx generate .
go tool gsx fmt -w .
```

### Required Tools
- Go 1.26 or later for the examples (the library itself needs Go 1.24)

## Architecture Overview

### Core Components

1. **Router System**: Built on top of `http.ServeMux`, the router parses struct tags to create routes
   - `struct_pages.go`: `Mount`, `StructPages`, options, and request dispatch
   - `parse.go`: Parses struct tags like `route:"/path Title"`, builds the page tree, handles DI matching
   - `page_node.go`: `PageNode` struct and tree traversal (`All`, `FullRoute`)

2. **Struct-Based Routing Pattern**: Routes are defined as struct fields with route tags
   ```go
   type pages struct {
       product `route:"/product Product"`
       team    `route:"POST /team Team"`
   }
   ```
   Each leaf page handles requests via one of: a `Page()` component method (most common), a `Props` method (Props-only pages), or a `ServeHTTP` method (form actions, redirects). Promoted (embedded) methods are skipped.

3. **HTMX Support**: Built-in partial rendering via the default `HTMXRenderTarget` selector
   - `htmx.go`: `HTMXRenderTarget` and `matchComponentByTarget` (page-prefix + suffix matching)
   - `render_target.go`: `RenderTarget` interface, `methodRenderTarget`, `functionRenderTarget`, `TargetSelector` type
   - `id_for.go`: `ID` (raw HTML id) and `IDTarget` (`#`-prefixed CSS selector) for HTMX `hx-target`

4. **URL Generation**: Type-safe URL generation via `URLFor`
   - `url_for.go`: `URLFor`, segment parsing, automatic param extraction from current request route
   - `id_for.go`: `Ref` type for dynamic name/route-based references when static type lookup doesn't fit

5. **Middleware Support**: Standard Go middleware with route metadata
   - `MiddlewareFunc = func(http.Handler, *PageNode) http.Handler` — middleware receives the `PageNode`
   - Apply globally via `WithMiddlewares` or per-page via a `Middlewares()` method (also applies to descendants)

6. **Dependency Injection**: Type-based DI via `WithArgs(...)`
   - `args.go`: `argRegistry` matches by type; a registered pointer fills a value param (not the reverse), with an assignability fallback
   - Each registered type appears once; use named types to disambiguate
   - Generic types are supported; interface-typed params are not filled by a registered implementation (see `generics_injection_test.go`)

### Key Design Patterns

- **Page Interface**: A page struct typically implements `Page()` (full render) and optionally `Content()` (HTMX partial body), plus arbitrary partial methods. Pages can also be Props-only (Props returns `RenderComponent(...)`) or ServeHTTP-only.
- **Nested Routing**: Structs can contain other structs to create nested route hierarchies. Children register before parents to avoid mux conflicts.
- **Context Passing**: Uses `ctxkey` for safe context value passing (`pcCtx`, `urlParamsCtx`, `currentPageCtx`)
- **Error Handling**: Built-in error page support via `WithErrorHandler`. Return `ErrSkipPageRender` from `Props` to skip rendering after a redirect.

### Testing Approach

The codebase uses standard Go testing with:
- Unit tests for each major component
- Test coverage for routing, parsing, HTMX, and URL generation
- Uses `google/go-cmp` for test comparisons

When adding new features:
1. Add corresponding tests in `*_test.go` files
2. Ensure examples still work after changes
3. Test with both regular HTTP and HTMX requests if applicable

## Docs Site

The published site (https://jackielii.github.io/structpages/) is built from **two sources**:

- **Markdown content** comes from `docs/*.md` (plus `README.md`, `PERFORMANCE.md`, `examples/README.md`) on `main` — the Docs Site workflow copies it in at build time.
- **The site shell** (Docusaurus config, homepage hero in `src/pages/index.tsx`, styles) lives on the separate `docs-site` branch, checked out as a worktree at `../structpages-docs-site`.

The workflow only auto-triggers on pushes to `main`. After pushing to `docs-site`, deploy manually with `gh workflow run "Docs Site"`. When changing repo-wide claims (e.g. the Alpha→Beta status, June 2026), remember the homepage badge in `index.tsx` on `docs-site` — it's hardcoded there and not covered by grepping `main`.

## Claude Code Skill

A library-consumer-facing skill ships with this repo at `skills/structpages/SKILL.md` (with `reference.md`, `examples.md` and `templ.md`). It teaches users of the library — not contributors — patterns for `Props`/`RenderTarget`, HTMX partial rendering, the gsx `url`/`id`/`target` filters over `URLFor`/`ID`/`IDTarget`, middleware, and DI. gsx is the primary path; `templ.md` covers templ projects. A minimal `.claude-plugin/plugin.json` makes the repo installable as a Claude Code plugin.

When working on the library itself: read `skills/structpages/SKILL.md` for an authoritative summary of the public API and idioms (it is kept in sync with source), or symlink `skills/structpages/` into your `~/.claude/skills/` for auto-load while editing this repo.
