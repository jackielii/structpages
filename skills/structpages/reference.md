# structpages API Reference

Signatures are from the package source. Markup examples are gsx with the `url`/`id`/`target` filters
registered as in SKILL.md; for templ see [templ.md](templ.md).

## Mount and Parse

```go
func Mount(mux Mux, page any, route, title string, options ...Option) (*StructPages, error)
func Parse(page any, route, title string, options ...Option) (*StructPages, error)
```

`Mount` parses the page tree, calls every `Init`, and registers handlers on `mux` (`nil` means
`http.DefaultServeMux`). `route` is the base path (usually `"/"`), `title` the root page title.

`Parse` builds the same tree without registering anything; use it in tests and tooling. It applies
`WithArgs`, `WithURLPrefix` and `WithTargetSelector`; middleware and error-handler options are inert. It
currently ignores `WithMaxIDLength`, so ids generated under `Parse` use the default budget of 40.

```go
type Mux interface {
    Handle(pattern string, handler http.Handler)
}
```

## *StructPages

```go
func (sp *StructPages) URLFor(page any, args ...any) (string, error)
func (sp *StructPages) ID(v any) (string, error)
func (sp *StructPages) IDTarget(v any) (string, error)
func (sp *StructPages) PageContext(ctx context.Context) context.Context
```

The methods resolve against the tree without a request: use them at boot (URL validation, building
config) and in tooling. They cannot auto-fill path params from a current request, and `ID`/`IDTarget`
have no current mount, so a type mounted twice is ambiguous.

`PageContext` returns `ctx` carrying the tree, so the context functions and gsx filters work under a bare
`context.Background()` (SKILL.md §8).

## Context functions and gsx filters

```go
func URLFor(ctx context.Context, page any, args ...any) (string, error)
func ID(ctx context.Context, v any) (string, error)
func IDTarget(ctx context.Context, v any) (string, error)
func CurrentPage(ctx context.Context) *PageNode
```

All three resolvers return an error when `ctx` carries no page tree (outside a request and without
`PageContext`).

Registered as gsx filters (`gsx.toml`):

```toml
[filters]
url    = "github.com/jackielii/structpages.URLFor"
id     = "github.com/jackielii/structpages.ID"
target = "github.com/jackielii/structpages.IDTarget"
```

| gsx | Go equivalent |
|---|---|
| `{ x \|> url }` | `structpages.URLFor(ctx, x)` |
| `{ x \|> url(params) }` | `structpages.URLFor(ctx, x, params)` |
| `{ x \|> id }` | `structpages.ID(ctx, x)` |
| `{ x \|> target }` | `structpages.IDTarget(ctx, x)` |

Filters work in element and component attribute holes, text holes, and `@{}` holes of `js`/`f` literals
on attributes; a non-nil error is returned from `Render`. gsx rejects them in a literal assigned inside a
`{{ }}` block, which has no error channel. A direct call such as
`href={structpages.URLFor(ctx, x)}` also compiles, because gsx holes accept `(string, error)`, but the
filter is the idiom. Do not register the package with `filter_packages`: names would become `uRLFor`.

`CurrentPage` returns the matched leaf `*PageNode` while serving a `Props`/component page, nil otherwise
(including `ServeHTTP` pages and `PageContext`). Walk `Parent` for active-navigation state.

## URLFor

### Page argument

1. **Typed page value** — `Detail{}`. Strict: a type mounted under several parents is an error listing
   every match. A page group (only child routes, no render of its own) resolves to its `/{$}` child, so
   the URL is the canonical `/section/` rather than a redirecting `/section`.
2. **`[]any` composition** — leading typed values are a chain: the first resolves normally, each next one
   descends into a uniquely typed child. Strings after the chain are appended literally (query templates,
   suffixes). A typed value after a string is an error.
3. **`Ref`** — `structpages.Ref("Parent.Field")` walks field names; the first segment matches a top-level
   node or any uniquely named node. `Ref("Name")` matches by name; `Ref("/route/{x}")` by route pattern.
   A top-level plain string is sugar for `Ref`: `"Admin.Settings" |> url`. Strings inside `[]any` stay URL
   fragments.
4. **Predicate** — `func(*PageNode) bool`, an escape hatch.

### Arguments

Detected in this order:

- **Map** (recommended): one `map[string]any`, values looked up by placeholder name, path and query alike.
- **Positional**: argument count equals placeholder count; filled left to right.
- **Key/value pairs**: even count, string keys, at least one key naming a placeholder
  (`url("itemId", 7)`).
- **Auto-fill**: unfilled placeholders that are path params of the current request's route are taken from
  the request. Params of other routes are not.

Path values are escaped per segment; `{path...}` wildcards keep their slashes. `{$}` is removed and
`WithURLPrefix` is prepended.

## ID and IDTarget

Input forms:

- **Method expression** `Index.UserList` or **bound method** `p.UserList` → `index-user-list` (page path +
  method).
- **Standalone function** `StatsWidget` → `<package>-stats-widget` (short package name prefix).
- **Chain** `[]any{adminRoot{}, dashboard{}, "Header"}` or `[]any{adminRoot{}, dashboard.Header}`; when the
  trailing method expression's receiver and the explicit leaf both appear they must agree.
- **`Ref`** `Ref("Index.UserList")`, or `Ref("UserList")` when unambiguous.
- **Plain string** → returned unchanged, by both functions (`IDTarget(ctx, "body")` is `body`, not `#body`).
  `ID`/`IDTarget` have no string-as-`Ref` sugar; wrap in `Ref(...)` for a lookup.

`IDTarget` prefixes `#` to every resolved id.

**Format.** The kebab-cased field-name path from the root (root excluded) joined with the method:
`admin-users-user-list`. Longer than the budget (`WithMaxIDLength`, default 40) it becomes the leaf form
`user-list`, plus a 4-hex hash when another node shares the leaf name. An id only changes when that node
is renamed or moved. Kebab conversion: `HTMLParser` → `html-parser`.

**Mount context.** During a page's own render the current mount is used, so `p.Header |> id` differs per
mount of the same struct. Without a current mount (another page, `sp.ID`) a type mounted twice is an error
naming the mounts; disambiguate with a chain, a `Ref`, or a standalone component.

## Options

| Option | Effect |
|---|---|
| `WithArgs(args ...any)` | DI registry. Each type once; duplicates fail `Mount`. |
| `WithErrorHandler(func(http.ResponseWriter, *http.Request, error))` | Renders every error from `Props`, `ServeHTTP`, component lookup and render. Default: plain 500. |
| `WithMiddlewares(mw ...MiddlewareFunc)` | Global middleware, first is outermost, runs before page `Middlewares`. |
| `WithTargetSelector(TargetSelector)` | Chooses the `RenderTarget`. Default `HTMXRenderTarget`; use `HTMXv4RenderTarget` for htmx 4. |
| `WithURLPrefix(prefix string)` | Prefix added to generated URLs when served behind `StripPrefix` or a proxy. Routing unchanged. |
| `WithMaxIDLength(n int)` | Id budget before the compact form. Ids only, never routes. |
| `WithWarnEmptyRoute(func(*PageNode))` | Called for pages with no handler and no children (skipped). `nil` prints a default warning; a no-op func silences it. |

## Page methods

Discovered on both the value and pointer type; promoted methods are skipped.

### Components

Any method returning exactly one value that implements `Render(context.Context, io.Writer) error` is a
component. In gsx, `component (p Index) Page(props indexProps)` generates
`func (p Index) Page(props indexProps) gsx.Node`. Parameters are matched by type against the `Props`
results, then the DI registry.

- `Page` is rendered for full loads.
- Any other component is rendered when its id matches `HX-Target`.
- With no matching component and no `Props`-issued `RenderComponent`, the request errors (a static
  `HX-Target` that matches nothing falls back to `Page`).

### Props

```go
func (p Index) Props(r *http.Request, w http.ResponseWriter, sel structpages.RenderTarget, s *store.Store) (indexProps, error)
```

Parameters are DI-matched in any order: `*http.Request`, `http.ResponseWriter`, `RenderTarget`,
`*PageNode`/`PageNode`, and `WithArgs` values. Results: any number of values (passed to the component),
optionally a trailing `error`.

The writer is **not** buffered. Headers and cookies are fine; a body write is sent immediately and is not
undone by a later error. Return `ErrSkipPageRender` when `Props` wrote the whole response itself; return
`RenderComponent(...)` to render something other than the selected component.

Only `Props` is invoked. Other `*Props` methods are recorded in `PageNode.Props` but never called.

### ServeHTTP

1. `ServeHTTP(w, r)` — `http.Handler`; direct writes.
2. `ServeHTTP(w, r) error` — buffered.
3. `ServeHTTP(w, r, deps...)` — DI, no result; direct writes.
4. `ServeHTTP(w, r, deps...) error` — DI, buffered.

Buffered forms: on error the buffer is reset, then `RenderComponent` errors render their component and
anything else goes to `WithErrorHandler`. `ErrSkipPageRender` has no special meaning here. In the DI forms
`RenderTarget` is injectable. A page with `ServeHTTP` never runs `Props` or components. For streaming,
`http.NewResponseController(w).Flush()` works through the buffered writer (it implements `FlushError` and
`Unwrap`).

### Middlewares

```go
func (p adminPages) Middlewares(deps ...) []structpages.MiddlewareFunc
```

DI-matched parameters; the result type must be exactly `[]structpages.MiddlewareFunc`. Applies to the page
and its descendants, after global middleware.

### Init

```go
func (p *Index) Init(deps ...) error
```

Called once while parsing, with DI. A returned error aborts `Mount`/`Parse`. Use a pointer receiver to keep
state on the page value.

## RenderTarget

```go
type RenderTarget interface {
    Is(method any) bool
}
type TargetSelector func(r *http.Request, pn *PageNode) (RenderTarget, error)
```

`Is` accepts method expressions, bound methods and standalone functions. It matches a standalone function
by its package-qualified id (the one `id` generates), and also by the bare or page-prefixed function id.
For a function target, `Is` records the matched function; `RenderComponent(sel, args...)` needs that.

### HTMXRenderTarget (default) and HTMXv4RenderTarget

- Not an HTMX request (`HX-Request` ≠ `true`) or no `HX-Target` → `Page`.
- Otherwise the target is matched to a component: exact generated id first, then `page-prefix-component`,
  then bare `component`, then the longest suffix match.
- No component match → a function target, resolved lazily by `Is(fn)` in `Props`.

`HTMXv4RenderTarget` additionally treats `HX-Request-Type: full` as a full page and reads the id from
htmx 4's `tag#id` `HX-Target` (falling back to the tag, so a `Form` component matches `hx-target="form"`).

### Custom selectors

A custom `RenderTarget` that also has `Component() <component>` can be returned as
`RenderComponent(target)`; the framework calls `Component()`.

## RenderComponent

```go
func RenderComponent(targetOrMethod any, args ...any) error
```

Returned as an error from `Props` or a `ServeHTTP` that returns `error`; other `Props` results are ignored.

| First argument | Args | Resolution |
|---|---|---|
| component value (`p.List(items)`, `<Widget n={1}/>`) | none | rendered directly; compile-time checked |
| custom target with `Component()` | none | `Component()` result rendered |
| `RenderTarget` after `Is` matched | optional | method target: called on the current page; function target: the recorded function |
| method expression `Index.List` / bound method | optional | owning mounted page found; missing params DI-filled |
| other function | must match | called with `args` |

Argument count and types are checked before the call and reported as errors.

## ErrSkipPageRender

```go
var ErrSkipPageRender = errors.New("skip page render")
```

Honoured only from `Props`: the request ends without rendering or calling the error handler.

## PageNode and MiddlewareFunc

```go
type PageNode struct {
    Name, Title, Method, Route string
    Value       reflect.Value
    Props       map[string]reflect.Method
    Components  map[string]reflect.Method
    Middlewares *reflect.Method
    Parent      *PageNode
    Children    []*PageNode
}
func (pn *PageNode) FullRoute() string
func (pn *PageNode) All() iter.Seq[*PageNode]

type MiddlewareFunc func(http.Handler, *PageNode) http.Handler
```

`Method` is `"ALL"` when the tag has none. Middleware is invoked once per route at registration, so the
outer function can inspect the node (e.g. collect a route table) and return the wrapped handler.

## Route tag

```
route:"[METHOD] /path [Title]"
```

- Methods: `GET`, `HEAD`, `POST`, `PUT`, `PATCH`, `DELETE`, `CONNECT`, `OPTIONS`, `TRACE`, `ALL`; omitted means `ALL`.
- Paths use Go 1.22 `ServeMux` patterns: `{name}`, `{name...}`, `{$}`.
- Child routes are joined to the parent with `path.Join`, which removes trailing slashes; use `{path...}` for
  prefix subtrees.

## Lint

```shell
go install github.com/jackielii/structpages/tools/lint/cmd/structpages-lint@latest
structpages-lint ./...
```

| Category | Flags |
|---|---|
| `urlfor` | `URLFor` targets: unmounted or ambiguous type, unknown chain child, typed value after a fragment |
| `ref` | `Ref` strings (including ones stored in variables or struct fields) that resolve to no node |
| `params` | `URLFor` params that are not placeholders in the pattern |
| `idfor` | `ID`/`IDTarget` method expressions whose receiver is not mounted, or whose method is missing on a chain leaf |
| `route-literal` | `.go` string literals equal to a concrete mounted route (comparisons, `Ref` args, tests and generated files skipped) |
| `url-attr` | hard-coded internal URLs in URL-bearing attributes of **`.templ`** files; `.gsx` is not scanned yet |

Suppress with `//structpages:lint:ignore <category>[,<category>]` on the line or the line above; with no
category it suppresses everything on that line.
