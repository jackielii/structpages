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

## Development Setup

To set up pre-commit hooks for automatic code formatting and linting:

```shell
./scripts/setup-hooks.sh
```

This will configure git to run `goimports`, `gofmt`, and `golangci-lint` before each commit.

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
if err := sp.MountPages(r, index{}, "/", "index"); err != nil {
    log.Fatal(err)
}
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

Example middleware implementation:

```go
// Authentication middleware that checks for a valid session
func requireAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        session := r.Context().Value("session")
        if session == nil {
            http.Redirect(w, r, "/login", http.StatusSeeOther)
            return
        }
        next.ServeHTTP(w, r)
    })
}

// Logging middleware that tracks page access
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        log.Printf("%s %s took %v", r.Method, r.URL.Path, time.Since(start))
    })
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

### HTMX Helper Functions

Enable HTMX support globally when creating your StructPages instance:

```go
sp := structpages.New(
    structpages.WithDefaultPageConfig(structpages.HTMXPageConfig),
    // other options...
)
```

With this configuration, HTMX requests will automatically render the appropriate component based on the HX-Target header. For example:
- If HX-Target is "content", it will look for and call the `Content()` method on your page struct
- If HX-Target is "sidebar", it will look for and call the `Sidebar()` method
- If no HX-Target or the method doesn't exist, it falls back to the `Page()` method

### Custom HTMX Target Handling

For more complex scenarios, implement custom PageConfig that switches based on HX-Target:

```go
type todoPage struct{}

templ (t todoPage) Page() {
    // Full page
}

templ (t todoPage) TodoList() {
    // Render just the todo list
}

templ (t todoPage) TodoItem() {
    // Render a single todo item
}

// Return the component name as a string based on HX-Target
func (t todoPage) PageConfig(r *http.Request) (string, error) {
    hxTarget := r.Header.Get("HX-Target")
    
    switch hxTarget {
    case "todo-list":
        return "TodoList", nil
    case "todo-item":
        return "TodoItem", nil
    default:
        return "Page", nil
    }
}
```

## UrlFor Functionality

Generate type-safe URLs for your pages:

### Setup for Templ Templates

First, create a wrapper function for use in templ files:

```go
// urlFor wraps structpages.URLFor for templ templates
func urlFor(ctx context.Context, page any, args ...any) (templ.SafeURL, error) {
    url, err := structpages.URLFor(ctx, page, args...)
    return templ.URL(url), err
}
```

### Basic Usage

```templ
// Simple page references without parameters
<a href={ urlFor(ctx, index{}) }>Home</a>
<a href={ urlFor(ctx, product{}) }>Products</a>
<a href={ urlFor(ctx, team{}) }>Our Team</a>
```

### With Path Parameters

```go
// Route definition
type pages struct {
    userProfile `route:"/users/{id} User Profile"`
    blogPost    `route:"/blog/{year}/{month}/{slug} Blog Post"`
}

// In Go code (e.g., in handlers or middleware)
url, err := structpages.URLFor(ctx, userProfile{}, "123")
// Returns: /users/123
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

Pass data to your components using typed Props:

```go
type productPage struct{}

// Define typed props for better type safety
type productPageProps struct {
    Product Product
    RelatedProducts []Product
    IsInStock bool
}

// Props method returns typed props and can receive injected dependencies
func (p productPage) Props(r *http.Request, store *Store) (productPageProps, error) {
    productID := r.PathValue("id")
    product, err := store.LoadProduct(productID)
    if err != nil {
        return productPageProps{}, err
    }
    
    related, _ := store.LoadRelatedProducts(productID)
    
    return productPageProps{
        Product: product,
        RelatedProducts: related,
        IsInStock: product.Stock > 0,
    }, nil
}

// Page method receives typed props
templ (p productPage) Page(props productPageProps) {
    @layout() {
        <h1>{ props.Product.Name }</h1>
        <p>{ props.Product.Description }</p>
        if props.IsInStock {
            <button>Add to Cart</button>
        } else {
            <span>Out of Stock</span>
        }
        @relatedProductsList(props.RelatedProducts)
    }
}
```

#### Props Method Resolution Rules

Structpages looks for Props methods in the following order:

1. **Component-specific Props method**: `<ComponentName>Props()` - e.g., `PageProps()`, `ContentProps()`, `SidebarProps()`
2. **Generic Props method**: `Props()` - used as a fallback if no component-specific method exists

This allows you to have different props for different components:

```go
type dashboardPage struct{}

// Different props for different components
func (d dashboardPage) PageProps(r *http.Request, store *Store) (PageData, error) {
    // Full page data including layout
    return PageData{User: store.GetUser(r), Stats: store.GetStats()}, nil
}

func (d dashboardPage) ContentProps(r *http.Request, store *Store) (ContentData, error) {
    // Just the content data for HTMX partial updates
    return ContentData{Stats: store.GetStats()}, nil
}

func (d dashboardPage) Page(data PageData) templ.Component {
    // Full page render
}

func (d dashboardPage) Content(data ContentData) templ.Component {
    // Partial content render
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

## Advanced Features

### Custom Handlers

Structpages supports two types of custom handlers:

#### ServeHTTP with Error Return (Buffered)

When `ServeHTTP` returns an error, structpages uses a buffered writer to capture the response. This allows proper error page rendering if an error occurs:

```go
type formPage struct{}

// This handler uses a buffered writer
func (f formPage) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
    if r.Method == "POST" {
        // Process form
        if err := processForm(r); err != nil {
            // Response is buffered, so error page can be rendered
            return err
        }
        http.Redirect(w, r, "/success", http.StatusSeeOther)
        return nil
    }
    
    // Render form
    return structpages.Render(w, r, f.Page())
}
```

#### Standard http.Handler (Direct Write)

Implementing the standard `http.Handler` interface writes directly to the response without buffering:

```go
type apiEndpoint struct{}

// This handler writes directly to the response
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

Structpages supports dependency injection by passing services when mounting pages. These services are then available in page methods:

```go
// Define your services
type Store struct {
    db *sql.DB
}

type SessionManager struct {
    // session configuration
}

// Pass services when mounting pages
sp := structpages.New()
r := structpages.NewRouter(http.NewServeMux())

store := &Store{db: db}
sessionManager := NewSessionManager()

// Services are passed as additional arguments to MountPages
if err := sp.MountPages(r, pages{}, "/", "My App", 
    store,           // Will be available in page & other methods
    sessionManager,  // Will be available in page & other methods
    logger,          // Any other dependencies
); err != nil {
    log.Fatal(err)
}
```

**Important:** Dependency injection is type-based. Each type can only be registered once. Attempting to register duplicate types will result in an error. If you need to inject multiple values of the same underlying type (e.g., multiple strings), create distinct types:

```go
// DON'T do this - will return an error for duplicate type
if err := sp.MountPages(r, pages{}, "/", "My App", 
    "api-key",      // First string
    "db-name",      // Second string - will cause error
); err != nil {
    // Error: duplicate type string in args registry
}

// DO this instead - create distinct types
type APIKey string
type DatabaseName string

if err := sp.MountPages(r, pages{}, "/", "My App", 
    APIKey("your-api-key"),
    DatabaseName("mydb"),
); err != nil {
    log.Fatal(err)
}

// Use in your methods
func (p userPage) Props(r *http.Request, apiKey APIKey, dbName DatabaseName) (UserProps, error) {
    // Both values are available with type safety
    client := NewAPIClient(string(apiKey))
    conn := OpenDB(string(dbName))
    // ...
}
```

#### Using Injected Services

Services are automatically injected into page methods that declare them as parameters:

```go
type userListPage struct{}

// Props method receives injected Store
func (p userListPage) Props(r *http.Request, store *Store) (UserListProps, error) {
    users, err := store.GetUsers()
    if err != nil {
        return UserListProps{}, err
    }
    return UserListProps{Users: users}, nil
}

// ServeHTTP can also receive injected services
func (p signOutPage) ServeHTTP(w http.ResponseWriter, r *http.Request, sm *SessionManager) error {
    // Clear user session
    sm.Destroy(r.Context())
    http.Redirect(w, r, "/", http.StatusSeeOther)
    return nil
}

// Middleware methods can receive services too
func (p protectedPages) Middlewares(sm *SessionManager) []structpages.MiddlewareFunc {
    return []structpages.MiddlewareFunc{
        func(next http.Handler, pn *structpages.PageNode) http.Handler {
            return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                if !sm.Exists(r.Context(), "user") {
                    http.Redirect(w, r, "/login", http.StatusSeeOther)
                    return
                }
                next.ServeHTTP(w, r)
            })
        },
    }
}
