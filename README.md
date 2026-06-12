# structpages

[![CI](https://github.com/jackielii/structpages/actions/workflows/ci.yml/badge.svg)](https://github.com/jackielii/structpages/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/jackielii/structpages.svg)](https://pkg.go.dev/github.com/jackielii/structpages)
[![codecov](https://codecov.io/gh/jackielii/structpages/branch/main/graph/badge.svg)](https://codecov.io/gh/jackielii/structpages)
[![Go Report Card](https://goreportcard.com/badge/github.com/jackielii/structpages)](https://goreportcard.com/report/github.com/jackielii/structpages)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Documentation:** <https://jackielii.github.io/structpages/>

Struct Pages provides a way to define routing using struct tags and methods. It integrates with Go's `http.ServeMux`, allowing you to quickly build web applications with minimal boilerplate.

**Status**: **Beta** - The API has settled and the library is battle-tested in production, carrying medium-to-large applications. Remaining breaking changes before v1 will be rare and called out in release notes.

## Features

- 🏗️ **Struct-based routing** - Define routes using struct tags
- 🎨 **Templ support** - Built-in integration with [Templ](https://templ.guide/)
- ⚡ **HTMX-friendly** - Automatic partial rendering support
- 🔧 **Middleware** - Standard Go middleware pattern
- 🎯 **Type-safe URLs** - Generate URLs from struct references
- 📦 **Dependency injection** - Pass dependencies to handlers via options

## Installation

```shell
go get github.com/jackielii/structpages
```

## Quick Start

Define your page structure using struct tags:

```go
package main

import (
    "log"
    "net/http"
    "github.com/jackielii/structpages"
)

type index struct {
    product `route:"/product Product"`
    team    `route:"/team Team"`
    contact `route:"/contact Contact"`
}

// Implement the Page method using Templ
templ (index) Page() {
    <html>
        <body>
            <h1>Welcome</h1>
            <nav>
                <a href="/product">Product</a>
                <a href="/team">Team</a>
                <a href="/contact">Contact</a>
            </nav>
        </body>
    </html>
}

func main() {
    mux := http.NewServeMux()
    _, err := structpages.Mount(mux, index{}, "/", "Home")
    if err != nil {
        log.Fatal(err)
    }

    log.Println("Starting server on :8080")
    http.ListenAndServe(":8080", mux)
}
```

Route definitions use the format `[method] path [Title]`:

- `/path` - All methods, no title
- `POST /path` - POST requests only
- `GET /path Page Title` - GET requests with title "Page Title"

## Documentation

- [API Reference](./docs/api.md) - Complete API documentation for Mount, options, and methods
- [Routing Patterns](./docs/routing.md) - Route definitions, path parameters, nested routes
- [Supported Request Flows](./docs/supported-flows.md) - How requests are dispatched to handlers and components
- [Middleware](./docs/middleware.md) - Using middleware with your pages
- [HTMX Integration](./docs/htmx.md) - Partial rendering and HTMX support
- [URLFor & ID Generation](./docs/urlfor.md) - Type-safe URL and HTML id generation
- [Templ Patterns](./docs/templ.md) - Working with Templ templates
- [Advanced Features](./docs/advanced.md) - Dependency injection, Init, dynamic Refs, type aliases
- [Testing renders](./skills/structpages/SKILL.md#8-testing-renders-with-a-bare-context) - `structpages.Parse` + `sp.PageContext(ctx)` for unit tests that render templ components without an HTTP server

## Examples

Check out the [examples directory](./examples) for complete working applications:

- [Simple](./examples/simple) - Basic routing and page rendering
- [HTMX](./examples/htmx) - HTMX integration with partial updates
- [HTMX RenderTarget](./examples/htmx-render-target) - Standalone-function components shared across pages, with per-component data loading via `RenderTarget`
- [Todo](./examples/todo) - Full TODO app: form actions via `ServeHTTP` returning `RenderComponent(...)` to re-render a sibling component (in-memory store)

## Lint Tool

`structpages-lint` is a static analyzer that checks the most common ways `structpages` calls go wrong.

```shell
go install github.com/jackielii/structpages/tools/lint/cmd/structpages-lint@latest
structpages-lint ./...
```

Categories:

- `urlfor`, `ref`, `id`, `idtarget`, `params` — checks `structpages.URLFor` / `Ref` / `ID` / `IDTarget` call sites against the reconstructed page tree.
- `url-attr` — scans `.templ` files for URL-bearing HTML attributes (`href`, `action`, `formaction`, `hx-{get,post,put,patch,delete}`, `hx-{push,replace}-url`) whose values are hard-coded internal paths, string concatenations, or `fmt.Sprint*` calls — i.e. cases where you should have called `structpages.URLFor`. Allows `https://`, `mailto:`, `#`, protocol-relative `//`.
- `route-literal` — scans `.go` files for string literals whose value exactly equals a mounted route (e.g. `return "/admin/queues"`), where you should resolve the URL by page type via `structpages.URLFor` instead, so renames break the build rather than drifting. Narrow by design: only an exact concrete-route match counts (param/`{$}` routes, trailing-slash and query variants, and the bare `/` never match); literals in `==`/`switch` comparisons and `Ref(...)` arguments are skipped (they read a route, not generate a URL), as are `_test.go` and generated files.

Suppress a single diagnostic with a comment. Prefer `//` in both file types — Go-style comments are stripped from the generated HTML; `<!-- … -->` HTML comments render into every response.

```go
//structpages:lint:ignore url-attr           // in .go files
```

```templ
// structpages:lint:ignore url-attr          // in .templ files (preferred)
<!-- structpages:lint:ignore url-attr -->    <!-- also works in .templ -->
```

## Claude Code Skill

This repo ships a [Claude Code](https://claude.com/claude-code) plugin that teaches Claude the structpages idioms (`Props`/`RenderTarget`, HTMX partials, `URLFor`/`ID`/`IDTarget`, middleware, DI). Inside Claude Code:

```text
/plugin marketplace add jackielii/structpages
/plugin install structpages@structpages
```

Restart Claude Code when prompted. The skill loads automatically when you work on a structpages project; you can also invoke it explicitly with `/structpages:structpages`.

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for development setup and guidelines.

## License

MIT License - see [LICENSE](./LICENSE) file for details.
