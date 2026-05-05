# Advanced Features

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
            // Response is buffered, so the error handler can render
            // an error page even though we already wrote partial output
            return err
        }
        http.Redirect(w, r, "/success", http.StatusSeeOther)
        return nil
    }

    // Any non-nil return goes to the configured error handler
    // (see WithErrorHandler). The framework does not auto-map error
    // types to status codes — your error handler decides.
    return fmt.Errorf("method not allowed: %s", r.Method)
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

Use the `Init` method for one-time setup at `Mount` time. `Init` may take an `error` return and may receive injected dependencies (matched by type from `WithArgs`):

```go
type databasePage struct {
    db *sql.DB
}

func (d *databasePage) Init(store *Store) error {
    // Called once during Mount. Errors abort Mount.
    d.db = store.DB()
    return nil
}
```

Either value or pointer receiver works; use pointer if `Init` mutates the page (the typical case). Prefer `WithArgs` for runtime dependencies — `Init` is for setup that has to happen exactly once and isn't naturally a method parameter.

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

// Pass services when mounting pages — wrap them in WithArgs(...)
mux := http.NewServeMux()

store := &Store{db: db}
sessionManager := NewSessionManager()

// Services are registered via the WithArgs option (Mount's variadic
// param is options ...Option, not raw values)
sp, err := structpages.Mount(mux, pages{}, "/", "My App",
    structpages.WithArgs(store, sessionManager, logger),
)
if err != nil {
    log.Fatal(err)
}
```

**Important:** Dependency injection is type-based. Each type can only be registered once. Attempting to register duplicate types will result in an error. If you need to inject multiple values of the same underlying type (e.g., multiple strings), create distinct types:

```go
// DON'T do this - will return an error for duplicate type
mux := http.NewServeMux()
_, err := structpages.Mount(mux, pages{}, "/", "My App",
    structpages.WithArgs(
        "api-key",  // First string
        "db-name",  // Second string - will cause error
    ),
)
if err != nil {
    // Error: duplicate type string in args registry
}

// DO this instead - create distinct types
type APIKey string
type DatabaseName string

sp, err := structpages.Mount(mux, pages{}, "/", "My App",
    structpages.WithArgs(
        APIKey("your-api-key"),
        DatabaseName("mydb"),
    ),
)
if err != nil {
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

**Type matching with coercion.** The argument registry coerces between pointer and value forms and falls back to assignability (`args.go`). One concrete consequence: a single `*AppContext` registration can fill a parameter typed as any interface that `*AppContext` implements. So you can register concrete types and have handler methods declare interface parameters (good for testability).

**Generic types and interface types in DI.** Both work — `generics_injection_test.go` covers basic injection, type parameters, slices/maps as deps, type aliases, function types, complex constraints, pointer semantics, and interface injection (12 tests). Anywhere these docs say "type", read it as "any reflect-distinguishable type, including generics and interfaces".

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
```

### Dynamic References with Ref

The `Ref` type enables dynamic references to pages and methods when static type references aren't available. This is useful for configuration-driven menus, generic components, and scenarios where page or method names are determined at runtime.

#### URLFor with Ref

Use `Ref` to reference pages dynamically:

```go
// Reference by page name (struct field name)
url, err := URLFor(ctx, Ref("homePage"))
// → "/"

// Reference by route path (must start with /)
url, err := URLFor(ctx, Ref("/user/settings"))
// → "/user/settings"

// With path parameters
url, err := URLFor(ctx, Ref("productPage"), "123")
// → "/product/123"

// Compose with literals
url, err := URLFor(ctx, []any{Ref("userPage"), "?tab=profile"})
// → "/user?tab=profile"
```

**Matching rules for URLFor:**
- If `Ref` starts with `/`, matches by full route
- Otherwise, matches by page name (the struct field name)
- Returns error if no match found

**Example: Dynamic Menu**

```go
type MenuItem struct {
    PageRef Ref
    Label   string
}

var menu = []MenuItem{
    {Ref("home"), "Home"},
    {Ref("users"), "Users"},
    {Ref("settings"), "Settings"},
}

templ Navigation(ctx context.Context) {
    <nav>
        for _, item := range menu {
            <a href={ URLFor(ctx, item.PageRef) }>{ item.Label }</a>
        }
    </nav>
}
```

#### ID/IDTarget with Ref

Use `Ref` to reference component methods dynamically:

```go
// Qualified reference (PageName.MethodName)
id, err := ID(ctx, Ref("userPage.UserList"))
// → "user-page-user-list"

idTarget, err := IDTarget(ctx, Ref("userPage.UserList"))
// → "#user-page-user-list"

// Simple method name (must be unambiguous)
id, err := ID(ctx, Ref("UserList"))
// → "user-page-user-list" (if only one page has UserList)
```

**Matching rules for ID/IDTarget:**
- If `Ref` contains `.`, splits into `PageName.MethodName` (qualified)
- Otherwise, searches all pages for the method name
  - If found on one page, returns that page's ID
  - If found on multiple pages, returns error with helpful message
- Verifies method exists on resolved page
- Returns error if not found

**Example: Configuration-Driven Form**

```go
type FormConfig struct {
    ActionPage      string  // From config file
    TargetComponent string  // From config file
}

templ DynamicForm(ctx context.Context, config FormConfig) {
    @{
        actionURL, _ := URLFor(ctx, Ref(config.ActionPage))
        targetID, _ := IDTarget(ctx, Ref(config.TargetComponent))
    }
    <form hx-post={ actionURL } hx-target={ targetID }>
        <button>Submit</button>
    </form>
}
```

#### Error Handling

Both URLFor and ID/IDTarget with `Ref` return descriptive errors for runtime safety:

```go
// Page not found by name
url, err := URLFor(ctx, Ref("NonExistentPage"))
// Error: "no page found with name \"NonExistentPage\""

// Page not found by route
url, err := URLFor(ctx, Ref("/bad/route"))
// Error: "no page found with route \"/bad/route\""

// Method not found
id, err := ID(ctx, Ref("NonExistentMethod"))
// Error: "method \"NonExistentMethod\" not found on any page"

// Ambiguous method (exists on multiple pages)
id, err := ID(ctx, Ref("UserList"))
// Error: "method \"UserList\" found on multiple pages: userPage, adminPage.
//         Use qualified name like \"userPage.UserList\""

// Method not on specified page
id, err := ID(ctx, Ref("userPage.AdminSettings"))
// Error: "method \"AdminSettings\" not found on page \"userPage\""
```

**Testing Dynamic References**

Outside a request context, use the methods on the returned `*StructPages` value (which has its own access to the parse context):

```go
func TestMenuReferences(t *testing.T) {
    mux := http.NewServeMux()
    sp, _ := structpages.Mount(mux, &pages{}, "/", "App")

    // Verify all menu items reference valid pages
    for _, item := range menu {
        _, err := sp.URLFor(item.PageRef)
        if err != nil {
            t.Errorf("Invalid menu item %s: %v", item.Label, err)
        }
    }
}
```

Within a request handler, use the context-based functions: `structpages.URLFor(r.Context(), ...)`. The framework auto-injects the parse context into the request context via internal middleware.

### Type Aliases and URLFor/IDFor

Go type aliases (`type X = Y`) are identical at runtime. This has implications when the same type (or its alias) is used for multiple routes.

#### The Limitation

```go
type productPage struct{}

func (productPage) Page() templ.Component { return productTemplate() }

// Type alias - identical to productPage at runtime
type featuredProduct = productPage

type pages struct {
    products productPage     `route:"/products Products"`
    featured featuredProduct `route:"/featured Featured"`
}
```

When using `URLFor` with type references:

```go
// Both return "/products" - the first matching route
url1, _ := URLFor(ctx, productPage{})     // → "/products"
url2, _ := URLFor(ctx, featuredProduct{}) // → "/products" (not "/featured"!)
```

This happens because `reflect.TypeOf(featuredProduct{})` returns the same type as `reflect.TypeOf(productPage{})`. Go's reflection cannot distinguish between a type and its alias.

#### The Workaround: Use Ref

Use `Ref("fieldName")` to target routes by their struct field name instead of type:

```go
// Target specific routes using field names
url1, _ := URLFor(ctx, Ref("products")) // → "/products"
url2, _ := URLFor(ctx, Ref("featured")) // → "/featured" ✓

// Same for IDFor/IDTarget
id1, _ := IDTarget(ctx, Ref("products.Page")) // → "#products-page"
id2, _ := IDTarget(ctx, Ref("featured.Page")) // → "#featured-page" ✓
```

#### When This Matters

This limitation only affects scenarios where:
1. The same type (or its alias) is used for multiple routes
2. You need to reference a specific route using `URLFor` or `IDFor`

If each route uses a unique type, type-based matching works perfectly:

```go
type productsPage struct{}
type featuredPage struct{}  // Different type, not an alias

type pages struct {
    products productsPage `route:"/products Products"`
    featured featuredPage `route:"/featured Featured"`
}

// Works correctly - different types
url1, _ := URLFor(ctx, productsPage{}) // → "/products"
url2, _ := URLFor(ctx, featuredPage{}) // → "/featured" ✓
```

#### Best Practices

1. **Use distinct types** for routes that need individual `URLFor`/`IDFor` references
2. **Use `Ref("fieldName")`** when you must use the same type for multiple routes
3. **Type aliases are fine** for routing and rendering - only `URLFor`/`IDFor` type matching is affected
