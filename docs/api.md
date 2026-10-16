---
title: API Guide
slug: /api
sidebar_position: 12
---

# API Guide

The curated guide to the public API: what each piece is for, and where the guides cover it. The generated [Package Reference](./reference/package.md) lists every exported symbol.

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

Builds the page tree without registering routes. Use in tests and tooling that need `URLFor`/`ID`/`IDTarget` against the real page tree but don't want an HTTP server. Accepts the same options as `Mount`; options that only matter when serving (middlewares, request args, the target selector) are inert, and pages' `RequestArgs` methods aren't called.

## StructPages methods

```go
func (sp *StructPages) URLFor(page any, args ...any) (string, error)
func (sp *StructPages) ID(v any) (string, error)
func (sp *StructPages) IDTarget(v any) (string, error)
func (sp *StructPages) PageContext(ctx context.Context) context.Context
```

Use the method forms outside request context (initialization, boot-time validation, tests). Within request handlers and component renders, use the context-based package functions — the framework injects the parse context via internal middleware.

`PageContext` wraps a bare context with `sp`'s page tree so the context-form functions resolve against it. The recommended test pattern: `Parse` once per package, wrap `context.Background()` in `PageContext`, render against the wrapped ctx (see [Testing](./testing.md#testing-renders-with-a-bare-context)).

## Context functions

```go
func URLFor(ctx context.Context, page any, args ...any) (string, error)
func ID(ctx context.Context, v any) (string, error)
func IDTarget(ctx context.Context, v any) (string, error)
func CurrentPage(ctx context.Context) *PageNode
func Within(ctx context.Context, page any) (bool, error)
```

Page-argument forms, params formats, strict-mode semantics, and chain composition are covered in [URLFor & ID](./urlfor.md). Id-generation semantics (full field-path ids, multi-mount behavior, length budget) are covered in [HTMX Integration](./htmx.md#how-ids-are-generated).

`CurrentPage` returns the `*PageNode` of the route currently being served, or `nil` outside a request (a bare context, or one wrapped only by `PageContext`). It is set before a matched Props/Component page renders, so handlers, `Props`, and the components they render can identify the current page without threading it through every call — e.g. shared layout chrome deciding active-nav state. Pages served by their own `ServeHTTP` do not set it.

`Within` reports whether the current page is `page` or one of its descendants: the active-navigation test. `page` takes the forms `URLFor` takes (a page value, `Ref`, a top-level string, a `[]any` chain of page steps without string fragments, or a predicate). A group target matches every page under it, and pages compare by node, so a `/{$}` index leaf matches only itself, not its siblings. Outside a page render (no `CurrentPage`) it returns `false, nil`; a page that doesn't resolve is an error. See [Active navigation](./urlfor.md#active-navigation).

## Options

### WithArgs

```go
structpages.WithArgs(store, sessionManager, logger)
```

Register dependency-injection values, matched by type into `Props` / `ServeHTTP` / component / `Middlewares` / `RequestArgs` / `Init` parameters. Each type registers once; see [Dependency Injection](./dependency-injection.md) for the matching rules and named-type disambiguation.

### WithRequestArgs

```go
structpages.WithRequestArgs(currentUser) // func(*http.Request) (*User, error)
```

Register per-request values: each provider is a `func(*http.Request) (T, error)` or `func(*http.Request, *PageNode) (T, error)`, and its result fills parameters of type `T` in `Props`, extended `ServeHTTP` and component methods. A provider runs at most once per request, the first time a parameter needs it; its error stops the request like an error from `Props`. A page's `RequestArgs` method adds providers for part of the tree. See [Request args](./dependency-injection.md#request-args).

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
structpages.WithTargetSelector(structpages.HTMXRenderTarget)   // htmx 1.x and 2.x
structpages.WithTargetSelector(structpages.HTMXv4RenderTarget) // htmx 4
```

Set how every request is read: the selector returns the `RenderTarget`, what the request wants. **There is no default htmx selector**: without this option every request wants `Page`, so an htmx app must choose the selector for its htmx version. A custom `TargetSelector` can return any `RenderTarget`; the framework only calls its `Is`. See [How a Request Is Rendered](./rendering.md#what-the-request-wants-the-target-selector) and [custom target selectors](./htmx.md#custom-target-selectors).

### WithMaxIDLength

```go
structpages.WithMaxIDLength(60) // default 40
```

Character budget for generated element ids before they degrade from the readable full-path form (`admin-users-user-list`) to the compact form of the last path segment and the method (`users-user-list`, plus a stable hash suffix when the leaf name is not unique). Affects id generation only, never routing.

### WithURLPrefix

```go
structpages.WithURLPrefix("/app")
```

Prefix prepended to all generated URLs — for apps served under a sub-path. See [Serving under a path prefix](./routing.md#serving-under-a-path-prefix).

### WithWarnEmptyRoute

```go
structpages.WithWarnEmptyRoute(func(pn *structpages.PageNode) {
    log.Printf("skipping empty page: %s", pn.Name)
})
```

Called for each page with no handler and no children, which is otherwise skipped silently. Nothing is reported by default.

## Page methods

Pages can implement these optional methods. Parameters are matched by **type**, in any order. Per request (`Props`, extended `ServeHTTP`, components): `*http.Request`, `http.ResponseWriter`, `structpages.RenderTarget`, `*structpages.PageNode`, request args, and `WithArgs` values; components take `Props`' results first. Once at `Mount` (`Middlewares`, `RequestArgs`, `Init`): `*structpages.PageNode` and `WithArgs` values. See [Dependency Injection](./dependency-injection.md#what-each-method-can-receive).

### Page

```gsx
component (p myPage) Page(props MyProps) { … }
```

The main render entry — a page component composing the full page, and what renders when the request wants none of the page's other components. Pages without a `Page` method can still render by returning `RenderComponent(...)` from Props.

Any other method returning a component (`Content`, `UserList`, …) is a partial: it renders when the request's target `Is` it and `Props` returns no error. Its parameters are filled from `Props`' results by type, so each component declares only what it needs.

### Props

```go
func (p myPage) Props(r *http.Request, target structpages.RenderTarget, store *Store) (MyProps, error)
```

The normal page flow. Returning `(results…, nil)` renders what the request wants (the component the target `Is`, else `Page`) with those results; returning an error breaks the flow: `RenderComponent(...)` renders something else, `ErrSkipPageRender` means `Props` already responded, and any other error goes to the error handler. See [How a Request Is Rendered](./rendering.md#props-the-normal-flow-or-a-break).

Only the method literally named `Props` is auto-invoked. It gets the unbuffered writer — set headers or cookies if needed, but return errors rather than writing the body (see [Error Handling](./error-handling.md)).

### ServeHTTP

A page that handles the request itself: four signatures, with or without injected dependencies and an `error` return. Only the error-returning forms are buffered. The signatures are listed in [Page Response Patterns](./supported-flows.md#servehttp-signatures); the buffering rules are in [Error Handling](./error-handling.md).

In the DI forms, `RenderTarget` is also injectable, so a handler method can branch on `target.Is(...)` before responding.

### Middlewares

```go
func (p T) Middlewares(deps ...) []structpages.MiddlewareFunc
```

Page-specific middleware, also applied to all descendants.

### RequestArgs

```go
func (p T) RequestArgs(deps ...) []any
```

Request-arg providers for this page and every page below it, in the shapes `WithRequestArgs` takes. Called once at `Mount`, with `WithArgs` values injected, so providers can capture services. A page's providers shadow an ancestor's, or a mount-wide one, of the same type. See [Request args for part of the tree](./dependency-injection.md#request-args-for-part-of-the-tree).

### Init

```go
func (p *T) Init(deps ...) error
```

One-time setup at `Mount`; errors abort the mount. See [Dependency Injection](./dependency-injection.md#init).

## RenderTarget

```go
type RenderTarget interface {
    Is(component any) bool
}
```

What the request wants, as read by the target selector. Injected into `Props`, DI-form `ServeHTTP` and components. `Is` takes a component of this page (`target.Is(p.UserList)`), another page's component (`target.Is(admin.Users.List)`) or a standalone component function (`target.Is(UserStatsWidget)`), and has no side effects. When `Props` returns no error, the framework asks `Is` for each of the page's components and renders the one the target is, else `Page`; a target that is several components is an error.

An element rendered with id `ID(ctx, X)` produces a request whose target `Is(X)`. See [Regions shared across pages](./rendering.md#regions-shared-across-pages).

## RenderComponent

```go
func RenderComponent(comp interface{ Render(context.Context, io.Writer) error },
    more ...interface{ Render(context.Context, io.Writer) error }) error
```

Breaks `Props`' normal flow: the framework renders `comp`, then each of `more`, in order, as one response. Honored in the Props error path and the buffered-`ServeHTTP` error path; when returned from Props, the other return values are ignored. Several components update several regions at once, for example with htmx 4 `<hx-partial>` blocks; see [Updating several regions](./rendering.md#updating-several-regions).

`comp` is any component — a `gsx.Node`, a `templ.Component`, or a page partial called with its arguments — so the compiler checks those arguments at the call site:

```go
return MyProps{}, structpages.RenderComponent(p.UserList(users))      // same page
return structpages.RenderComponent(index{}.TodoList(todos))           // another page — zero-value receiver
return MyProps{}, structpages.RenderComponent(UserStatsWidget(stats)) // standalone component
```

## HTMXRenderTarget

```go
structpages.WithTargetSelector(structpages.HTMXRenderTarget)
```

The `TargetSelector` for htmx 1.x and 2.x; opt in with `WithTargetSelector`. Non-HTMX requests (no `HX-Request: true`), or HTMX requests with no `HX-Target`, select the `Page` method. Otherwise the `HX-Target` value is matched against the page's components:

- **Pass 0 — authoritative**: compare against each component's *real generated id* — the same value `ID()` emits, including the full field-path prefix and any length-budget compaction. This is the true inverse of `ID`/`IDTarget`.
- **Pass 1 — exact heuristics**: `<pageprefix>-<componentid>`, then bare `<componentid>`.
- **Pass 2 — suffix match (longest wins)**: full id ends with target; target ends with full id; or target ends with `<componentid>` *only when* target starts with `<pageprefix>-` (guards against cross-page false matches).

If no component matches, the target `Is` none of the page's components, so `Props` returning no error renders `Page`. It still answers `Is` for a standalone component function carrying that id (its package-qualified id, or the bare and page-prefixed forms) and for another page's component carrying exactly its generated id, so `Props` can recognise a region it shares and return `RenderComponent`.

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

Return from `Props` to skip rendering when the response was written directly (rare — prefer the [`Redirect` signal](./error-handling.md#redirects-a-control-flow-signal-not-http-redirect)). The Props and component paths, and a request arg's error, honour it; an error-returning `ServeHTTP` that returns it sends it to the error handler.
