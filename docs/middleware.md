# Middleware Usage

### Global Middleware

Apply middleware to all routes:

```go
mux := http.NewServeMux()
sp, err := structpages.Mount(mux, pages{}, "/", "My App",
    structpages.WithMiddlewares(
        loggingMiddleware,
        authMiddleware,
    ),
)
if err != nil {
    log.Fatal(err)
}
```

### Page Middlewares

Implement the `Middlewares()` method to add middleware to a specific page; it also applies to all descendant routes:

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

`Middlewares()` can take injected dependencies (matched by type from `WithArgs`):

```go
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

The framework prepends two implicit middlewares to every route, then layers the user-supplied chain on top. The final order, from outermost (runs first on the request, last on the response) to innermost:

1. **Framework: `withPcCtx`** — injects the parse context into `r.Context()` so `URLFor` / `ID` / `IDTarget` work in handlers.
2. **Framework: `extractURLParams`** — pre-extracts the current request's path params into context for `URLFor` auto-fill.
3. **Global middlewares from `WithMiddlewares(...)`** — first item is outermost.
4. **Page-specific middlewares from `Middlewares()`** — accumulate down the page tree (parent's middlewares wrap children's).
5. **The page handler** — innermost.

Middleware execution forms an "onion": the outermost middleware sees the request first and the response last. The `TestMiddlewareOrder` test in the codebase validates this behavior.

Note: because of the auto-injected middlewares, you don't need to do anything to make `URLFor` and `ID`/`IDTarget` work from inside your handlers — the parse context and current-route params are already in `r.Context()`.

