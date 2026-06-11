---
title: API Guide
slug: /api
sidebar_position: 12
---

# API Guide

Hand-written guide to the public API. For generated godoc, see the [package reference](./reference/package.md).

## Mount

```go
func Mount(mux Mux, page any, route, title string, options ...Option) (*StructPages, error)
```

Main entry point. Parses the page tree, registers routes on the mux, and returns a `*StructPages` for URL/id generation.

- `mux`: anything implementing `Handle(pattern string, handler http.Handler)` — e.g. `*http.ServeMux`. Pass `nil` for `http.DefaultServeMux`.
- `page`: root struct with route tags
- `route`: base path (usually `"/"`)
- `title`: root page title
- `options`: see [Options](#options)

```go
mux := http.NewServeMux()
sp, err := structpages.Mount(mux, &pages{}, "/", "App")
if err != nil {
    log.Fatal(err)
}
if err := http.ListenAndServe(":8080", mux); err != nil {
    log.Fatal(err)
}
```

## Parse (no-mux variant)

```go
func Parse(page any, route, title string, options ...Option) (*StructPages, error)
```

Builds the page tree without registering routes. Use in tests and tooling that need `URLFor`/`ID`/`IDTarget` against the real page tree but don't want an HTTP server. Accepts the same options as `Mount`; mux-shaped options (middlewares) are inert.

## StructPages methods

```go
func (sp *StructPages) URLFor(page any, args ...any) (string, error)
func (sp *StructPages) ID(v any) (string, error)
func (sp *StructPages) IDTarget(v any) (string, error)
func (sp *StructPages) PageContext(ctx context.Context) context.Context
```

Use the method forms outside request context (initialization, boot-time validation, tests). Within request handlers and templ renders, use the context-based package functions — the framework injects the parse context via internal middleware.

`PageContext` wraps a bare context with `sp`'s page tree so the context-form functions resolve against it. The recommended test pattern: `Parse` once per package, wrap `context.Background()` in `PageContext`, render against the wrapped ctx (see [Templ Patterns](./templ.md#testing-renders-with-a-bare-context)).

## Context functions

```go
func URLFor(ctx context.Context, page any, args ...any) (string, error)
func ID(ctx context.Context, v any) (string, error)
func IDTarget(ctx context.Context, v any) (string, error)
```

Page-argument forms, params formats, strict-mode semantics, and chain composition are covered in [URLFor & ID](./urlfor.md). Id-generation semantics (full field-path ids, multi-mount behavior, length budget) are covered in [HTMX Integration](./htmx.md#how-ids-are-generated).

## Options

### WithArgs

```go
structpages.WithArgs(store, sessionManager, logger)
```

Register dependency-injection values, matched by type into `Props` / `ServeHTTP` / `Middlewares` / `Init` parameters. Each type registers once; see [Advanced](./advanced.md#dependency-injection) for coercion rules and named-type disambiguation.

### WithErrorHandler

```go
structpages.WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) { ... })
```

The single callback that owns every error response from buffered handlers and Props. See [Error Handling](./error-handling.md#the-global-handler) for the full pattern — typed statuses, the `Redirect` signal, cancellation, logged-500 fallback.

### WithMiddlewares

```go
structpages.WithMiddlewares(loggingMiddleware, authMiddleware)
```

Global middleware applied to all routes. `MiddlewareFunc` is `func(next http.Handler, pn *PageNode) http.Handler`. See [Middleware](./middleware.md) for execution order.

### WithTargetSelector

```go
structpages.WithTargetSelector(structpages.HTMXv4RenderTarget)
```

Replace the default `HTMXRenderTarget` — e.g. with the htmx 4 variant, or a custom selector for content negotiation. See [HTMX Integration](./htmx.md#custom-target-selectors).

### WithMaxIDLength

```go
structpages.WithMaxIDLength(60) // default 40
```

Character budget for generated element ids before they degrade from the readable full-path form (`admin-users-user-list`) to the compact leaf-only form (`user-list`, plus a stable hash suffix when the leaf name is not unique). Affects id generation only, never routing.

### WithURLPrefix

```go
structpages.WithURLPrefix("/app")
```

Prefix prepended to all generated URLs — for apps served under a sub-path.

### WithWarnEmptyRoute

```go
structpages.WithWarnEmptyRoute(func(pn *structpages.PageNode) {
    log.Printf("skipping empty page: %s", pn.Name)
})
```

Customize or suppress (`func(*PageNode) {}`) warnings for pages with no handler and no children.

## Page methods

Pages can implement these optional methods. Parameters on `Props`, `ServeHTTP`, `Middlewares`, and `Init` are matched by **type**, in any order; injectable types are `*http.Request`, `http.ResponseWriter`, `structpages.RenderTarget`, `*structpages.PageNode`, and anything registered via `WithArgs`.

### Page

```go
templ (p myPage) Page(props MyProps) { ... }
```

The main render entry — a page component composing the full page. Pages without a `Page` method can still render by returning `RenderComponent(...)` from Props.

### Props

```go
func (p myPage) Props(r *http.Request, target structpages.RenderTarget, store *Store) (MyProps, error)
```

Loads data before render; the returned props struct is passed to the selected page component. Only the method literally named `Props` is auto-invoked. Runs against a buffered writer — return errors, never write `w` (see [Error Handling](./error-handling.md)).

### ServeHTTP

Four signatures:

```go
func (p T) ServeHTTP(w http.ResponseWriter, r *http.Request)                      // standard http.Handler, unbuffered
func (p T) ServeHTTP(w http.ResponseWriter, r *http.Request) error                // buffered; error → WithErrorHandler
func (p T) ServeHTTP(w http.ResponseWriter, r *http.Request, store *Store)        // DI, no return, unbuffered
func (p T) ServeHTTP(w http.ResponseWriter, r *http.Request, store *Store) error  // DI, buffered
```

In the DI forms, `RenderTarget` is also injectable, so a handler method can branch on `target.Is(...)` before responding.

### Middlewares

```go
func (p T) Middlewares(deps ...) []structpages.MiddlewareFunc
```

Page-specific middleware, also applied to all descendants.

### Init

```go
func (p *T) Init(deps ...) error
```

One-time setup at `Mount`; errors abort the mount. See [Advanced](./advanced.md#initialization).

## RenderTarget

```go
type RenderTarget interface {
    Is(method any) bool
}
```

Represents the page component selected for this request. Injected into Props (and DI-form `ServeHTTP`). `Is` accepts page component references (`target.Is(p.UserList)`) and standalone component functions (`target.Is(UserStatsWidget)`).

## RenderComponent

```go
func RenderComponent(targetOrMethod any, args ...any) error
```

Returns a sentinel error instructing the framework to render a specific component. Honored in the Props error path and the buffered-`ServeHTTP` error path; when returned from Props, the other return values are ignored.

**Direct (preferred — compile-time-checked):**

```go
return MyProps{}, structpages.RenderComponent(p.UserList(users))      // same page
return structpages.RenderComponent(index{}.TodoList(todos))           // another page — zero-value receiver
return MyProps{}, structpages.RenderComponent(UserStatsWidget(stats)) // standalone component
```

**Reflective (framework finds the page and applies DI):**

```go
return structpages.RenderComponent(MyPage.ItemList)        // params DI-injected by the framework
return structpages.RenderComponent(MyPage.ItemList, items) // explicit args fill non-injected params, checked at runtime
```

Reserve the reflective form for components whose parameters the framework should DI-inject. Argument count and assignability are validated before the call — mismatches surface as readable errors, but at runtime, not compile time.

A custom `RenderTarget` that also implements `Component() component` can be rendered with `RenderComponent(target)` (no args).

## HTMXRenderTarget

```go
func HTMXRenderTarget(r *http.Request, pn *PageNode) (RenderTarget, error)
```

The default `TargetSelector`. Non-HTMX requests (no `HX-Request: true`), or HTMX requests with no `HX-Target`, select the `Page` method. Otherwise the `HX-Target` value is matched against the page's components:

- **Pass 0 — authoritative**: compare against each component's *real generated id* — the same value `ID()` emits, including the full field-path prefix and any length-budget compaction. This is the true inverse of `ID`/`IDTarget`.
- **Pass 1 — exact heuristics**: `<pageprefix>-<componentid>`, then bare `<componentid>`.
- **Pass 2 — suffix match (longest wins)**: full id ends with target; target ends with full id; or target ends with `<componentid>` *only when* target starts with `<pageprefix>-` (guards against cross-page false matches).

If no method matches, the raw target is carried as a function target and bound lazily when Props calls `target.Is(SomeFunc)` — this is how standalone component functions become HTMX targets.

### HTMXv4RenderTarget

```go
structpages.WithTargetSelector(structpages.HTMXv4RenderTarget)
```

htmx 4 variant. htmx 4 sends `HX-Target` as `"<tag>#<id>"` (or bare `"<tag>"`) and adds `HX-Request-Type: full|partial`. The v4 selector treats `HX-Request-Type: full` as a hard hint to render `Page`, prefers the id portion of the target, falls back to the tag for id-less targets, and otherwise applies the same matching rules.

## Error types

### ErrSkipPageRender

```go
var ErrSkipPageRender = errors.New("skip page render")
```

Return from `Props` to skip rendering when the response was written directly (rare — prefer the [`Redirect` signal](./error-handling.md#redirects-a-control-flow-signal-not-httpredirect)). Only the Props error path checks this sentinel.

## Buffered response

Error-returning `ServeHTTP` (and every `Props`) runs against a buffered writer: on error the buffer is discarded and `WithErrorHandler` renders instead. The no-return forms are unbuffered. For streaming through either form, `http.NewResponseController(w)` reaches the real flusher via the `Unwrap()` chain. Full rules and patterns: [Error Handling](./error-handling.md).
