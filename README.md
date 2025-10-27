# structpages

[![CI](https://github.com/jackielii/structpages/actions/workflows/ci.yml/badge.svg)](https://github.com/jackielii/structpages/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/jackielii/structpages.svg)](https://pkg.go.dev/github.com/jackielii/structpages)
[![codecov](https://codecov.io/gh/jackielii/structpages/branch/main/graph/badge.svg)](https://codecov.io/gh/jackielii/structpages)
[![Go Report Card](https://goreportcard.com/badge/github.com/jackielii/structpages)](https://goreportcard.com/report/github.com/jackielii/structpages)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Struct Pages provides a way to define routing using struct tags and methods. It integrates with Go's `http.ServeMux`, allowing you to quickly build web applications with minimal boilerplate.

**Status**: **Alpha** - This package is in early development and may have breaking changes in the future. Currently used in a medium-sized project, but not yet battle-tested in production.

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
- [Middleware](./docs/middleware.md) - Using middleware with your pages
- [HTMX Integration](./docs/htmx.md) - Partial rendering and HTMX support
- [URLFor](./docs/urlfor.md) - Type-safe URL generation
- [Templ Patterns](./docs/templ.md) - Working with Templ templates
- [Advanced Features](./docs/advanced.md) - Dependency injection, error handling, and more

## Examples

Check out the [examples directory](./examples) for complete working applications:

- [Simple](./examples/simple) - Basic routing and page rendering
- [HTMX](./examples/htmx) - HTMX integration with partial updates
- [Todo](./examples/todo) - Full todo application with database

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for development setup and guidelines.

## License

MIT License - see [LICENSE](./LICENSE) file for details.
