# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Structpages is a Go web framework library that provides struct-based routing. It integrates with Go's standard `http.ServeMux` and provides a declarative way to define routes using struct tags. The framework is designed to reduce boilerplate code when building web pages and components, with built-in support for the Templ templating engine.

**Status**: Alpha (early development stage)

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
cd examples/simple  # or examples/htmx or examples/todo

# Install dependencies
go mod download

# Generate Go code from Templ files (required before running)
templ generate -include-version=false

# Run the example server (typically on :8080)
go run main.go

# Watch mode for Templ files during development
templ generate -include-version=false --watch
```

### Required Tools
- Go 1.24.3 or later
- Templ CLI: `go install github.com/a-h/templ/cmd/templ@latest`

## Architecture Overview

### Core Components

1. **Router System**: Built on top of `http.ServeMux`, the router parses struct tags to create routes
   - `router.go`: Core router implementation
   - `parse.go`: Parses struct tags like `route:"/path Title"`
   - `page_node.go`: Handles page node structure and rendering

2. **Struct-Based Routing Pattern**: Routes are defined as struct fields with route tags
   ```go
   type pages struct {
       product `route:"/product Product"`
       team    `route:"POST /team Team"`
   }
   ```
   Each struct must implement a `Page()` method that returns a templ component.

3. **HTMX Support**: Built-in support for partial rendering
   - `htmx.go`: HTMX-specific functionality
   - Allows returning partial components for HTMX requests
   - `id_for.go`: Generates component method IDs for hx-target

4. **URL Generation**: Type-safe URL generation
   - `url_for.go`: Provides `UrlFor` functionality to generate URLs from struct references

5. **Middleware Support**: Standard Go middleware pattern integration
   - Middleware can be applied at router level or per-route

### Key Design Patterns

- **Page Interface**: All routable structs must implement the `Page()` method returning a templ component
- **Nested Routing**: Structs can contain other structs to create nested route hierarchies
- **Context Passing**: Uses `ctxkey` for safe context value passing
- **Error Handling**: Built-in error page support with customizable error handlers

### Testing Approach

The codebase uses standard Go testing with:
- Unit tests for each major component
- Test coverage for routing, parsing, HTMX, and URL generation
- Uses `google/go-cmp` for test comparisons

When adding new features:
1. Add corresponding tests in `*_test.go` files
2. Ensure examples still work after changes
3. Test with both regular HTTP and HTMX requests if applicable
