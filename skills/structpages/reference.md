# structpages API Reference

## Core Function: Mount

```go
func Mount(mux Mux, page any, route, title string, options ...Option) (*StructPages, error)
```

Main entry point. Parses page tree, registers routes on mux, returns `*StructPages` for URL/ID generation.

- `mux`: `*http.ServeMux` (or nil for default)
- `page`: Root struct with route tags
- `route`: Base path (usually `"/"`)
- `title`: Root page title
- `options`: `WithArgs`, `WithErrorHandler`, `WithMiddlewares`, `WithTargetSelector`, `WithWarnEmptyRoute`

## Parse (no-mux variant)

```go
func Parse(page any, route, title string, options ...Option) (*StructPages, error)
```

Builds the page tree without registering any routes on a mux. Use in tests and tooling that need URLFor/ID/IDTarget against the real page tree but don't want an HTTP server. Accepts the same options as `Mount`; mux-shaped options (middlewares) are inert.

## StructPages Type

Returned by `Mount()` and `Parse()`. Methods:

```go
func (sp *StructPages) URLFor(page any, args ...any) (string, error)
func (sp *StructPages) ID(v any) (string, error)
func (sp *StructPages) IDTarget(v any) (string, error)
func (sp *StructPages) PageContext(ctx context.Context) context.Context
```

`PageContext` wraps `ctx` with `sp`'s page tree so the context-form `URLFor` / `ID` / `IDTarget` (in templ renders, props helpers) resolve against `sp`. The recommended test pattern: `Parse` once per package, wrap a bare `context.Background()` in `PageContext`, render against the wrapped ctx.

Use the method form of `URLFor`/`ID`/`IDTarget` outside of request context (e.g., during initialization). Within request handlers, use the context-based versions.

## Context Functions

```go
func URLFor(ctx context.Context, page any, args ...any) (string, error)
func ID(ctx context.Context, v any) (string, error)
func IDTarget(ctx context.Context, v any) (string, error)
```

### URLFor Page Argument Types

Recommended call shape: `URLFor(ctx, page, params)` where `params` is a `map[string]any`.

1. **Struct instance**: `URLFor(ctx, MyPage{}, params)` — matches by type. Strict: errors if the type matches multiple nodes (use the chain or Ref form below).
2. **`[]any` chain / composition**: `URLFor(ctx, []any{ParentPage{}, LeafPage{}}, params)` — typed values form a chain (descend by child type). Trailing strings concat as literal URL fragments: `[]any{Page{}, "?q={q}"}`. Mixing typed values after a string fragment is rejected.
3. **Ref string**: `URLFor(ctx, Ref("Parent.Field"), params)` — qualified path (walks down by `PageNode.Name`). `Ref("PageName")` matches the first node with that name. Use Ref when the typed page can't be imported (cross-package cycle) or for type aliases.
4. **Predicate**: `URLFor(ctx, func(*PageNode) bool { ... })` — escape hatch for custom matching.

### URLFor Args Formats

**Recommended: `map[string]any`** — explicit, position-independent, refactor-safe.

```go
URLFor(ctx, page{}, map[string]any{"id": 123, "slug": "hello"})
```

The other forms are also supported. Order of detection inside `formatPathSegments`:

- **Map**: a single `map[string]any` first arg. Recommended; values are looked up by placeholder name.
- **Positional**: arg count exactly matches placeholder count → `URLFor(ctx, page{}, "val1", "val2")` fills left to right. Brittle if placeholders are reordered.
- **Key-value pairs**: even arg count, every even-indexed arg is a string, AND at least one of those strings matches a placeholder name → `URLFor(ctx, page{}, "id", 123, "slug", "hello")`. Equivalent to map form but spread across positional args; harder to scan.
- **Auto-fill from request**: unfilled placeholders that match path params from the *current request's route* are filled automatically. Other routes' params do not auto-fill.

### ID / IDTarget Input Types

- **Unbound method**: `ID(ctx, MyPage.UserList)` → `"my-page-user-list"`
- **Bound method**: `ID(ctx, p.UserList)` → same result
- **Standalone function**: `ID(ctx, UserWidget)` → `"user-widget"` (no page prefix, type-stable across all mounts)
- **`[]any` chain form**: leading typed values + trailing method spec; the trailing element is either a string method name or a method expression
  - `IDTarget(ctx, []any{adminRoot{}, dashboardPage{}, "Header"})` — chain + string
  - `IDTarget(ctx, []any{adminRoot{}, dashboardPage.Header})` — chain + method expression (receiver type collapses with the leaf if both appear, and must agree)
- **Ref string**: `ID(ctx, Ref("MyPage.UserList"))` qualified, or `Ref("UserList")` if unambiguous across all pages
- **Plain string**: `ID(ctx, "my-custom-id")` → returned as-is

`IDTarget` works the same way but prepends `#` to method-derived IDs: `"#my-page-user-list"`. **For plain string inputs, `IDTarget` returns the string verbatim** — `IDTarget("body")` is `"body"`, not `"#body"`. Pass `"#body"` if you want the hash.

#### Mount-context semantics

The id is derived from the matched PageNode's *field name* (mount role), not the struct type name. When the same struct is mounted under multiple field names with different kebabs:

- **Self-render** (inside the page's own templ): the resolver consults the current request's page node from context, so the id reflects *that mount* — admin's render emits `"admin-dash-header"`, user's emits `"user-dash-header"`.
- **Cross-page** (call site has no current-page context): the resolver collects every mount of the receiver type. If all share the same field name (entryPage-style: three mounts all named `EntryDetail`), the resulting kebab id is identical → return it. If field names differ → error with the available mounts and three disambiguation primitives:
  1. `[]any` chain form (type-safe — chain steps are real types)
  2. `Ref("Parent.Field")` (string lookup, lint-validated, useful when the type isn't importable)
  3. Move the slot to a standalone function (type-stable id, no mount dependency)

Naming: CamelCase → kebab-case (`HTMLParser` → `html-parser`). The reverse direction (`kebabToPascal`) is lossy: `html-parser` → `HtmlParser`, not `HTMLParser`. HX-Target matching uses suffix and page-prefix rules (see `HTMXRenderTarget` below) rather than simple reverse conversion, so this lossiness rarely matters in practice.

## Options

### WithArgs

```go
structpages.WithArgs(db, logger, appCtx)
```

Type-based DI. Each type registered once. Injected into `Props`, `ServeHTTP`, `Middlewares`, `Init` methods by type matching.

The matcher (in `args.go`) coerces between pointer and value forms and falls back to assignability. So a single `*AppContext` registration also satisfies parameters of any interface `*AppContext` implements. Generic types and interface types both work — see `generics_injection_test.go` for the tested matrix (basic injection, duplicate-type errors, slices/maps, type aliases, function types, nil handling, complex constraints, pointer semantics, method matching, interface injection).

To register two values of the same underlying type, define named types to disambiguate.

### WithErrorHandler

```go
structpages.WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
    status := http.StatusInternalServerError
    var se ErrorWithStatus
    if errors.As(err, &se) {
        status = se.Status
    }
    // render an HTML/HTMX error page with `status`
})
```

Called when `Props` or an error-returning `ServeHTTP` returns a non-nil error (the response buffer is discarded first). This is the single place that turns errors into responses, so handlers should *return* errors rather than writing `w` themselves. Define a typed error carrying a status code and unwrap it here with `errors.As`; plain errors default to a logged 500. The writer passed in is still the buffered one — writing to it here is fine because no handler runs after. See examples.md §13.

### WithMiddlewares

```go
structpages.WithMiddlewares(loggingMW, authMW)
```

Global middleware applied to all routes. Executed in order (first = outermost).

### WithTargetSelector

```go
structpages.WithTargetSelector(func(r *http.Request, pn *PageNode) (RenderTarget, error) {
    return structpages.HTMXRenderTarget(r, pn) // default
})
```

Custom component selection logic. Default is `HTMXRenderTarget`.

### WithWarnEmptyRoute

```go
structpages.WithWarnEmptyRoute(func(pn *PageNode) { /* custom warning */ })
```

Customize/suppress warnings for pages with no handler and no children.

## Page Methods

### Props

```go
func (p MyPage) Props(r *http.Request, [w http.ResponseWriter,] [sel RenderTarget,] [deps ...any]) (PropsType, error)
```

Parameters matched by type (order doesn't matter):
- `*http.Request` — the request
- `http.ResponseWriter` — optional, for setting headers/cookies
- `RenderTarget` — which component will render (always non-nil; constructed by `targetSelector`)
- `*PageNode` — route metadata for the current node
- Any registered dependency

**Only the method literally named `Props` is auto-invoked.** Methods whose names *end* in `Props` (e.g. `UserListProps`, `PageProps`, `ContentProps`) are stored in `PageNode.Props` map but never auto-resolved by the framework — they are ordinary helper methods you can call yourself from inside `Props`. The earlier docs that suggested `PageProps`/`ContentProps` take priority were incorrect.

### Page / Content / Custom Components

Templ methods returning a component. `Page` is for full page, `Content` for body, custom names for partials.

```go
templ (p MyPage) Page(props MyProps) { ... }
templ (p MyPage) Content(props MyProps) { ... }
templ (p MyPage) UserList(users []User) { ... }  // partial with specific data type
```

### ServeHTTP

Four supported signatures:

1. `ServeHTTP(w, r)` — standard `http.Handler`. Direct write to `w`.
2. `ServeHTTP(w, r) error` — buffered. On non-nil error, the buffer is discarded and the error handler runs.
3. `ServeHTTP(w, r, deps...)` — DI form, no return value. Direct write to `w`.
4. `ServeHTTP(w, r, deps...) error` — DI form, buffered (because of the return value).

In the DI forms, `RenderTarget` is also injectable: the framework computes one via the configured `targetSelector` and adds it to the available args. This lets `ServeHTTP` decide which partial to render, e.g.:

```go
func (p IndexPage) ServeHTTP(w http.ResponseWriter, r *http.Request, target structpages.RenderTarget) error {
    if target.Is(IndexPage.Table) { /* … */ }
}
```

`ServeHTTP` takes precedence over the Props/Component flow — if defined, `Props` and component methods are not consulted.

**Choosing a form, and the `http.Error` anti-pattern.** The buffered (error-returning) forms exist so that on error the framework can discard a partial response and render through `WithErrorHandler` instead. Therefore:

- In signatures 2 and 4 (and in any `Props` method) **never write `w` directly** — no `http.Error`, no `w.WriteHeader`. Writing then `return err` discards the write when the buffer resets; writing then `return nil` bypasses the error handler. Return the error and let `WithErrorHandler` render it. For a specific status code, return a typed error (e.g. `ErrorWithStatus{Status, Title, Message}`) that the handler unwraps via `errors.As`.
- For endpoints that serve JSON / non-HTML / streamed responses, use signature **3** (`ServeHTTP(w, r, deps...)`, no return). It is unbuffered, so writes go straight to the client and the HTML error handler is never invoked. There `http.Error` and direct `w` writes are the correct tools — you own the status code.

See examples.md §13 for the full worked pattern.

### Middlewares

```go
func (p MyPages) Middlewares([deps ...any]) []MiddlewareFunc
```

Page-specific middleware. Also applies to all descendants.

### Init

```go
func (p MyPage) Init([deps ...any]) error    // value receiver works
func (p *MyPage) Init([deps ...any]) error   // pointer receiver also works (use this if Init mutates)
```

Called at Mount time for one-time setup. Either receiver kind is allowed; the framework iterates both struct and pointer types in `processMethods` and `prepareReceiver` adjusts addressability as needed. Pointer receiver is the usual choice since `Init` typically wants to store state on the page.

Return `error` to abort `Mount`.

## RenderTarget Interface

```go
type RenderTarget interface {
    Is(method any) bool
}
```

`Is()` checks if target matches a component. Works with:
- Page methods: `sel.Is(MyPage.UserList)` or `sel.Is(p.UserList)`
- Standalone functions: `sel.Is(UserWidget)`

**For function targets** (`functionRenderTarget`): `Is()` has the side effect of storing the matched function value. You **must** call `Is(fn)` before `RenderComponent(target, args...)` — otherwise `RenderComponent` returns "function target has no funcValue".

**For method targets** (`methodRenderTarget`): the method is captured at construction time, so `Is()` is recommended for the readable switch pattern but not strictly required for `RenderComponent` to work.

### componentGetter (extension point for custom RenderTargets)

If a custom `TargetSelector` returns a `RenderTarget` that *also* implements:

```go
interface { Component() component }
```

then `RenderComponent(target)` (with no args) calls `Component()` directly to get the component to render. Useful for selectors that already know the data and want to bypass the args/method pipeline.

## RenderComponent

```go
func RenderComponent(targetOrMethod any, args ...any) error
```

Returns a sentinel-typed error (`*errRenderComponent`) that instructs the framework to render a specific component. Detected in both the Props error path and the buffered-`ServeHTTP` error path.

Patterns, split by whether they go through reflection:

**Direct (no reflection — preferred when applicable)**

1. **Pre-built component**: `RenderComponent(myTemplComponent)` — render a templ component already constructed by the caller. No args allowed. Use this whenever you have (or can construct) the component value yourself; it's compile-time-checked end-to-end. The same-page idiom is `RenderComponent(p.X(args))` and the standalone-function idiom is `RenderComponent(MyWidget(args))`.
2. **componentGetter**: `RenderComponent(customTarget)` where `customTarget` implements `Component() component`. No args. Calls `Component()` to get the component to render.

**Reflective dispatch (framework looks up the method and applies DI)**

3. **Method expression** (cross-page or same-page): `RenderComponent(MyPage.ItemList, items)` — framework finds the page that owns the method, looks up the component, calls it with `items`, filling any DI-injected parameters. Necessary when the caller doesn't have the target page's receiver in scope (typical `ServeHTTP` handlers re-rendering a sibling page's partial).
4. **Bound method value**: `RenderComponent(p.EditSection, props)` — same as #3 with the receiver already bound. Equivalent to direct form #1 (`RenderComponent(p.EditSection(props))`), but goes through reflection; prefer the direct form when `p` is in scope.
5. **Via target**: `RenderComponent(target, args...)` after `target.Is()` matched (required for function targets — `Is()` stores the function pointer). Works for method targets too, but if the receiver is in scope, `RenderComponent(p.X(args))` is clearer and faster.

When returned from `Props`, the other return values are ignored.

Argument-count and assignability are validated *before* the call (in `executeRenderOp`) — mismatches surface as readable errors instead of panics. Direct-form callers get the same checks from the Go compiler at build time.

## HTMXRenderTarget (Default TargetSelector)

1. Checks `HX-Request: true` header. If absent, returns `methodRenderTarget` for `Page`.
2. Reads `HX-Target` header. If empty, returns `methodRenderTarget` for `Page`.
3. Tries `matchComponentByTarget` (in `htmx.go`):
   - **First pass — exact matches**: `pagePrefix-componentID` first, then bare `componentID`.
   - **Second pass — suffix matches**, longest-wins, with three rules:
     - `fullID` ends with `target`
     - `target` ends with `fullID`
     - `target` ends with `componentID` *only if* `target` also starts with `pagePrefix-` (this guard prevents `home-content` from matching component `Content` on page `IndexPage`)
4. If a method matches, returns `methodRenderTarget` for that method.
5. If no method matches, returns `functionRenderTarget` carrying the raw `HX-Target` for lazy evaluation in `Is()` against standalone function components.

Non-HTMX requests always get `methodRenderTarget` for `Page` — if `Page` is not defined (Props-only page), `methodRenderTarget.Is()` returns false and `Props` must call `RenderComponent` itself.

## ErrSkipPageRender

```go
var ErrSkipPageRender = errors.New("skip page render")
```

Return from `Props` to skip rendering (after writing a redirect, etc.). **This is checked only in the Props error path** (`struct_pages.go:325`). Returning `ErrSkipPageRender` from `ServeHTTP` does not have the same effect — it falls through to the error handler.

## PageNode

```go
type PageNode struct {
    Name, Title, Method, Route string
    Value       reflect.Value
    Props       map[string]reflect.Method
    Components  map[string]reflect.Method
    Middlewares  *reflect.Method
    Parent      *PageNode
    Children    []*PageNode
}
```

- `FullRoute() string` - complete route including parents
- `All() iter.Seq[*PageNode]` - iterate this node and all descendants

## MiddlewareFunc

```go
type MiddlewareFunc func(http.Handler, *PageNode) http.Handler
```

Unlike standard Go middleware, receives `*PageNode` for route metadata access.

## Ref Type

```go
type Ref string
```

Dynamic references for URLFor/ID/IDTarget:
- URLFor: `Ref("PageName")` by name, `Ref("Parent.Field")` qualified path. **URLFor also accepts a plain string at the top level as sugar** — `URLFor(ctx, "Parent.Field")` is equivalent to `URLFor(ctx, Ref("Parent.Field"))`. Strings inside `[]any{...}` composition are still URL fragments.
- ID: `Ref("PageName.MethodName")` qualified, `Ref("MethodName")` if unambiguous. ID/IDTarget do **not** accept the string-as-Ref sugar — plain strings to ID/IDTarget are returned as literal IDs/selectors (e.g. `IDTarget(ctx, "body")` returns `"body"`). Use `Ref(...)` explicitly when you mean dynamic lookup.

## Mux Interface

```go
type Mux interface {
    Handle(pattern string, handler http.Handler)
}
```

Satisfied by `*http.ServeMux`.

## Route Tag Format

```
route:"[METHOD] /path [Title]"
```

- Method: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS, TRACE, ALL (or omit for all)
- Path: Go 1.22+ mux patterns. `{param}` for path params, `{param...}` for wildcards, `{$}` for exact match
- Title: remaining text after path

**Prefix subtrees use `{path...}`, not trailing slashes.** `FullRoute()` uses `path.Join`, which strips trailing slashes when concatenating with parent paths — so `route:"/static/"` under `/admin` registers as `GET /admin/static` (exact match), not `GET /admin/static/` (prefix match). Use `route:"/static/{path...}"` and read `r.PathValue("path")` to capture the subpath. See SKILL.md "Mounting a module's static-asset subtree" and examples.md §12.

## Buffered Response

Error-returning `ServeHTTP` (and every `Props` method) uses a buffered writer. On a non-nil error the buffer is discarded and `WithErrorHandler` renders instead — so do **not** write `w` directly in these forms; return the error (typed, e.g. `ErrorWithStatus`, when a specific status is needed). The no-return `ServeHTTP(w, r, deps...)` form skips the structpages buffering wrapper — use it for one-shot JSON/API endpoints where you write directly and own the status code.

For streaming (SSE, progress), flush with `http.NewResponseController(w)`: the buffered wrapper implements `FlushError()` and `Unwrap()`, so the controller drains the buffer to the client and reaches any underlying flusher. This works from *either* `ServeHTTP` form — and unlike grabbing the raw `w`, it is the only way to *guarantee* an unbuffered write through whatever middleware also wraps the writer. See examples.md §13.

## Lint Tool

`structpages-lint` is a static analyzer for `structpages` projects.

```shell
go install github.com/jackielii/structpages/tools/lint/cmd/structpages-lint@latest
structpages-lint ./...
```

Diagnostic categories:

| Category | What it flags |
|---|---|
| `urlfor` | `structpages.URLFor` chain/composition errors (unknown child type, fragment-before-step). |
| `ref` | `structpages.Ref(...)` strings that don't resolve to a page tree node. |
| `id`, `idtarget` | `structpages.ID` / `IDTarget` method expressions whose receiver is not mounted. |
| `params` | `URLFor` params that don't appear in the route pattern. |
| `url-attr` | URL-bearing HTML attributes in `.templ` files (`href`, `action`, `formaction`, `hx-{get,post,put,patch,delete}`, `hx-{push,replace}-url`) whose values are hard-coded internal paths, string concats, or `fmt.Sprint*` calls. |

Suppression syntax (place above the call/element, or on the same line):

| Source | Preferred | Also supported |
|---|---|---|
| `.go` files | `//structpages:lint:ignore <category>[,…]` | — |
| `.templ` files | `// structpages:lint:ignore <category>[,…]` (Go-style; stripped from HTML output) | `<!-- structpages:lint:ignore <category>[,…] -->` (renders into HTML, prefer only when intentional) |

A bare directive with no category suppresses every category on the targeted line. Categories are comma-separated.
