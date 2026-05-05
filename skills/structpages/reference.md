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

## StructPages Type

Returned by `Mount()`. Methods:

```go
func (sp *StructPages) URLFor(page any, args ...any) (string, error)
func (sp *StructPages) ID(v any) (string, error)
func (sp *StructPages) IDTarget(v any) (string, error)
```

Use these outside of request context (e.g., during initialization). Within request handlers, use the context-based versions.

## Context Functions

```go
func URLFor(ctx context.Context, page any, args ...any) (string, error)
func ID(ctx context.Context, v any) (string, error)
func IDTarget(ctx context.Context, v any) (string, error)
```

### URLFor Page Argument Types

1. **Struct instance**: `URLFor(ctx, MyPage{})` — matches by type
2. **Ref string**: `URLFor(ctx, Ref("pageName"))` — by name, or `Ref("/route")` by route
3. **Predicate**: `URLFor(ctx, func(*PageNode) bool { ... })`
4. **`[]any` slice**: `URLFor(ctx, []any{MyPage{}, "?page={page}"}, "page", 5)` — concatenates a route and a query-string template

### URLFor Args Formats

In order of detection inside `formatPathSegments`:

- **Positional**: arg count exactly matches placeholder count → `URLFor(ctx, page{}, "val1", "val2")` fills left to right.
- **Key-value pairs**: even arg count, every even-indexed arg is a string, AND at least one of those strings matches a placeholder name → `URLFor(ctx, page{}, "id", 123, "slug", "hello")`.
- **Map**: `URLFor(ctx, page{}, map[string]any{"id": 123})`.
- **Auto-fill from request**: unfilled placeholders that match path params from the *current request's route* are filled automatically. Other routes' params do not auto-fill.

### ID / IDTarget Input Types

- **Unbound method**: `ID(ctx, MyPage.UserList)` → `"my-page-user-list"`
- **Bound method**: `ID(ctx, p.UserList)` → same result
- **Standalone function**: `ID(ctx, UserWidget)` → `"user-widget"` (no page prefix)
- **Ref string**: `ID(ctx, Ref("MyPage.UserList"))` qualified, or `Ref("UserList")` if unambiguous across all pages
- **Plain string**: `ID(ctx, "my-custom-id")` → returned as-is

`IDTarget` works the same way but prepends `#` to method-derived IDs: `"#my-page-user-list"`. **For plain string inputs, `IDTarget` returns the string verbatim** — `IDTarget("body")` is `"body"`, not `"#body"`. Pass `"#body"` if you want the hash.

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
    // handle error
})
```

Called when Props or error-returning ServeHTTP fails.

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

Patterns:

1. **Direct component**: `RenderComponent(myTemplComponent)` — render a pre-built templ component (no args allowed).
2. **componentGetter**: `RenderComponent(customTarget)` where `customTarget` implements `Component() component` (no args).
3. **Same-page via target**: `RenderComponent(target, args...)` after `target.Is()` matched (required for function targets).
4. **Method expression** (cross-page or same-page): `RenderComponent(MyPage.ItemList, items)` — framework finds the page that owns the method, looks up the component, and calls it with `items`.
5. **Bound method value**: `RenderComponent(p.EditSection, props)` — same as #4 with the receiver already bound.

When returned from `Props`, the other return values are ignored.

Argument-count and assignability are validated *before* the call (in `executeRenderOp`) — mismatches surface as readable errors instead of panics.

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
- URLFor: `Ref("pageName")` by name, `Ref("/path")` by route
- ID: `Ref("PageName.MethodName")` qualified, `Ref("MethodName")` if unambiguous

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

## Buffered Response

Error-returning `ServeHTTP` uses buffered writer. On error, buffer is discarded and error handler renders instead. Supports `Flush()` for streaming/SSE.
