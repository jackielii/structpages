---
name: structpages
description: >
  Guide for building Go web applications with the structpages framework (struct-based routing + templ + HTMX).
  Use when writing routes, pages, page groups, Props methods, handler methods (ServeHTTP), page components,
  partials, HTMX partial rendering and nested swap levels, URL generation (URLFor/ID/IDTarget),
  RenderTarget/RenderComponent patterns, or middleware with structpages.
  Also use when the user asks about structpages patterns, conventions, vocabulary, or debugging structpages issues.
---

# structpages Framework Guide

structpages provides struct-based routing for Go web apps, integrating with `http.ServeMux`, templ templating, and HTMX.

## Quick Reference

For detailed API docs, see [reference.md](reference.md).
For real-world patterns and examples, see [examples.md](examples.md).

## Vocabulary

structpages has its own canonical terms for its recurring patterns. Where a React / Next.js / React Router concept maps cleanly, it's noted as a cross-reference for knowledge transfer — but the structpages term is primary. Two guardrails: Go wins where Go owns the concept (`ServeHTTP` is a **handler method**, not a "server action"), and pure composition isn't named (a layout is just a **component** that takes **children** — there's no "layout route").

### Core nouns

| Term | What it is | Cross-ref |
|---|---|---|
| **page** | a route-tagged struct — a node in the route tree | Next/RR route/page |
| **page group** | a page with no render of its own (no `Page` or `ServeHTTP`), only child pages; served through its `/{$}` page | — (not a "layout route") |
| **component** | a standalone `templ Foo()` block — reusable, mount-independent, package-prefixed id | React component |
| **page component** | a `templ (p Page) Foo()` method — mount-aware, receiver in scope (incl. `Page`, `Content`). Used two ways: **composition** (called inside another page component) and **re-rendering** (returned alone as a partial) | React component (bound) |
| **children** | templ `{ children... }` composition | React children |
| **partial** | a page component returned on its own as an HTMX response to re-render just that region — a *role* a page component plays, not a distinct kind | HTMX |

### The props cluster

| Term | What it is | Cross-ref |
|---|---|---|
| **Props method** | the `Props(...)` method that loads data via DI | *like RR `loader` / Next `getServerSideProps`* |
| **props struct** | the named struct type the Props method returns and page components accept | *like a React props type* |
| **props** | a value of the props struct, in flight into a page component | React props (the value) |

The chain reads: the **Props method** returns a **props struct**; that **props** value is handed to a **page component**.

### Methods on a page

| Term | Method | Job |
|---|---|---|
| **Page method** | `Page(props)` | the main render entry — a page component that composes the full page (layout + content) |
| **Props method** | `Props(...)` | loads data via DI → returns the props struct |
| **handler method** | `ServeHTTP(...)` | imperative entry: mutate / redirect / serve JSON, or render a partial via `RenderComponent` — the Go `http.Handler` shape |
| **Middlewares method** | `Middlewares()` | declares middleware for the page + descendants |

(`Content` is not a framework concept — just a conventional page component name for a layout's main region; the matcher treats it like any other page component.)

The two render entries differ in flavor: the **Page method** renders declaratively (compose page components); the **handler method** renders imperatively (write the response, or hand a page component to `RenderComponent`). Both ultimately render through page components.

### API helpers (literal — these are the public API)

`RenderComponent`, `RenderTarget`, `URLFor`, `ID` / `IDTarget`, `Ref`, `WithArgs` (dependency injection / **args**).

### Loose comparisons (analogies, not structpages terms)

For readers arriving from React/Next — transfer aids, not structpages vocabulary.

| structpages | React/Next analogy | note |
|---|---|---|
| `/{$}` route of a page group | RR **index route** | nothing special — just the group's own page |
| **Page method** vs **handler method** | declarative `page` vs imperative **Route Handler / API route** | two ways to respond within one router — **not** "Page Router vs App Router" |
| **component** composition | Server Component composition | both render on the server |

## Request Lifecycle

For a rendering page: **route match → Props method** (with `RenderTarget` injected to pick the region) **→ page component render** — `Page` for full loads, a partial for HTMX requests targeting that region's id. A handler method (`ServeHTTP`) bypasses this pipeline: it responds imperatively, optionally handing a page component to `RenderComponent`.

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

**Mounting a module's static-asset subtree alongside its pages.** Use the wildcard form for prefix subtrees — `path.Join` strips trailing slashes when computing the full route, so `route:"/static/"` registers as an exact `GET /admin/static` (no prefix match). Mount `route:"GET /static/{path...} Assets"` on a small `ServeHTTP` page serving an embedded FS instead. This keeps the module self-contained: `/admin` and `/admin/static/*` register together, with no separate `pub.Handle(…)` call to keep in sync. Full pattern (embed, middleware, link-side considerations): examples.md §12.

### 2. Page Response Patterns

There are four main shapes — choose based on what the page does. The first renders declaratively (Props method + Page method); the other three are handler methods (`ServeHTTP`).

**A page that renders: Props method + Page method**

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

For regions inside `Content` that must swap independently (master-detail panes, dialogs), add inner levels — see §5c.

**A handler method that returns a partial (most common HTMX form action)**

```go
type AddTodo struct{}

func (a AddTodo) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
    text := r.FormValue("text")
    if text != "" {
        store.Add(text)
    }
    // Construct the refreshed partial and return it as the response
    return structpages.RenderComponent(Index{}.TodoList(store.List()))
}
```

This is the canonical pattern for POST/DELETE handlers that update state and return a refreshed partial. **Pass a constructed component** — a normal Go call the compiler checks. Page structs are stateless, so a zero-value receiver (`Index{}`) constructs a *sibling* page's component just as well as your own. The reflective method-expression form (`RenderComponent(Index.TodoList)`) is reserved for components whose parameters the framework should DI-inject — see §5b.

**A handler method that redirects (no HTML response)**

Don't call `http.Redirect` directly in an HTMX app — during an HTMX request the XHR follows the 3xx and swaps the redirect *target's* body into the partial's swap target. Return a control-flow signal instead and let the global error handler send the right mechanism per request kind (`HX-Redirect`/`HX-Location` for HTMX, 303 otherwise):

```go
// Control-flow signal, not a real error — rides the error-return path.
type Redirect struct{ To string }
func (Redirect) Error() string { return "redirect" }

func (p SubmitForm) ServeHTTP(w http.ResponseWriter, r *http.Request, appCtx *AppContext) error {
    // perform action...
    url, err := structpages.URLFor(r.Context(), DetailPage{}, map[string]any{"id": id})
    if err != nil { return err }
    return Redirect{To: url}
}
```

The `WithErrorHandler` branch that turns `Redirect` into the response is in examples.md §13. The URL comes from `URLFor`, never a string literal (`route-literal` lint).

**A handler method that serves JSON (API endpoint, no error return)**

API endpoints use the **no-error** form so writes go straight to the wire (unbuffered) and the framework's HTML error handler stays out of it. You own the response — including errors, which are JSON like everything else (no `http.Error`; its `text/plain` body is not an API response):

```go
type TrackTime struct{}

func (p TrackTime) ServeHTTP(w http.ResponseWriter, r *http.Request, appCtx *AppContext) {
    var body trackTimeRequest
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        writeJSONError(w, http.StatusBadRequest, "invalid request")
        return
    }
    if err := appCtx.Store.UpdateTime(r.Context(), body); err != nil {
        writeJSONError(w, http.StatusInternalServerError, "update failed")
        return
    }
    w.WriteHeader(http.StatusOK)
}

// One small app-level helper — the API's single error shape:
func writeJSONError(w http.ResponseWriter, status int, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
```

`ServeHTTP` supports four signatures (see reference.md for details). The DI form (extra arg types beyond `w, r`) buffers the response only when the method has a return value.

### Error handling in handlers

The error-returning forms of `ServeHTTP` — and **every** `Props` method — run against a *buffered* response writer. On a non-nil error the buffer is discarded and the error goes to the `WithErrorHandler` callback. So:

- **Never call `http.Error` (or write `w`) in an error-returning handler or in `Props`.** If you write then `return err`, the write is discarded; if you write then `return nil`, you bypass the error handler. Just return the error.
- **For a specific status code, return a typed error** (e.g. `ErrorWithStatus{Status, Title, Message}`) that the global handler unwraps with `errors.As`. Plain errors fall through to a logged 500.
- **API/JSON endpoints use the no-error handler-method form** — direct `w` writes are correct there because you own the status code and skip the buffering wrapper. Write JSON error bodies, not `http.Error`.
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
| string (auto-Ref) | `URLFor(ctx, "Parent.Field", params)` | can't import the page type (cross-package cycle). Equivalent to `Ref("Parent.Field")` — top-level strings only; strings inside `[]any` are still URL fragments. |
| Ref by qualified name | `URLFor(ctx, Ref("Parent.Field"), params)` | explicit form of the string sugar above; pick whichever reads better at the call site |

**Always strict.** A bare type that matches multiple nodes errors instead of silently picking one. The error lists every match and recommends the chain form. There is no opt-out — silent first-match is always wrong, so disambiguating at the call site is mandatory.

**Page groups resolve to their index.** A page group — a page with no render of its own, only child pages — is never served at its bare path: ServeMux matches only its subtree, and the bare path 307-redirects to add the trailing slash. So `URLFor` on a page group returns its index child's URL (the `/{$}` route), carrying the canonical trailing slash: `URLFor(ctx, Section{})` → `/section/`, not `/section`. Leaf pages return their own bare path unchanged. Link a page group by its type and the URL serves a 200 directly, with no redirect hop — don't hand-append a trailing slash, and don't link to the slashless form.

**Chain semantics.** Inside `[]any{...}`, leading typed values form a chain through the page tree: the first resolves to a node via the normal lookup; each subsequent typed value descends into a child of that type (must be unique among siblings, else error). Once a string appears, no more typed values are allowed; remaining strings concat literally to the pattern. The slice is positional — all chain steps first, then all URL fragments; a typed value after a string fragment errors at runtime with the offending position.

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

Page names, route strings, and Ref strings stay stringly typed even in the chain form. `structpages-lint` (below) is the primary guard — it validates them statically in CI. For URLs the linter can't see (built from runtime data, or behind dynamic dispatch), examples.md §14 shows a boot-time `validateURLs(sp)` inventory and an integration test that asserts rendered `href`s.

#### Lint your templates and URL calls

**Rule of thumb: never write an in-app URL as a string literal.** Resolve it by page type — `structpages.URLFor(ctx, SomePage{})` — so the literal can't drift when routes move; the typed call breaks the build instead. When an import cycle blocks naming the page type (a shared chrome package that its own leaf pages import), register a URL resolver from the package that *can* see the types, rather than reaching for a hard-coded route string.

`structpages-lint` enforces this in CI. Install once, then wire in alongside `go test`:

```shell
go install github.com/jackielii/structpages/tools/lint/cmd/structpages-lint@latest
structpages-lint ./...
```

It catches four classes of bug: dangling `URLFor`/`Ref` calls (`urlfor`, `ref`, `params`), unmounted `ID`/`IDTarget` receivers (`id`, `idtarget`), hard-coded URLs in `.templ` URL-bearing attributes (`url-attr`), and `.go` string literals that equal a mounted route (`route-literal`). See reference.md §Lint Tool for the full category table and the `structpages:lint:ignore` suppression syntax (prefer `//`-style directives in both `.go` and `.templ` — HTML comments render into every response).

Templ attribute expressions take `(string, error)` directly — no wrapper needed there. The exception, still inside templ, is a context that needs a plain string, like `templ.Attributes` map values; use a small `must` helper for those (and only those):

```go
func must[T any](v T, err error) T {
    if err != nil { panic(err) }
    return v
}
```

```templ
@PrimaryButton(templ.Attributes{
    "hx-get": must(structpages.URLFor(ctx, UserNewModal{})),
}) { + New User }
```

### 4. HTMX Partial Rendering

This is the framework's central loop. **One method reference — e.g. `MyPage.UserList` — drives three sites that must agree, and `ID`/`IDTarget` make them agree by construction:**

1. **Composition site** — where the page component is composed in, wrap it in an element with `id={ structpages.ID(ctx, MyPage.UserList) }`.
2. **Trigger site** — the element that fires the update points `hx-target={ structpages.IDTarget(ctx, MyPage.UserList) }` at the page's own route (`hx-get={ structpages.URLFor(ctx, MyPage{}) }`).
3. **Server site** — all HTMX requests for a page go to the SAME route; structpages matches the `HX-Target` header back to the page component by id, and the Props method branches on the injected `RenderTarget` with `sel.Is(p.UserList)` to load just that region's data and render it (§5).

Because all three derive from the same method reference, renaming the method or moving the mount can't desynchronize them — there is no string id to drift. Never hand-write the id at one site and generate it at another.

`structpages.ID` / `structpages.IDTarget` generate deterministic element IDs from method references. The id is the page's **full field-name path from the root** joined with the method (`ID` returns `"my-page-user-list"` for a top-level page, `"admin-users-user-list"` when nested; `IDTarget` prepends `#`). Including the ancestor path guarantees two different mounts never collide. If the full id exceeds the length budget (default 40 chars, see `WithMaxIDLength`) it degrades to the compact leaf-only form (`"user-list"`) with a stable hash suffix when the leaf name is shared. Components (standalone `templ` blocks) are prefixed by their package name (`ID(ctx, UserWidget)` → `"<package>-user-widget"`). For plain string arguments both functions return the string unchanged — `IDTarget("body")` is `"body"`, not `"#body"`.

```templ
// Site 1 — composition: set the element ID on the component's wrapper
<div id={ structpages.ID(ctx, MyPage.UserList) }>
    @p.UserList(props.Users)
</div>

// Site 2 — trigger: target that id, hit the page's own route
<input hx-get={ structpages.URLFor(ctx, MyPage{}) }
       hx-target={ structpages.IDTarget(ctx, MyPage.UserList) }
       hx-swap="outerHTML" />
```

```go
// Site 3 — server: Props branches on the injected RenderTarget
func (p MyPage) Props(r *http.Request, sel structpages.RenderTarget) (MyPageProps, error) {
    if sel.Is(p.UserList) {
        return MyPageProps{}, structpages.RenderComponent(p.UserList(loadUsers(r)))
    }
    return MyPageProps{Users: loadUsers(r) /* … everything for the full page */}, nil
}
```

**Self-render uses the current mount.** When `ID` / `IDTarget` runs inside a page's own templ, the id derives from *that mount's* field name — so the same struct type mounted under different parents produces different ids per render context:

```go
type root struct {
    AdminDash dashboardPage `route:"/admin"`
    UserDash  dashboardPage `route:"/user"`
}
// templ (p dashboardPage) Page() { <div id={ structpages.ID(ctx, p.Header) }>... }
// admin render emits id="admin-dash-header"; user render emits id="user-dash-header".
```

**Cross-page references with multiple mounts must be unambiguous.** When `ID` / `IDTarget` is called from outside the page (e.g. an outer story file generating a target selector) and the referenced struct type is mounted multiple times, each mount has its own path-based id — so a bare method expression is ambiguous and the call **errors** with the available mounts listed. (This holds even when the mounts share a field name: their ancestor paths still differ, so the ids differ.) Disambiguate with one of three primitives: the `[]any` chain form, a `Ref`, or a standalone function (see Rule 11).

```go
// IDTarget(ctx, []any{adminRoot{}, dashboardPage{}, "Header"})  // chain + string
// IDTarget(ctx, []any{adminRoot{}, dashboardPage.Header})       // chain + method expr
// IDTarget(ctx, Ref("AdminDash.Header"))                        // by field name
// IDTarget(ctx, EntryOverlaySlot)                               // standalone func: package-prefixed id
```

The chain form mirrors `URLFor`'s shape: leading typed values are chain steps; the trailing element is either a string method name or a method expression whose receiver type IS the leaf. When both the explicit leaf type and the method expression's receiver appear, they must agree.

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

Why this form: `p.UserList(users)` is a normal Go call, so the compiler checks arg types and counts. The alternative — `RenderComponent(MyPage.UserList, users)` or `RenderComponent(sel, users)` — goes through reflection inside the framework, which defers those checks to runtime. Use the reflective forms only when the method's params should be DI-injected by the framework (see §5b).

Note: only methods named `Props` are auto-invoked. `*Props`-suffixed helpers (e.g. `userListData` above; some codebases call them `UserListProps`) are *just regular methods* the user calls from inside `Props` — there's no priority resolution.

Components (standalone `templ` blocks) work the same way — just call the function:

```go
case sel.Is(UserStatsWidget):
    return MyPageProps{}, structpages.RenderComponent(UserStatsWidget(loadStats()))
```

### 5b. RenderComponent by method expression (DI-injected params)

Page structs are stateless, so even a *different* page's component is normally constructed directly with a zero-value receiver — `RenderComponent(MyPage{}.ItemList(items))` — and that stays the preferred form. The reflective method-expression form is for components whose parameters the framework should DI-inject rather than you supplying them:

```go
// ItemList takes DI-injectable params (e.g. *http.Request, *AppContext) —
// the framework finds the mounted page, fills them, and invokes the method:
func (p MyDelete) ServeHTTP(w http.ResponseWriter, r *http.Request, appCtx *AppContext) error {
    if err := store.Delete(...); err != nil { return err }
    return structpages.RenderComponent(MyPage.ItemList)
}
```

Explicit args are matched into the non-injected parameters (`RenderComponent(MyPage.ItemList, items)`), validated by reflection before the call — readable errors, but at runtime, not compile time. If you're loading the data yourself anyway, construct the component instead.

### 5c. Nested swap levels (Page → Content → Detail)

A page's page components can be composed into **nested swap levels**, each an independent HTMX target. The outer level wraps the next in its templ; the levels are *not* a tree the matcher walks — they're sibling page components on one page, each with its own id (§4). Because `HX-Target` selects the page component whose id it matches exactly, an `HX-Target` of a given level's id re-renders *only* that level, even though `Page` composes `Content` composes `Detail`. Compose one level per region you need to swap on its own:

- **`Page`** (the Page method) — the full document. Rendered on a cold load / `hx-boost` body swap. Composes the app layout around `Content`.
- **`Content`** — the page's main region (a naming convention, not a framework concept). Holds the page chrome — heading, back-link, toolbar — around the inner level. Rendered when only the main content swaps (boosted nav between pages).
- **`Detail`** (or another inner name) — a region *inside* `Content` that must swap on its own, independently of the chrome. Holds **none** of the page chrome.

```templ
templ (d FooDetail) Page(p Props)    { @ui.Layout(title) { <main class="…">@d.Content(p)</main> } }
templ (d FooDetail) Content(p Props) { <div id={ structpages.ID(ctx, FooDetail.Content) }>
                                          <a href={ back }>&larr; Foos</a>   // standalone-page chrome
                                          @d.Detail(p)
                                        </div> }
templ (FooDetail) Detail(p Props)    { <div id={ structpages.ID(ctx, FooDetail.Detail) } class="@container …">
                                          … fields, lifecycle actions, dialog mount …   // NO back-link, NO header
                                        </div> }
```

**Why three levels, not two.** The trap is reusing `Content` as the swap fragment for an embedded region — e.g. a master-detail inspector pane hosting the *standalone detail page's* `Content`. That drags the page chrome (back-link, page header, outer container) into the pane, where it's wrong. Splitting out `Detail` gives the embedded region a chrome-less partial while `Content` keeps the standalone-page chrome. **The level you embed/swap is the one with no chrome of its own.**

**Master-detail rule of thumb.** The list page renders a detail *mount* whose id is `ID(ctx, FooDetail.Detail)`; rows `hx-get` the detail route with `hx-target = IDTarget(ctx, FooDetail.Detail)`. Lifecycle actions and dialog handlers that re-render the detail also target — and `RenderComponent` — `FooDetail.Detail`, never `.Content`. The standalone detail page (deep-link / no-JS) is the only thing that renders `Content` (chrome + `Detail`).

Add a fourth level whenever a sub-region needs to swap independently again — the rule generalizes: **one page component per independently-swappable region, outer wraps inner, embed/target the innermost that has no chrome above it.**

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

### 8. Testing renders with a bare context

Unit tests that render templ components directly — without spinning up an HTTP server — need a page tree in `context.Background()` so calls to `URLFor` / `ID` / `IDTarget` resolve. Use `structpages.Parse` (builds the tree, no mux) and `sp.PageContext(ctx)` (wraps ctx with the tree):

```go
sp, err := structpages.Parse(webPages{}, "/", "App",
    structpages.WithArgs(fakeAppCtx),
)
if err != nil { t.Fatal(err) }
ctx := sp.PageContext(context.Background())

// Now URLFor in templ components and props helpers resolves against webPages{}.
buf := &bytes.Buffer{}
if err := MyPage{}.Page(props).Render(ctx, buf); err != nil { t.Fatal(err) }
```

This is the recommended fix for two patterns that fail under bare-context renders:

1. **Component-level renders with handcrafted props.** Props is hand-rolled; the only framework dep is `URLFor` in the templ. `sp.PageContext` is a one-line wrap.
2. **Cross-module URL refs (e.g. appshell linking to `Home` in a sibling module).** Mount only one sub-tree on a test mux and `URLFor` to siblings outside it fails. Building the canonical root via `Parse(webPages{}, ...)` gives the tree those refs need, without registering any routes.

`Parse` accepts the same options as `Mount` — `WithArgs` for DI args used by `Props`, `WithURLPrefix` if the test asserts prefixed URLs, etc. Mux-shaped options (middlewares) are accepted but inert since no handlers register.

## Key Rules

1. **Props methods extract path params** via `r.PathValue("param")`, not function arguments.
2. **Never hardcode URLs** — always use `structpages.URLFor`.
3. **Partials take ONLY their specific data**, not the full props struct.
4. **`RenderComponent` is returned as an error** — when returned, the Props return values (other than the error) are ignored.
5. **Prefer `RenderComponent(p.X(args))` / `RenderComponent(MyPage{}.X(args))`** — constructed components are compile-time-checked; zero-value receivers make this work cross-page too. Reserve the reflective method-expression form for components whose params the framework should DI-inject (§5b).
6. **Children are registered before parents** on the mux (so nested-route conflicts resolve correctly).
7. **Promoted (embedded) methods are skipped** — only methods defined directly on the struct count.
8. **URL params auto-fill from current request's route only** — sibling routes with different param names do not auto-fill.
9. **`ErrSkipPageRender` is only honored from `Props`** (e.g. after writing a redirect). Returning it from `ServeHTTP` does nothing special.
10. **Disambiguation primitives:** type mounted under multiple parents → the `[]any{ParentPage{}, LeafPage{}}` chain form (strict `URLFor` errors on bare lookups). Can't import the page type (cycle) → string page arg / `Ref("Parent.Field")`; validate Ref strings at boot (§3) and with `structpages-lint`.
11. **Plain strings pass through `ID` and `IDTarget` unchanged** — `IDTarget("body")` is `"body"`, not `"#body"` (asymmetric to `URLFor` on purpose: literal CSS selectors are legitimate, literal URL paths are anti-pattern). For an id independent of mount position, define the slot as a component (standalone function) — `IDTarget(ctx, EntryOverlaySlot)` → `"#<package>-entry-overlay-slot"`, package-prefixed, no mount-path dependence. Preferred shape for cross-package slot targeting (§4).
12. **The `form:` struct tag is not read by the framework** — only `route:` is. Anything else on a route field is ignored.
13. **Never write `w` (e.g. `http.Error`) in `Props` or an error-returning `ServeHTTP`** — they are buffered; return the error instead. Use a typed error like `ErrorWithStatus` for a specific status code. API/JSON endpoints use the no-error `ServeHTTP(w, r, deps...)` form, where direct writes are correct — JSON error bodies there, never `http.Error` (see examples.md §13).
14. **Never hand-write a partial's element id** — derive all three sites (composition `id={ID(…)}`, trigger `hx-target={IDTarget(…)}`, server `sel.Is(…)`) from the same method reference (§4).
