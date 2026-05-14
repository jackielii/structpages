---
name: structpages
description: >
  Guide for building Go web applications with the structpages framework (struct-based routing + templ + HTMX).
  Use when writing routes, page handlers, Props methods, templ templates, HTMX partial rendering,
  URL generation (URLFor/ID/IDTarget), RenderTarget/RenderComponent patterns, or middleware with structpages.
  Also use when the user asks about structpages patterns, conventions, or debugging structpages issues.
---

# structpages Framework Guide

structpages provides struct-based routing for Go web apps, integrating with `http.ServeMux`, templ templating, and HTMX.

## Quick Reference

For detailed API docs, see [reference.md](reference.md).
For real-world patterns and examples, see [examples.md](examples.md).

## Core Concepts

### 1. Route Definition

Routes are struct fields with `route:` tags. Format: `route:"[METHOD] /path [Title]"`

```go
type pages struct {
    home    `route:"/{$}   Home"`            // exact root match
    about   `route:"/about About"`           // all methods (default)
    create  `route:"POST /create Create"`    // POST only
    detail  `route:"/item/{id} Item"`        // path parameter
    files   `route:"/files/{path...} Files"` // wildcard
}
```

If no method is given, the route accepts all methods (internally stored as `"ALL"`).

Nesting creates URL hierarchies:

```go
type pages struct {
    admin adminPages `route:"/admin Admin"`
}
type adminPages struct {
    dashboard `route:"/{$} Dashboard"`    // -> /admin/
    users     `route:"/users Users"`      // -> /admin/users
}
```

**Mounting a module's static-asset subtree alongside its pages.** Use the wildcard form for prefix subtrees — `path.Join` strips trailing slashes when computing the full route, so `route:"/static/"` registers as an exact `GET /admin/static` (no prefix match). Use `{path...}` instead:

```go
type adminPages struct {
    dashboard `route:"/{$} Dashboard"`
    users     `route:"/users Users"`
    Assets    staticFiles `route:"GET /static/{path...} Assets"`
}

//go:embed all:static
var staticFS embed.FS
var staticRoot = must(fs.Sub(staticFS, "static"))

type staticFiles struct{}
func (staticFiles) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    http.ServeFileFS(w, r, staticRoot, r.PathValue("path"))
}
```

This keeps the module self-contained: `/admin` and `/admin/static/*` register together, with no separate `pub.Handle("/admin/static/", …)` call to keep in sync. See examples.md §12 for the full pattern, including middleware and link-side considerations.

### 2. Page Handler Patterns

There are three main patterns — choose based on what the page does.

**Pattern A: Props + Page/Content (renders HTML)**

```go
type MyPage struct{}

type MyPageProps struct {
    Items []Item
}

// Props fetches data. Parameters are type-matched via DI.
func (p MyPage) Props(r *http.Request, appCtx *AppContext) (MyPageProps, error) {
    items, err := appCtx.Store.GetItems(r.Context())
    if err != nil {
        return MyPageProps{}, err
    }
    return MyPageProps{Items: items}, nil
}

// Page wraps in layout (used for full page loads — non-HTMX, or HTMX with no matching target)
templ (p MyPage) Page(props MyPageProps) {
    @AppShellLayout() {
        @p.Content(props)
    }
}

// Content renders the body (used by convention for HTMX partials targeting "#…content")
templ (p MyPage) Content(props MyPageProps) {
    <div>...</div>
}
```

**Pattern B: ServeHTTP that writes, then re-renders a sibling component (most common HTMX form action)**

```go
type AddTodo struct{}

func (a AddTodo) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
    text := r.FormValue("text")
    if text != "" {
        store.Add(text)
    }
    // Render the sibling page's TodoList component as the response
    return structpages.RenderComponent(Index.TodoList)
}
```

`RenderComponent(SomePage.SomeMethod)` resolves to that page's component and renders it. This is the canonical pattern for POST/DELETE handlers that update state and return a refreshed partial.

**Pattern C: ServeHTTP for redirects (no HTML response)**

```go
type SubmitForm struct{}

func (p SubmitForm) ServeHTTP(w http.ResponseWriter, r *http.Request, appCtx *AppContext) error {
    // perform action...
    http.Redirect(w, r, "/somewhere", http.StatusSeeOther)
    return nil
}
```

**Pattern D: ServeHTTP for API/JSON endpoints (no error return)**

API endpoints use the **no-error** form so writes go straight to the wire (unbuffered) and the framework's HTML error handler stays out of it:

```go
type TrackTime struct{}

func (p TrackTime) ServeHTTP(w http.ResponseWriter, r *http.Request, appCtx *AppContext) {
    var body trackTimeRequest
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }
    if err := appCtx.Store.UpdateTime(r.Context(), body); err != nil {
        http.Error(w, "update failed", http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusOK)
}
```

`ServeHTTP` supports four signatures (see reference.md for details). The DI form (extra arg types beyond `w, r`) buffers the response only when the method has a return value.

### Error handling in handlers

The error-returning forms of `ServeHTTP` — and **every** `Props` method — run against a *buffered* response writer. On a non-nil error the buffer is discarded and the error goes to the `WithErrorHandler` callback. So:

- **Never call `http.Error` (or write `w`) in an error-returning handler or in `Props`.** If you write then `return err`, the write is discarded; if you write then `return nil`, you bypass the error handler. Just return the error.
- **For a specific status code, return a typed error** (e.g. `ErrorWithStatus{Status, Title, Message}`) that the global handler unwraps with `errors.As`. Plain errors fall through to a logged 500.
- **API/JSON endpoints use Pattern D** (no error return) — there `http.Error` and direct `w` writes are correct, because you own the status code and skip the buffering wrapper.
- **For streaming (SSE), flush with `http.NewResponseController(w)`** — it works from either `ServeHTTP` form (the buffered wrapper implements `FlushError()`/`Unwrap()`) and is the only way to *guarantee* unbuffered delivery through other middleware.

See examples.md §13 for the full pattern, including the `WithErrorHandler` wiring.

### 3. URL Generation

`structpages.URLFor(ctx, page, args...)` returns `(string, error)`. In templ, attribute values can take `(string, error)` directly:

```templ
<a href={ structpages.URLFor(ctx, MyPage{}) }>Link</a>
<a href={ structpages.URLFor(ctx, DetailPage{}, item.ID) }>Detail</a>
<form action={ structpages.URLFor(ctx, SavePage{}, item.ID) } method="POST">
```

For appending query strings, pass a `[]any` of segments:

```go
url, err := structpages.URLFor(ctx,
    []any{MyList{}, "?page={page}&q={q}"},
    "page", pageNum, "q", query,
)
```

When you need a plain string (not in a templ attribute that handles errors), wrap with a small `must` helper:

```go
func must[T any](v T, err error) T {
    if err != nil { panic(err) }
    return v
}

myURL := must(structpages.URLFor(ctx, MyPage{}))
```

**Optional convenience wrappers.** Some apps define short local wrappers like `urlFor`, `idFor`, `idForTarget` — e.g. to return `templ.SafeURL` or to shorten the package qualifier. These are app-level conveniences, not framework functions:

```go
// in your app — purely optional
func urlFor(ctx context.Context, page any, args ...any) (templ.SafeURL, error) {
    s, err := structpages.URLFor(ctx, page, args...)
    return templ.SafeURL(s), err
}
func idFor(ctx context.Context, v any) (string, error)        { return structpages.ID(ctx, v) }
func idForTarget(ctx context.Context, v any) (string, error)  { return structpages.IDTarget(ctx, v) }
```

The rest of this guide uses the framework names (`structpages.URLFor`, `structpages.ID`, `structpages.IDTarget`) directly.

### 4. HTMX Partial Rendering

All HTMX requests for a page go to the SAME route. structpages picks which component to render from the `HX-Target` header by matching element IDs against component method names.

`structpages.ID` / `structpages.IDTarget` generate deterministic element IDs from method references (`ID` returns `"my-page-user-list"`; `IDTarget` returns `"#my-page-user-list"`). For plain string arguments both functions return the string unchanged — `IDTarget("body")` is `"body"`, not `"#body"`.

```templ
// Set element ID on the component's wrapper
<div id={ structpages.ID(ctx, MyPage.UserList) }>
    @p.UserList(props.Users)
</div>

// HTMX targeting
<input hx-get={ structpages.URLFor(ctx, MyPage{}) }
       hx-target={ structpages.IDTarget(ctx, MyPage.UserList) }
       hx-swap="outerHTML" />
```

### 5. RenderTarget Pattern

For pages with multiple HTMX-updatable sections, inject `RenderTarget` into Props to load only the data each section needs:

```go
func (p MyPage) Props(r *http.Request, appCtx *AppContext, sel structpages.RenderTarget) (MyPageProps, error) {
    switch {
    case sel.Is(MyPage.UserList):
        users, err := p.userListData(r, appCtx)
        if err != nil { return MyPageProps{}, err }
        return MyPageProps{}, structpages.RenderComponent(MyPage.UserList, users)

    case sel.Is(MyPage.GroupList):
        groups, err := p.groupListData(r, appCtx)
        if err != nil { return MyPageProps{}, err }
        return MyPageProps{}, structpages.RenderComponent(MyPage.GroupList, groups)

    case sel.Is(MyPage.Page), sel.Is(MyPage.Content):
        // Full page — load everything
        return MyPageProps{Users: …, Groups: …}, nil
    }
    return MyPageProps{}, nil
}
```

Note: only methods named `Props` are auto-invoked. `*Props`-suffixed helpers (e.g. `userListData` above; some codebases call them `UserListProps`) are *just regular methods* the user calls from inside `Props` — there's no priority resolution.

`RenderComponent` in `ServeHTTP` for write+rerender flows:

```go
func (p MyDelete) ServeHTTP(w http.ResponseWriter, r *http.Request, appCtx *AppContext) error {
    if err := store.Delete(...); err != nil { return err }
    items, _ := loadItems(r, appCtx)
    return structpages.RenderComponent(MyPage.ItemList, items)
}
```

For function targets specifically (standalone templ funcs like `UserStatsWidget`), `target.Is(fn)` *must* be called before `RenderComponent(target, args...)` — `Is()` stores the function pointer for later rendering. For method targets, `Is()` is the recommended pattern but not strictly required (the method is captured at construction).

### 6. Middleware

Global middleware via `WithMiddlewares`. Page-specific via `Middlewares()` method (also applies to all descendants):

```go
func (p ProtectedPages) Middlewares(appCtx *AppContext) []structpages.MiddlewareFunc {
    return []structpages.MiddlewareFunc{RequireAuth(appCtx)}
}
```

`MiddlewareFunc` signature: `func(http.Handler, *PageNode) http.Handler` — receives the `PageNode` so middleware can inspect route metadata.

### 7. Dependency Injection

Register deps via `WithArgs`. They're matched by type into method parameters:

```go
sp, err := structpages.Mount(mux, TopPages{}, "/", "App",
    structpages.WithArgs(appCtx),
)

// Now any Props/ServeHTTP/Middlewares/Init method can receive *AppContext
func (p MyPage) Props(r *http.Request, appCtx *AppContext) (Props, error) { ... }
```

Each registered type appears once. The matcher coerces between pointer and value forms and falls back to assignability, so a single `*AppContext` registration also satisfies parameters of any interface it implements. To register two values of the same underlying type, define named types to disambiguate.

Generic types and interface types are supported as well — see `generics_injection_test.go` for the tested matrix (pointer semantics, interface injection, slices/maps, complex constraints, type parameters).

`*PageNode` is always available for injection (the framework adds the current node automatically).

## Key Rules

1. **Props methods extract path params** via `r.PathValue("param")`, not function arguments.
2. **Never hardcode URLs** — always use `structpages.URLFor`.
3. **Partial templ methods** take ONLY their specific data, not the full props struct.
4. **`RenderComponent` is returned as an error** — when returned, the Props return values (other than the error) are ignored.
5. **`target.Is(fn)` is required** before `RenderComponent(target, args...)` for *function* targets; recommended for method targets.
6. **Children are registered before parents** on the mux (so nested-route conflicts resolve correctly).
7. **Promoted (embedded) methods are skipped** — only methods defined directly on the struct count.
8. **URL params auto-fill from current request's route only** — sibling routes with different param names do not auto-fill.
9. **`ErrSkipPageRender` is only honored from `Props`** (e.g. after writing a redirect). Returning it from `ServeHTTP` does nothing special.
10. **Type aliases break URLFor's type-based lookup** — use `structpages.Ref("FieldName")` to disambiguate.
11. **Plain strings pass through `ID` and `IDTarget` unchanged** — `IDTarget("body")` returns `"body"`, not `"#body"`.
12. **The `form:` struct tag is not read by the framework** — only `route:` is. Anything else on a route field is ignored.
13. **Never write `w` (e.g. `http.Error`) in `Props` or an error-returning `ServeHTTP`** — they are buffered; return the error instead. Use a typed error like `ErrorWithStatus` for a specific status code. API/JSON endpoints use the no-error `ServeHTTP(w, r, deps...)` form, where direct writes are correct (see examples.md §13).
