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

`RenderComponent(SomePage.SomeMethod)` is a method-expression: the framework finds that page, applies DI, and invokes the method. This is the canonical pattern for POST/DELETE handlers that update state and return a refreshed partial *belonging to another page*. When you're rendering a component on the *same* page (you have its receiver in scope), prefer constructing the component directly — see §5.

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
<a href={ structpages.URLFor(ctx, DetailPage{}, map[string]any{"id": item.ID}) }>Detail</a>
<form action={ structpages.URLFor(ctx, SavePage{}, map[string]any{"id": item.ID}) } method="POST">
```

**Prefer `map[string]any` for path parameters.** It's explicit at the call site, survives route changes, and reads as a single value rather than a sequence of positional or alternating args. Positional and key/value-pair forms also work (see reference.md §URLFor Args Formats) but are easier to misalign during refactors.

For appending query strings, pass a `[]any` of segments — the strings are concatenated as the URL template, and the `map[string]any` fills both path and query placeholders:

```go
url, err := structpages.URLFor(ctx,
    []any{MyList{}, "?page={page}&q={q}"},
    map[string]any{"page": pageNum, "q": query},
)
```

**The recommended URLFor shape is two arguments**: `URLFor(ctx, page, params)`. Pick the right `page` form, hand off path/query placeholders in a `map[string]any` for `params`.

| form | shape | use when |
|---|---|---|
| bare typed page | `URLFor(ctx, MyPage{}, params)` | the type is mounted exactly once |
| typed chain | `URLFor(ctx, []any{Parent{}, Leaf{}}, params)` | same leaf type mounted under multiple parents — parent disambiguates |
| chain + URL fragment | `URLFor(ctx, []any{Parent{}, Leaf{}, "?tab={t}"}, params)` | need to append a query template or path suffix |
| Ref by qualified name | `URLFor(ctx, Ref("Parent.Field"), params)` | can't import the page type (cross-package import cycle) |

**Always strict.** A bare type that matches multiple nodes errors instead of silently picking one. The error lists every match and recommends the chain form. There is no opt-out — silent first-match is always wrong, so disambiguating at the call site is mandatory.

**Chain semantics.** Inside `[]any{...}`, leading typed values form a chain through the page tree: the first resolves to a node via the normal lookup; each subsequent typed value descends into a child of that type (must be unique among siblings, else error). Once a string appears, no more typed values are allowed; remaining strings concat literally to the pattern. This is the same as the existing composition slice — the new bit is that *multiple* typed values form a chain.

**Wrong shape — interleaving fails.** The slice is positional: all chain steps first, then all URL fragments. Mixing them rejects at runtime:

```go
// Wrong — typed value after a string fragment:
url, _ := structpages.URLFor(ctx,
    []any{componentsRoot{}, "?tab={tab}", entryPage{}},
    map[string]any{"slug": "x", "tab": "props"})
// → error: URLFor: typed value at slice position 2 follows a string
//   fragment; chain steps must all come before any string fragment

// Right — chain first, fragments after:
url, _ := structpages.URLFor(ctx,
    []any{componentsRoot{}, entryPage{}, "?tab={tab}"},
    map[string]any{"slug": "x", "tab": "props"})
// → "/components/x?tab=props"
```

```go
type root struct {
    Components componentsRoot `route:"/components Components"`
    Patterns   patternsRoot   `route:"/patterns Patterns"`
}
type componentsRoot struct { Detail entryPage `route:"/{slug} Component"` }
type patternsRoot   struct { Detail entryPage `route:"/{slug} Pattern"` }

// Bare URLFor errors — entryPage matches two nodes.
url, err := structpages.URLFor(ctx, entryPage{}, map[string]any{"slug": "button"})

// Chain anchors at the parent struct; descends into the entryPage child.
url, err := structpages.URLFor(ctx,
    []any{componentsRoot{}, entryPage{}},
    map[string]any{"slug": "button"})
// → "/components/button"
```

#### Validating URLs (no dangling URLs in production)

Both the chain form (field-name strings show up as type identity once compiled — but page names, route strings, and Ref strings remain stringly typed) and `Ref` carry strings somewhere. Strings are fine — they just need to be validated. Two complementary guards:

**1. Init-time validator — fails the boot, not the first request.**

```go
// validate.go
func validateURLs(sp *structpages.StructPages) error {
    var errs []error
    check := func(label string, gen func() (string, error)) {
        if _, err := gen(); err != nil {
            errs = append(errs, fmt.Errorf("%s: %w", label, err))
        }
    }
    check("home", func() (string, error) { return sp.URLFor(homePage{}) })
    check("components detail", func() (string, error) {
        return sp.URLFor([]any{componentsRoot{}, entryPage{}},
            map[string]any{"slug": "sample"})
    })
    // For Refs (cross-package, where importing would cycle):
    check("admin settings", func() (string, error) {
        return sp.URLFor(structpages.Ref("Admin.Settings"))
    })
    return errors.Join(errs...)
}

// main.go
sp, err := structpages.Mount(mux, &root{}, "/", "App")
if err != nil { log.Fatal(err) }
if err := validateURLs(sp); err != nil {
    log.Fatalf("URL validation failed:\n%v", err)
}
```

A renamed field, moved route, or broken Ref now kills the boot with the inventory of what's dangling. Same dynamic as a database migration check: refuse to start serving if the world doesn't look right.

**2. Wrap URL generation in typed helpers + an integration test.**

```go
// urls.go — one helper per URL family. The only strings live here.
func urlForGroupIndex(ctx context.Context, group string) (string, error) {
    parent, ok := groupParent(group)
    if !ok { return "", fmt.Errorf("unknown group %q", group) }
    return structpages.URLFor(ctx, []any{parent, groupIndex{}})
}

// integration_test.go — mount, render, assert URLs in the body.
func TestRenderedURLsResolve(t *testing.T) {
    mux := http.NewServeMux()
    structpages.Mount(mux, &root{}, "/", "App")
    cases := []struct{ path string; wantContains []string }{
        {"/",            []string{`href="/foundations/"`, `href="/components/"`}},
        {"/components/", []string{`href="/components/button"`}},
    }
    for _, tc := range cases {
        rec := httptest.NewRecorder()
        mux.ServeHTTP(rec, httptest.NewRequest("GET", tc.path, nil))
        for _, want := range tc.wantContains {
            if !strings.Contains(rec.Body.String(), want) {
                t.Errorf("%s body missing %q", tc.path, want)
            }
        }
    }
}
```

The helper layer narrows the surface of refactorable strings; the integration test catches drift in helpers, parents, fields, routes, or accidental ambiguity, exercised end-to-end through the real renderer.

**Why both?** The validator runs at boot — safety net for production deploys, even if CI was skipped. The integration test runs in CI — fast feedback during development, with a clearer diff when something breaks. A `TestValidateURLs` in your test file that just calls the validator gives you the validator's coverage in CI too, for one extra line.

What this catches:
- **Renamed field** in a parent struct → chain step errors with the parent's available children listed.
- **Renamed route or page** referenced by `Ref` → `no page found with route/name "..."`.
- **New page type introducing strict-mode ambiguity** → URLFor errors at the bare lookup.
- **Call site bypassing helpers** → the rendered body lacks the URL the test asserts.

See `examples/url-validation/` for the full pattern: `urls.go` (helpers), `validate.go` (init-time inventory), `integration_test.go` (end-to-end). The library's `chain_test.go` covers the URL-shape mechanics at the unit level.

#### Lint your templates and URL calls

`structpages-lint ./...` catches three classes of bug in CI:

- Dangling `URLFor` / `Ref` calls — renamed routes, ambiguous lookups, wrong params (`urlfor`, `ref`, `params`).
- Bad `ID` / `IDTarget` method expressions — receiver not mounted as a page (`id`, `idtarget`).
- **URL-bearing HTML attributes** in `.templ` files (`href`, `action`, `formaction`, `hx-{get,post,put,patch,delete}`, `hx-{push,replace}-url`) whose values are hard-coded internal paths, string concats, or `fmt.Sprint*` — i.e. cases where you should have called `structpages.URLFor` (`url-attr`). Allows `https://`, `mailto:`, `#`, `//cdn.example.com/...`.

Install once, then wire into CI alongside `go test`:

```shell
go install github.com/jackielii/structpages/tools/lint/cmd/structpages-lint@latest
structpages-lint ./...
```

Suppress a single diagnostic with a comment:

```go
//structpages:lint:ignore url-attr
url := structpages.URLFor(...)            // in .go files
```

```html
<!-- structpages:lint:ignore url-attr -->
<a href="/legacy">…</a>                    <!-- in .templ files -->
```

The directive applies to its own line and the line immediately below, so placing it above an element works the same as inline. Multiple categories are comma-separated; bare `structpages:lint:ignore` suppresses every category on that line.

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

For pages with multiple HTMX-updatable sections, inject `RenderTarget` into Props to load only the data each section needs. **Prefer constructing the component directly** — `p` is in scope, so call the method and hand the resulting component to `RenderComponent`:

```go
func (p MyPage) Props(r *http.Request, appCtx *AppContext, sel structpages.RenderTarget) (MyPageProps, error) {
    switch {
    case sel.Is(p.UserList):
        users, err := p.userListData(r, appCtx)
        if err != nil { return MyPageProps{}, err }
        return MyPageProps{}, structpages.RenderComponent(p.UserList(users))

    case sel.Is(p.GroupList):
        groups, err := p.groupListData(r, appCtx)
        if err != nil { return MyPageProps{}, err }
        return MyPageProps{}, structpages.RenderComponent(p.GroupList(groups))

    case sel.Is(p.Page), sel.Is(p.Content):
        // Full page — load everything
        return MyPageProps{Users: …, Groups: …}, nil
    }
    return MyPageProps{}, nil
}
```

Why this form: `p.UserList(users)` is a normal Go call, so the compiler checks arg types and counts. The alternative — `RenderComponent(MyPage.UserList, users)` or `RenderComponent(sel, users)` — goes through reflection inside the framework, which defers those checks to runtime. Use the reflective forms only when you genuinely don't have the receiver in scope (see §5b).

Note: only methods named `Props` are auto-invoked. `*Props`-suffixed helpers (e.g. `userListData` above; some codebases call them `UserListProps`) are *just regular methods* the user calls from inside `Props` — there's no priority resolution.

Standalone function components work the same way — just call the function:

```go
case sel.Is(UserStatsWidget):
    return MyPageProps{}, structpages.RenderComponent(UserStatsWidget(loadStats()))
```

### 5b. Cross-page RenderComponent (method expression)

When `ServeHTTP` (or another handler) needs to render a component owned by a *different* page, you don't have that page's receiver. Pass a method expression — the framework finds the page, applies DI, and invokes the method:

```go
func (p MyDelete) ServeHTTP(w http.ResponseWriter, r *http.Request, appCtx *AppContext) error {
    if err := store.Delete(...); err != nil { return err }
    items, _ := loadItems(r, appCtx)
    return structpages.RenderComponent(MyPage.ItemList, items)
}
```

This is the one case where the reflection path earns its keep: you can't construct `MyPage.ItemList(items)` directly because you don't have a `MyPage` instance, and the method may rely on DI-injected dependencies the framework will fill in.

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
5. **Prefer `RenderComponent(p.X(args))` to `RenderComponent(MyPage.X, args)` or `RenderComponent(target, args)`** when the receiver is in scope — direct construction is compile-time-checked; the reflective forms defer arg/type checks to runtime. The reflective forms are still correct, just slower and more error-prone; reserve them for cross-page renders where you don't have the receiver.
6. **Children are registered before parents** on the mux (so nested-route conflicts resolve correctly).
7. **Promoted (embedded) methods are skipped** — only methods defined directly on the struct count.
8. **URL params auto-fill from current request's route only** — sibling routes with different param names do not auto-fill.
9. **`ErrSkipPageRender` is only honored from `Props`** (e.g. after writing a redirect). Returning it from `ServeHTTP` does nothing special.
10. **Disambiguation primitives:** when the same page type is mounted under multiple parents, use the `[]any{ParentPage{}, LeafPage{}}` chain form — strict `URLFor` (the default) errors on bare lookups. Use `structpages.Ref("Parent.Field")` (qualified path) or `Ref("PageName")` when a package needs to URL-to a page it can't import (importing would cycle); Ref also handles Go type aliases that collapse to one `reflect.Type`. Ref strings are validated at startup via the init-time validator pattern (see §3 "Validating URLs").
11. **Plain strings pass through `ID` and `IDTarget` unchanged** — `IDTarget("body")` returns `"body"`, not `"#body"`.
12. **The `form:` struct tag is not read by the framework** — only `route:` is. Anything else on a route field is ignored.
13. **Never write `w` (e.g. `http.Error`) in `Props` or an error-returning `ServeHTTP`** — they are buffered; return the error instead. Use a typed error like `ErrorWithStatus` for a specific status code. API/JSON endpoints use the no-error `ServeHTTP(w, r, deps...)` form, where direct writes are correct (see examples.md §13).
