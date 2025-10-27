# Middleware Usage

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

