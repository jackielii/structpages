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
