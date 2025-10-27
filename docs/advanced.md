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
mux := http.NewServeMux()

store := &Store{db: db}
sessionManager := NewSessionManager()

// Services are passed as additional arguments to Mount
sp, err := structpages.Mount(mux, pages{}, "/", "My App",
    store,           // Will be available in page & other methods
    sessionManager,  // Will be available in page & other methods
    logger,          // Any other dependencies
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
    "api-key",      // First string
    "db-name",      // Second string - will cause error
)
if err != nil {
    // Error: duplicate type string in args registry
}

// DO this instead - create distinct types
type APIKey string
type DatabaseName string

sp, err := structpages.Mount(mux, pages{}, "/", "My App",
    APIKey("your-api-key"),
    DatabaseName("mydb"),
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

#### IDFor with Ref

Use `Ref` to reference component methods dynamically:

```go
// Qualified reference (PageName.MethodName)
id, err := IDFor(ctx, Ref("userPage.UserList"))
// → "#user-page-user-list"

// Simple method name (must be unambiguous)
id, err := IDFor(ctx, Ref("UserList"))
// → "#user-page-user-list" (if only one page has UserList)

// With IDParams for suffixes and raw IDs
id, err := IDFor(ctx, IDParams{
    Method:   Ref("userPage.UserModal"),
    Suffixes: []string{"container"},
    RawID:    true,
})
// → "user-page-user-modal-container"
```

**Matching rules for IDFor:**
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
        targetID, _ := IDFor(ctx, Ref(config.TargetComponent))
    }
    <form hx-post={ actionURL } hx-target={ targetID }>
        <button>Submit</button>
    </form>
}
```

#### Error Handling

Both URLFor and IDFor with `Ref` return descriptive errors for runtime safety:

```go
// Page not found by name
url, err := URLFor(ctx, Ref("NonExistentPage"))
// Error: "no page found with name \"NonExistentPage\""

// Page not found by route
url, err := URLFor(ctx, Ref("/bad/route"))
// Error: "no page found with route \"/bad/route\""

// Method not found
id, err := IDFor(ctx, Ref("NonExistentMethod"))
// Error: "method \"NonExistentMethod\" not found on any page"

// Ambiguous method (exists on multiple pages)
id, err := IDFor(ctx, Ref("UserList"))
// Error: "method \"UserList\" found on multiple pages: userPage, adminPage.
//         Use qualified name like \"userPage.UserList\""

// Method not on specified page
id, err := IDFor(ctx, Ref("userPage.AdminSettings"))
// Error: "method \"AdminSettings\" not found on page \"userPage\""
```

**Testing Dynamic References**

```go
func TestMenuReferences(t *testing.T) {
    // Mount pages
    mux := http.NewServeMux()
    sp, _ := structpages.Mount(mux, &pages{}, "/", "App")

    // Get context with parseContext
    ctx := sp.Context()

    // Verify all menu items reference valid pages
    for _, item := range menu {
        _, err := structpages.URLFor(ctx, item.PageRef)
        if err != nil {
            t.Errorf("Invalid menu item %s: %v", item.Label, err)
        }
    }
}
