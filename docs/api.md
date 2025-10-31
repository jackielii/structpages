# API Reference

## Core Functions

### Mount

```go
func Mount(mux Mux, page any, route, title string, options ...Option) (*StructPages, error)
```

Mount is the main entry point for setting up routes. It parses the page structure, registers routes on the provided mux, and returns a `StructPages` instance for URL generation.

**Parameters:**
- `mux`: HTTP router implementing the `Mux` interface (e.g., `*http.ServeMux`). Pass `nil` to use `http.DefaultServeMux`.
- `page`: Struct containing route definitions using struct tags
- `route`: Base route path (e.g., `"/"`)
- `title`: Page title for the root route
- `options`: Configuration options (WithArgs, WithErrorHandler, WithMiddlewares, etc.)

**Returns:**
- `*StructPages`: Instance for generating type-safe URLs via `URLFor` and `IDFor`
- `error`: Error if mounting fails

**Example:**
```go
mux := http.NewServeMux()
sp, err := structpages.Mount(mux, &pages{}, "/", "Home")
if err != nil {
    log.Fatal(err)
}

// Use mux for serving HTTP
http.ListenAndServe(":8080", mux)

// Use sp for generating URLs
url, _ := sp.URLFor(productPage{}, "123")
```

## Mux Interface

```go
type Mux interface {
    Handle(pattern string, handler http.Handler)
}
```

The `Mux` interface allows StructPages to work with any HTTP router that follows Go's standard routing pattern. `*http.ServeMux` implements this interface.

## StructPages

```go
type StructPages struct {
    // Internal fields
}
```

Returned by `Mount()`, StructPages provides methods for type-safe URL generation.

### Methods

#### URLFor

```go
func (sp *StructPages) URLFor(page any, args ...any) (string, error)
```

Generate a URL for a page type with optional path parameters.

**Example:**
```go
// Simple path
url, _ := sp.URLFor(homePage{})  // "/"

// With path parameter
url, _ := sp.URLFor(userPage{}, "123")  // "/users/123"

// With named parameters
url, _ := sp.URLFor(postPage{}, "year", 2024, "slug", "hello")  // "/blog/2024/hello"
```

#### IDFor

```go
func (sp *StructPages) IDFor(v any) (string, error)
```

Generate a consistent HTML ID or CSS selector for a component method.

**Example:**
```go
id, _ := sp.IDFor((*todoPage).TodoList)  // "todo-page-todo-list"
```

## Options

Options are passed to `Mount()` to configure behavior.

### WithArgs

```go
func WithArgs(args ...any) func(*StructPages)
```

Add global dependency injection arguments available to all page methods.

**Example:**
```go
type Database struct { /* ... */ }
type Logger struct { /* ... */ }

db := &Database{}
logger := &Logger{}

// Pass dependencies using WithArgs
sp, err := structpages.Mount(mux, &pages{}, "/", "Home",
    structpages.WithArgs(db, logger),
    structpages.WithErrorHandler(errorHandler),
)
```

Handler methods can receive injected dependencies:

```go
func (p productPage) Props(r *http.Request, db *Database, logger *Logger) (ProductProps, error) {
    // Use db and logger
}
```

### WithErrorHandler

```go
func WithErrorHandler(handler func(w http.ResponseWriter, r *http.Request, err error)) func(*StructPages)
```

Set a custom error handler for handling errors during request processing.

**Example:**
```go
errorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
    log.Printf("Error: %v", err)
    http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

sp, err := structpages.Mount(mux, &pages{}, "/", "Home",
    structpages.WithErrorHandler(errorHandler),
)
```

### WithMiddlewares

```go
func WithMiddlewares(middlewares ...MiddlewareFunc) func(*StructPages)
```

Add global middleware that applies to all routes.

**Example:**
```go
loggingMiddleware := func(next http.Handler, pn *PageNode) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log.Printf("%s %s", r.Method, r.URL.Path)
        next.ServeHTTP(w, r)
    })
}

sp, err := structpages.Mount(mux, &pages{}, "/", "Home",
    structpages.WithMiddlewares(loggingMiddleware),
)
```

### WithDefaultComponentSelector

```go
func WithDefaultComponentSelector(selector func(r *http.Request, pn *PageNode) (string, error)) func(*StructPages)
```

Set a global component selector function that determines which component to render when `RenderComponent` is not explicitly called in Props. Useful for implementing patterns like HTMX partial rendering across all pages.

**Example:**
```go
// HTMX boost pattern - render only content for HTMX requests
selector := func(r *http.Request, pn *PageNode) (string, error) {
    if r.Header.Get("HX-Request") == "true" {
        return "Content", nil  // Skip layout, render just content
    }
    return "Page", nil  // Full page with layout
}

sp, err := structpages.Mount(mux, &pages{}, "/", "Home",
    structpages.WithDefaultComponentSelector(selector),
)
```

### WithWarnEmptyRoute

```go
func WithWarnEmptyRoute(warnFunc func(*PageNode)) func(*StructPages)
```

Customize or suppress warnings for pages with no handler and no children.

**Example:**
```go
// Use default warning (prints to stdout)
sp, err := structpages.Mount(mux, &pages{}, "/", "Home",
    structpages.WithWarnEmptyRoute(nil),
)

// Custom warning function
customWarn := func(pn *PageNode) {
    log.Printf("Skipping empty page: %s", pn.Name)
}
sp, err := structpages.Mount(mux, &pages{}, "/", "Home",
    structpages.WithWarnEmptyRoute(customWarn),
)

// Suppress warnings entirely
sp, err := structpages.Mount(mux, &pages{}, "/", "Home",
    structpages.WithWarnEmptyRoute(func(*PageNode) {}),
)
```

## Page Methods

Pages can implement several optional methods:

### Page

```go
func (p PageType) Page() Component
```

Required for pages that render content. Returns the component to render.

### Props

```go
func (p PageType) Props(r *http.Request, deps ...any) (any, error)
```

Optional. Prepare data before rendering. Can receive injected dependencies.

### ServeHTTP

```go
func (p PageType) ServeHTTP(w http.ResponseWriter, r *http.Request, deps ...any)
```

Optional. Handle HTTP requests directly without component rendering.

### Middlewares

```go
func (p PageType) Middlewares() []MiddlewareFunc
```

Optional. Return page-specific middleware.

## Context Functions

For use within handlers:

### URLFor

```go
func URLFor(ctx context.Context, page any, args ...any) (string, error)
```

Generate URLs using context (available during request handling).

### IDFor

```go
func IDFor(ctx context.Context, v any) (string, error)
```

Generate IDs using context.

## Error Types

### ErrSkipPageRender

```go
var ErrSkipPageRender = errors.New("skip page render")
```

Return this error from `Props` to skip rendering (useful for redirects).

**Example:**
```go
func (p loginPage) Props(r *http.Request) (any, error) {
    if isAuthenticated(r) {
        http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
        return nil, ErrSkipPageRender
    }
    return LoginProps{}, nil
}
```
