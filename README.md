# structpages

[![CI](https://github.com/jackielii/structpages/actions/workflows/ci.yml/badge.svg)](https://github.com/jackielii/structpages/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/jackielii/structpages.svg)](https://pkg.go.dev/github.com/jackielii/structpages)
[![codecov](https://codecov.io/gh/jackielii/structpages/branch/main/graph/badge.svg)](https://codecov.io/gh/jackielii/structpages)
[![Go Report Card](https://goreportcard.com/badge/github.com/jackielii/structpages)](https://goreportcard.com/report/github.com/jackielii/structpages)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Struct Pages provides a way to define routing using struct tags and methods. It
integrates with the [http.ServeMux], allowing you to quickly build up pages and
components without too much boilerplate.

**Status**: **Alpha** - This package is in early development and may have breaking changes in the future. Currently used in a medium-sized project, but not yet battle-tested in production.

## Features

- Struct based routing
- Templ support built-in
- Built on top of http.ServeMux
- Middleware support
- HTMX partial rendering

## Installation

```shell
go get github.com/jackielii/structpages
```

## Usage

```templ
type index struct {
	product `route:"/product Product"`
	team    `route:"/team Team"`
	contact `route:"/contact Contact"`
}

templ (index) Page() {
	@html() {
		<h1>Welcome to the Index Page</h1>
		<p>Navigate to the product, team, or contact pages using the links below:</p>
	}
}
...
```

Route definitions are done using struct tags in for form of `[method] path [Title]`. Valid patterns:

- `/path` - For all methods that match `/path` without a title
- `POST /path` - For POST requests matching `/path`
- `/path Awesome Product` - For ALL requests matching `/path` with a title "Awesome Product"

```go
sp := structpages.New()
r := structpages.NewRouter(http.NewServeMux())
sp.MountPages(r, index{}, "/", "index")
log.Println("Starting server on :8080")
http.ListenAndServe(":8080", r)
```

Check out the [examples](./examples) for more usages.

## Routing Patterns and Struct Tags

### Basic Route Definition

Routes are defined using struct tags with the `route:` prefix. Each struct field with a route tag becomes a route in your application.

```go
type pages struct {
    home    `route:"/ Home"`           // GET / with title "Home"
    about   `route:"/about About Us"`  // GET /about with title "About Us"
    contact `route:"/contact"`         // GET /contact without title
}
```

### Route Tag Format

The route tag supports several formats:

1. **Path only**: `route:"/path"`
   - Matches all HTTP methods
   - No page title

2. **Path with title**: `route:"/path Page Title"`
   - Matches all HTTP methods
   - Sets page title to "Page Title"

3. **Method and path**: `route:"POST /path"`
   - Matches only specified HTTP method
   - No page title

4. **Full format**: `route:"PUT /path Update Page"`
   - Matches only PUT requests
   - Sets page title to "Update Page"

Supported HTTP methods: `GET`, `HEAD`, `POST`, `PUT`, `PATCH`, `DELETE`, `CONNECT`, `OPTIONS`, `TRACE`

### Path Parameters

Path parameters use Go 1.22+ `http.ServeMux` syntax:

```go
type pages struct {
    userProfile `route:"/users/{id} User Profile"`
    blogPost    `route:"/blog/{year}/{month}/{slug}"`
}

// Access parameters in your handler:
func (p userProfile) Page(r *http.Request) templ.Component {
    userID := r.PathValue("id")
    // ...
}
```

### Nested Routes

Create hierarchical URL structures by nesting structs:

```go
type pages struct {
    admin adminPages `route:"/admin Admin Panel"`
}

type adminPages struct {
    dashboard `route:"/ Dashboard"`        // Becomes /admin/
    users     `route:"/users User List"`   // Becomes /admin/users
    settings  `route:"/settings Settings"` // Becomes /admin/settings
}
```

## Middleware Usage

### Global Middleware

Apply middleware to all routes:

```go
sp := structpages.New()
r := structpages.NewRouter(http.NewServeMux(), 
    structpages.WithMiddlewares(
        loggingMiddleware,
        authMiddleware,
    ),
)
```

### Per-Page Middleware

Implement the `Middlewares()` method to add middleware to specific pages:

```go
type protectedPage struct{}

func (p protectedPage) Middlewares() []func(http.Handler) http.Handler {
    return []func(http.Handler) http.Handler{
        requireAuth,
        checkPermissions,
    }
}

func (p protectedPage) Page() templ.Component {
    return myProtectedContent()
}
```

### Middleware Execution Order

Middlewares are executed in the order they are defined:
1. Global middlewares (first to last)
2. Page-specific middlewares (first to last)
3. Page handler

The middleware execution forms a chain where each middleware wraps the next, creating an "onion" pattern. The `TestMiddlewareOrder` test in the codebase validates this behavior.

## HTMX Integration

Structpages has built-in support for HTMX partial rendering:

### Basic HTMX Support

Define a component method for HTMX requests:

```go
type todoItem struct{}

func (t todoItem) Page() templ.Component {
    // Full page render
    return todoPageTemplate()
}

func (t todoItem) TodoRow() templ.Component {
    // Partial render for HTMX
    return todoRowTemplate()
}

// Configure which component to render
func (t todoItem) PageConfig(r *http.Request) structpages.PageConfig {
    if r.Header.Get("HX-Request") == "true" {
        return structpages.PageConfig{
            Component: t.TodoRow,
        }
    }
    return structpages.PageConfig{
        Component: t.Page,
    }
}
```

### HTMX Helper Functions

Use the built-in HTMX helper:

```go
func (p myPage) PageConfig(r *http.Request) structpages.PageConfig {
    return structpages.HTMXPageConfig(r, p.Page, p.PartialContent)
}
```

This automatically returns `PartialContent` for HTMX requests and `Page` for regular requests.

## UrlFor Functionality

Generate type-safe URLs for your pages:

### Setup for Templ Templates

First, create a wrapper function for use in templ files:

```go
// urlFor wraps structpages.UrlFor for templ templates
func urlFor(ctx context.Context, page any, args ...any) (templ.SafeURL, error) {
    url, err := structpages.UrlFor(ctx, page, args...)
    return templ.SafeURL(url), err
}
```

### Basic Usage

```templ
// Simple page references without parameters
<a href={ urlFor(ctx, index{}) }>Home</a>
<a href={ urlFor(ctx, product{}) }>Products</a>
<a href={ urlFor(ctx, team{}) }>Our Team</a>

// In Go code
url, err := structpages.UrlFor(ctx, userProfile{}, "123")
// Returns: /users/123
```

### With Path Parameters

```go
// Route definition
type pages struct {
    userProfile `route:"/users/{id} User Profile"`
    blogPost    `route:"/blog/{year}/{month}/{slug} Blog Post"`
}
```

```templ
// Single parameter - positional
<a href={ urlFor(ctx, userProfile{}, "123") }>View User</a>

// Multiple parameters - as key-value pairs
<a href={ urlFor(ctx, blogPost{}, "year", "2024", "month", "06", "slug", "my-post") }>
    Read Post
</a>

// Using a map
<a href={ urlFor(ctx, blogPost{}, map[string]any{
    "year": "2024",
    "month": "06",
    "slug": "my-post",
}) }>Read Post</a>
```

### With Query Parameters

Use the `join` helper to add query parameters:

```go
// Helper function
func join(page any, pattern string) string {
    // Implementation that combines page with query pattern
}
```

```templ
// Add query parameters with template placeholders
<a href={ urlFor(ctx, join(product{}, "?page={page}"), "page", "2") }>
    Page 2
</a>

// Multiple query parameters
<form hx-post={ urlFor(ctx, join(toggle{}, "?redirect={url}"), 
    "id", todoId, 
    "url", currentURL) }>
    <button>Toggle</button>
</form>

// Complex example with path and query parameters
<a href={ urlFor(ctx, join(jobDetail{}, "?tab={tab}"), 
    "id", jobId, 
    "tab", "overview") }>
    Job Overview
</a>
```

## Templ Patterns

### Basic Page Pattern

```templ
// Define your page struct
type homePage struct{}

// Implement the Page method returning a templ component
templ (h homePage) Page() {
    @layout() {
        <h1>Welcome Home</h1>
        <p>This is the home page content.</p>
    }
}

// Shared layout component
templ layout() {
    <!DOCTYPE html>
    <html>
        <head>
            <title>My App</title>
        </head>
        <body>
            { children... }
        </body>
    </html>
}
```

### Props Pattern

Pass data to your components using Props:

```go
type productPage struct {
    ProductID string
}

func (p productPage) Props(r *http.Request) (map[string]any, error) {
    productID := r.PathValue("id")
    product, err := loadProduct(productID)
    if err != nil {
        return nil, err
    }
    return map[string]any{
        "product": product,
    }, nil
}

templ (p productPage) Page(props map[string]any) {
    @layout() {
        <h1>{ props["product"].(Product).Name }</h1>
        <p>{ props["product"].(Product).Description }</p>
    }
}
```

### Component Composition

Break down pages into smaller components:

```templ
templ (p dashboardPage) Page() {
    @layout() {
        @header()
        @sidebar()
        <main>
            @statsWidget()
            @recentActivity()
        </main>
    }
}

templ statsWidget() {
    <div class="widget">
        <h2>Statistics</h2>
        // ...
    </div>
}

templ recentActivity() {
    <div class="widget">
        <h2>Recent Activity</h2>
        // ...
    </div>
}
```

### Error Handling Pattern

```go
type formPage struct{}

func (f formPage) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
    if r.Method == "POST" {
        // Process form
        if err := processForm(r); err != nil {
            // Error will be handled by error page
            return err
        }
        http.Redirect(w, r, "/success", http.StatusSeeOther)
        return nil
    }
    
    // Render form
    return structpages.Render(w, r, f.Page())
}
```

## Advanced Features

### Custom Handlers

Implement `ServeHTTP` for complete control:

```go
type apiEndpoint struct{}

func (a apiEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "status": "ok",
    })
}
```

### Initialization

Use the `Init` method for setup:

```go
type databasePage struct {
    db *sql.DB
}

func (d *databasePage) Init() {
    // Called during route parsing
    d.db = connectToDatabase()
}
```

### Dependency Injection

Register and use dependencies:

```go
// Register a type
structpages.RegisterArg(&UserService{})

// Use in your page
func (p myPage) Page(r *http.Request, svc *UserService) templ.Component {
    users := svc.GetUsers()
    return renderUsers(users)
}
