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

### WithTargetSelector

```go
func WithTargetSelector(selector TargetSelector) func(*StructPages)
```

Set a global target selector function that determines which component to render. The selector returns a `RenderTarget` that is passed to your Props method, enabling conditional data loading and component selection.

The default selector is `HTMXRenderTarget`, which handles HTMX partial rendering automatically.

**Example:**
```go
// Custom selector for API requests
selector := func(r *http.Request, pn *PageNode) (structpages.RenderTarget, error) {
    if r.Header.Get("Accept") == "application/json" {
        // Return a specific component for JSON responses
        method := pn.Components["APIResponse"]
        return structpages.NewMethodRenderTarget("APIResponse", method), nil
    }
    // Fall back to default HTMX behavior
    return structpages.HTMXRenderTarget(r, pn)
}

sp, err := structpages.Mount(mux, &pages{}, "/", "Home",
    structpages.WithTargetSelector(selector),
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
func (p PageType) Props(r *http.Request, target RenderTarget, deps ...any) (any, error)
```

Optional. Prepare data before rendering. Receives:
- `r *http.Request`: The HTTP request
- `target RenderTarget`: Indicates which component will be rendered
- `deps ...any`: Injected dependencies (from `WithArgs`)

Use `target.Is(component)` to conditionally load data based on which component is being rendered.

**Example:**
```go
func (p DashboardPage) Props(r *http.Request, target RenderTarget, db *Database) (DashboardProps, error) {
    switch {
    case target.Is(p.UserList):
        // Only load user data for partial update
        users := db.LoadUsers()
        return DashboardProps{}, RenderComponent(target, users)

    case target.Is(p.Page):
        // Load all data for full page
        return DashboardProps{
            Users: db.LoadUsers(),
            Stats: db.LoadStats(),
        }, nil
    }
}

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

## RenderTarget

The `RenderTarget` interface represents the component that will be rendered for a request. It's passed to your Props method, enabling conditional data loading.

### Interface

```go
type RenderTarget interface {
    Is(method any) bool
}
```

### Is Method

```go
func (target RenderTarget) Is(method any) bool
```

Check if the target matches a specific component. Works with:
- **Page methods**: `target.Is(p.Page)`, `target.Is(p.UserList)`
- **Standalone functions**: `target.Is(UserStatsWidget)` (templ components that are functions)

**Example:**
```go
func (p DashboardPage) Props(r *http.Request, target RenderTarget) (DashboardProps, error) {
    // Check against page method
    if target.Is(p.UserList) {
        users := loadUsers()
        return DashboardProps{}, RenderComponent(target, users)
    }

    // Check against standalone function component
    if target.Is(UserStatsWidget) {
        stats := loadUserStats()
        return DashboardProps{}, RenderComponent(target, stats)
    }

    // Full page
    return loadFullPageData(), nil
}
```

### RenderComponent

```go
func RenderComponent(targetOrMethod any, args ...any) error
```

Override which component to render and pass specific arguments to it. Can be called from Props:

**Same-page component:**
```go
// Render the component specified by target with custom args
return DashboardProps{}, RenderComponent(target, userData)
```

**Cross-page component:**
```go
// Render a component from another page using method expression
return DashboardProps{}, RenderComponent(OtherPage{}.Content, data)
```

**Standalone function:**
```go
// Render a standalone function component
return DashboardProps{}, RenderComponent(target, stats)
```

### HTMXRenderTarget

```go
func HTMXRenderTarget(r *http.Request, pn *PageNode) (RenderTarget, error)
```

The default `TargetSelector` that handles HTMX partial rendering. It:
1. Checks for `HX-Request` header
2. Reads `HX-Target` header value
3. Matches it to a component method or function
4. Returns appropriate `RenderTarget`

For non-HTMX requests or missing targets, returns the `Page` component.

**Automatic behavior:**
- `HX-Target: "content"` → matches `Content()` method
- `HX-Target: "index-todo-list"` → matches `TodoList()` method (strips page prefix)
- `HX-Target: "dashboard-page-user-stats-widget"` → matches `UserStatsWidget` function

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
