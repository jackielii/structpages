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
- `*StructPages`: Instance for generating type-safe URLs via `URLFor`, `ID`, and `IDTarget`
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

#### ID and IDTarget

```go
func (sp *StructPages) ID(v any) (string, error)
func (sp *StructPages) IDTarget(v any) (string, error)
```

Generate consistent HTML IDs for component methods.
- `ID` returns raw ID (for HTML `id` attributes): `"todo-page-todo-list"`
- `IDTarget` returns CSS selector (for HTMX `hx-target`): `"#todo-page-todo-list"`

**Example:**
```go
id, _ := sp.ID((*todoPage).TodoList)         // "todo-page-todo-list"
target, _ := sp.IDTarget((*todoPage).TodoList)  // "#todo-page-todo-list"
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

A custom selector returns any type that implements the `RenderTarget` interface (`Is(method any) bool`). The framework's own `methodRenderTarget` and `functionRenderTarget` constructors are unexported, so a custom selector typically either: (a) delegates to `HTMXRenderTarget` and returns its result for the cases it doesn't want to override, or (b) returns its own type that implements `RenderTarget` (and optionally `Component() component` for direct rendering — see *RenderComponent* below).

**Example — content negotiation that falls back to HTMX:**

```go
// Custom RenderTarget for JSON responses
type jsonTarget struct{ data any }

func (t jsonTarget) Is(method any) bool { return false } // never matches normal components
func (t jsonTarget) Component() component {
    return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
        return json.NewEncoder(w).Encode(t.data)
    })
}

selector := func(r *http.Request, pn *structpages.PageNode) (structpages.RenderTarget, error) {
    if r.Header.Get("Accept") == "application/json" {
        return jsonTarget{data: loadJSON(r, pn)}, nil
    }
    return structpages.HTMXRenderTarget(r, pn)
}

sp, err := structpages.Mount(mux, &pages{}, "/", "Home",
    structpages.WithTargetSelector(selector),
)
```

When `Props` calls `RenderComponent(target)` (no args) on a target that implements `Component()`, the framework calls `Component()` to get the component to render — useful for selectors that already know the data.

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

Optional. Prepare data before rendering. The framework matches each parameter by **type** (not position), so any of these signatures work and parameters can appear in any order:

```go
func (p PageType) Props(r *http.Request) (PropsType, error)
func (p PageType) Props(r *http.Request, store *Store) (PropsType, error)
func (p PageType) Props(r *http.Request, w http.ResponseWriter, store *Store) (PropsType, error)
func (p PageType) Props(r *http.Request, target RenderTarget, store *Store) (PropsType, error)
```

Injectable parameter types: `*http.Request`, `http.ResponseWriter`, `RenderTarget`, `*PageNode`, and any type registered via `WithArgs`. **DI is positional+typed, not variadic** — there is no `deps ...any` form; declare each dep as its own typed parameter.

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
    return DashboardProps{}, nil
}
```

### ServeHTTP

Optional. Handle HTTP requests directly. Four signatures are supported (DI form takes typed params, not variadic `any`):

```go
func (p PageType) ServeHTTP(w http.ResponseWriter, r *http.Request)                               // standard http.Handler
func (p PageType) ServeHTTP(w http.ResponseWriter, r *http.Request) error                          // buffered, error → handler
func (p PageType) ServeHTTP(w http.ResponseWriter, r *http.Request, store *Store)                  // DI, no return
func (p PageType) ServeHTTP(w http.ResponseWriter, r *http.Request, store *Store) error            // DI, buffered
```

In the DI forms, `RenderTarget` is also injectable (the framework computes one and adds it to the available args), so `ServeHTTP` can decide which partial to render via `target.Is(...)` + `RenderComponent(...)`.

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

### ID and IDTarget

```go
func ID(ctx context.Context, v any) (string, error)
func IDTarget(ctx context.Context, v any) (string, error)
```

Generate IDs using context (available during request handling).
- `ID` returns raw ID (for HTML `id` attributes)
- `IDTarget` returns CSS selector (for HTMX `hx-target`)

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

The default `TargetSelector` that handles HTMX partial rendering. Algorithm:

1. Non-HTMX requests (no `HX-Request: true` header), or HTMX requests with no `HX-Target` → returns `methodRenderTarget` for the page's `Page()` method.
2. HTMX request with `HX-Target` → tries to match it against the page's component methods:
   - **Pass 1, exact match**: first against `<pageprefix>-<componentid>`, then against bare `<componentid>`.
   - **Pass 2, suffix match (longest wins)**: with three rules — full ID ends with target; target ends with full ID; or target ends with `<componentid>` (only when target also starts with `<pageprefix>-`, which guards against cross-page false matches).
3. If a method matches → returns `methodRenderTarget` for it.
4. If no method matches → returns `functionRenderTarget` carrying the raw `HX-Target`. The actual function value is bound lazily when `Props` calls `target.Is(SomeFunc)`.

**Examples** (page named `IndexPage`, components `Content`, `TodoList`):
- `HX-Target: "content"` → `Content()` (exact match without page prefix)
- `HX-Target: "index-page-todo-list"` → `TodoList()` (exact match with page prefix)
- `HX-Target: "todo-list"` → `TodoList()` (exact match without page prefix)
- `HX-Target: "dashboard-page-user-stats-widget"` (no method by that name) → `functionRenderTarget`; resolved to `UserStatsWidget` standalone function only after Props calls `target.Is(UserStatsWidget)`.

## Error Types

### ErrSkipPageRender

```go
var ErrSkipPageRender = errors.New("skip page render")
```

Return this error from `Props` to skip rendering (useful for redirects). **Only the Props error path checks for this sentinel** — returning it from `ServeHTTP` does nothing special.

**Example:**
```go
func (p loginPage) Props(r *http.Request, w http.ResponseWriter) (LoginProps, error) {
    if isAuthenticated(r) {
        http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
        return LoginProps{}, structpages.ErrSkipPageRender
    }
    return LoginProps{}, nil
}
```
