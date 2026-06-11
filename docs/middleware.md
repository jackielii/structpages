---
title: Middleware
slug: /middleware
sidebar_position: 10
---

# Middleware Usage

`MiddlewareFunc` is standard Go middleware that also receives the route's `*PageNode`, so middleware can inspect page metadata:

```go
type MiddlewareFunc func(next http.Handler, pn *structpages.PageNode) http.Handler
```

## Global middleware

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

## Page middlewares

Implement the `Middlewares()` method to add middleware to a specific page; it also applies to all descendant routes:

```go
type protectedPages struct {
    // children pages will be protected
}

func (p protectedPages) Middlewares() []structpages.MiddlewareFunc {
    return []structpages.MiddlewareFunc{
        requireAuth,
        checkPermissions,
    }
}
```

`Middlewares()` can take injected dependencies (matched by type from `WithArgs`):

```go
func (p protectedPages) Middlewares(sm *SessionManager) []structpages.MiddlewareFunc {
    return []structpages.MiddlewareFunc{
        func(next http.Handler, pn *structpages.PageNode) http.Handler {
            return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                if !sm.Exists(r.Context(), "user") {
                    redirectToLogin(w, r)
                    return
                }
                next.ServeHTTP(w, r)
            })
        },
    }
}

// Middleware is outside the error-return path, so do the HTMX check here:
// a 3xx during an HTMX request would be swapped into the partial's target.
func redirectToLogin(w http.ResponseWriter, r *http.Request) {
    loginURL, err := structpages.URLFor(r.Context(), loginPage{})
    if err != nil {
        // http.Error is acceptable here only because middleware sits outside
        // structpages' error handling.
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    if r.Header.Get("HX-Request") == "true" {
        w.Header().Set("HX-Location", loginURL) // ajax navigation; status must stay 2xx
        return
    }
    http.Redirect(w, r, loginURL, http.StatusSeeOther)
}
```

Note the login URL comes from `URLFor`, not a string literal — when the login route moves, this middleware follows. Handler methods themselves should redirect via the [`Redirect` control-flow signal](./error-handling.md) instead; the inline check is only needed here because middleware runs outside the error-return path.

Example logging middleware using the `PageNode`:

```go
func loggingMiddleware(next http.Handler, pn *structpages.PageNode) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        log.Printf("%s %s (%s) took %v", r.Method, r.URL.Path, pn.Title, time.Since(start))
    })
}
```

## Middleware execution order

The framework prepends two implicit middlewares to every route, then layers the user-supplied chain on top. The final order, from outermost (runs first on the request, last on the response) to innermost:

1. **Framework: `withPcCtx`** — injects the parse context into `r.Context()` so `URLFor` / `ID` / `IDTarget` work in handlers.
2. **Framework: `extractURLParams`** — pre-extracts the current request's path params into context for `URLFor` auto-fill.
3. **Global middlewares from `WithMiddlewares(...)`** — first item is outermost.
4. **Page-specific middlewares from `Middlewares()`** — accumulate down the page tree (parent's middlewares wrap children's).
5. **The page handler** — innermost.

Middleware execution forms an "onion": the outermost middleware sees the request first and the response last.

Because of the auto-injected middlewares, you don't need to do anything to make `URLFor` and `ID`/`IDTarget` work from inside your handlers and middleware — the parse context and current-route params are already in `r.Context()`.
