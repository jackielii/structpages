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

## Basic Usage

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
- `GET /path Awesome Product` - For GET requests matching `/path` with a title "Awesome Product"

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
    home    `route:"/ Home"`           // ALL / with title "Home"
    about   `route:"/about About Us"`  // ALL /about with title "About Us"
    contact `route:"/contact"`         // ALL /contact without title
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

Supported HTTP methods: `GET`, `HEAD`, `POST`, `PUT`, `PATCH`, `DELETE`, `CONNECT`, `OPTIONS`, `TRACE`. A special `ALL` method can be used to match all methods.

### Path Parameters

Path parameters use Go 1.22+ `http.ServeMux` syntax:

```go
type pages struct {
    userProfile `route:"/users/{id} User Profile"`
    blogPost    `route:"/blog/{year}/{month}/{slug}"`
}

// Access parameters in your Props method:
func (p userProfile) Props(r *http.Request) (UserProfileProps, error) {
    userID := r.PathValue("id") // "123" if URL is /users/123
    // Pass the userID via the props to the Page renderer
    return UserProfileProps{UserID: userID}, nil
}

templ (p userProfile) Page(props UserProfileProps) {
    @layout() {
        <h1>User Profile for { props.UserID }</h1>
        // Render user details
    }
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
sp := structpages.New(
    structpages.WithMiddlewares(
        loggingMiddleware,
        authMiddleware,
    ),
)
r := structpages.NewRouter(http.NewServeMux())
if err := sp.MountPages(r, pages{}, "/", "My App"); err != nil {
    log.Fatal(err)
}
```

### Page Middlewares

Implement the `Middlewares()` method to add middleware to specific page, which will also be applied to its descendants:

```go
type protectedPage struct{
    // children pages will be protected
}

func (p protectedPage) Middlewares() []structpages.MiddlewareFunc {
    return []structpages.MiddlewareFunc{
        requireAuth,
        checkPermissions,
    }
}

templ (p protectedPage) Page() {
    ...
}
```

Example middleware implementation:

```go
// Authentication middleware that checks for a valid session
func requireAuth(next http.Handler, pn *structpages.PageNode) http.Handler {
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
func loggingMiddleware(next http.Handler, pn *structpages.PageNode) http.Handler {
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

Structpages has built-in HTMX support enabled by default through `HTMXPageConfig`. This makes `IDFor` work seamlessly with HTMX partial rendering out of the box.

### How It Works

When an HTMX request is detected (via `HX-Request` header), the framework automatically:

1. Reads the `HX-Target` header value
2. Converts it from kebab-case to a component method name
3. Renders that specific component instead of the full page

For example:
- `HX-Target: "content"` → calls `Content()` method
- `HX-Target: "index-todo-list"` → calls `TodoList()` method on the index page (strips page prefix automatically)
- No HX-Target or non-existent component → falls back to `Page()` method

This works automatically with `IDFor`:

```go
// In your template
<div id={ structpages.IDFor(ctx, structpages.IDParams{Method: index.TodoList, RawID: true}) }>
    @p.TodoList()
</div>

// In HTMX attributes
hx-target={ structpages.IDFor(ctx, index.TodoList) }  // Generates "#index-todo-list"
```

The HTMX request will automatically extract the component name from the target ID and render just that component.

### Custom Component Selector

If you need different behavior, you can override the default:

```go
sp := structpages.New(
    structpages.WithDefaultComponentSelector(func(r *http.Request, pn *PageNode) (string, error) {
        // Your custom logic
        return "Page", nil
    }),
)
```

See `examples/htmx/main.go` and `examples/todo/main.go` for complete working examples.

## URLFor Functionality

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
func join(page any, pattern string) []any {
    return []any{page, pattern}
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

### Automatic URL Parameter Extraction

When calling `URLFor` within a handler, URL parameters from the current request are automatically available and will be used to fill matching parameters in the generated URL. This is particularly useful when generating URLs for related pages that share the same parameters.

```go
// Route definitions
type pages struct {
    viewProduct `route:"GET /product/{id} View Product"`
    editProduct `route:"GET /product/{id}/edit Edit Product"`
}

// In the view handler
func (v viewProduct) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Generate edit URL - the {id} parameter is automatically extracted from current request
    editURL, _ := structpages.URLFor(r.Context(), editProduct{})
    // If current URL is /product/123, editURL will be "/product/123/edit"
    
    // You can still override extracted parameters if needed
    differentURL, _ := structpages.URLFor(r.Context(), editProduct{}, "456")
    // differentURL will be "/product/456/edit"
}
```

This feature works with multiple parameters as well:

```go
type pages struct {
    viewPost `route:"GET /blog/{year}/{month}/{slug} View Post"`
    editPost `route:"GET /blog/{year}/{month}/{slug}/edit Edit Post"`
}

// In a template
templ (v viewPost) Page() {
    // All parameters (year, month, slug) are automatically available
    <a href={ urlFor(ctx, editPost{}) }>Edit this post</a>
    
    // Override just one parameter while keeping others
    <a href={ urlFor(ctx, viewPost{}, map[string]any{"slug": "different-post"}) }>
        View different post in same month
    </a>
}
```

This automatic extraction eliminates the need to manually pass parameters that are already present in the current request context, making URL generation more convenient and less error-prone.

### IDFor - Consistent HTML IDs

The `IDFor` function generates consistent HTML IDs from component method references, helping maintain consistency between template IDs, HTMX targets, and component names.

#### The Problem

When building HTMX applications, you often need to keep three things in sync:
1. The ID attribute in your component template
2. The HTMX target selector in your requests
3. The component method name

```templ
// If you change "UserList" to "UserTable", you need to manually update:
<div id="user-list">...</div>  // Manual ID
<button hx-target="#user-list">Refresh</button>  // Manual target reference
templ (p TeamManagementView) UserList(users []User) { ... }  // Component name
```

#### The Solution

Use `IDFor` with method expressions to generate IDs automatically:

```templ
// In your component template
templ (p TeamManagementView) UserList(users []User) {
    <div id={ structpages.IDFor(p.UserList) }>
        <!-- content -->
    </div>
}

// In HTMX attributes
@PrimaryButton(templ.Attributes{
    "hx-get":    "/api/users",
    "hx-target": "#" + structpages.IDFor(TeamManagementView{}.UserList),
})
```

Now when you rename `UserList` to `UserTable` using your IDE's refactoring tools, all references including `p.UserList` and `TeamManagementView{}.UserList` will be automatically updated!

#### With Suffixes

For compound IDs like `user-modal-container` or `group-search-input`:

```templ
// Generate "user-modal-container"
<div id={ structpages.IDFor(p.UserModal, "container") }>
    <!-- modal content -->
</div>

// Generate "group-search-input"
<input
    id={ structpages.IDFor(p.GroupSearch, "input") }
    name="search"
/>
```

#### Naming Convention

`IDFor` converts CamelCase/PascalCase to kebab-case:

- `IDFor(p.UserList)` → `user-list`
- `IDFor(p.GroupMembers)` → `group-members`
- `IDFor(p.HTMLParser)` → `html-parser`
- `IDFor(p.UserModal, "container")` → `user-modal-container`

See [IDFOR_USAGE.md](./IDFOR_USAGE.md) for more detailed examples and usage patterns.

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
// You can also include http.ResponseWriter to set headers, cookies, etc.
func (p productPage) Props(r *http.Request, w http.ResponseWriter, store *Store) (productPageProps, error) {
    productID := r.PathValue("id")
    product, err := store.LoadProduct(productID)
    if err != nil {
        return productPageProps{}, err
    }
    
    // You can manipulate the response if needed
    w.Header().Set("X-Product-ID", productID)
    
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

**ResponseWriter Support**: Props methods can include `http.ResponseWriter` as a parameter to manipulate the response directly (e.g., setting cookies, custom headers). This must be the second parameter after `*http.Request`.

This allows you to have different props for different components:

```go
type dashboardPage struct{}

// Different props for different components
func (d dashboardPage) PageProps(r *http.Request, w http.ResponseWriter, store *Store) (PageData, error) {
    // Full page data including layout
    // Can set cookies or headers as needed
    http.SetCookie(w, &http.Cookie{Name: "dashboard_visited", Value: "true"})
    return PageData{User: store.GetUser(r), Stats: store.GetStats()}, nil
}

func (d dashboardPage) ContentProps(r *http.Request, store *Store) (ContentData, error) {
    // Just the content data for HTMX partial updates
    // Note: ResponseWriter is optional - only include if needed
    return ContentData{Stats: store.GetStats()}, nil
}

templ (d dashboardPage) Page(data PageData) {
    // Full page render
}

templ (d dashboardPage) Content(data ContentData) {
    // Partial content render
}
```

### Cross-Page Component Rendering

Structpages provides the ability to render components from different pages using the `RenderPageComponent` function. This is useful when you want to conditionally redirect rendering to a component from another page, such as error pages or shared components.

#### RenderPageComponent

The `RenderPageComponent` function can be used in Props methods to render a component from a different page:

```go
type errorPage struct{}

templ (e errorPage) ErrorComponent(message string) {
    <div class="error">
        <h2>Error</h2>
        <p>{ message }</p>
    </div>
}

type productPage struct{}

func (p productPage) Props(r *http.Request, store *Store) (string, error) {
    productID := r.PathValue("id")
    product, err := store.LoadProduct(productID)
    if err != nil {
        // Instead of returning an error, render the ErrorComponent from errorPage
        return "", structpages.RenderPageComponent(&errorPage{}, "ErrorComponent", "Product not found")
    }
    return product.Name, nil
}

templ (p productPage) Page(productName string) {
    <h1>{ productName }</h1>
    <p>Product details...</p>
}
```

#### Parameters

- `page`: The page struct instance containing the component to render
- `component`: The name of the component method to call on the specified page
- `args`: Optional arguments to pass to the component method (these replace the original Props return values)

#### Behavior

When `RenderPageComponent` is returned as an error from a Props method:

1. The framework looks up the specified page in the current router
2. Finds the requested component method on that page
3. Calls the component with the provided arguments
4. Renders the component instead of the original page's component

This pattern is particularly useful for:
- Error handling and displaying error pages
- Conditional rendering based on authentication or permissions
- Redirecting to maintenance or unavailable pages
- Sharing common components across different pages

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
    return customError{
        Code:    http.StatusMethodNotAllowed,
        Message: "Method not allowed",
    }
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

Use the `Init` method for setup (You shouldn't use `Init` for dependency injection, see below):

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
