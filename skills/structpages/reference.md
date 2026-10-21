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
`WithArgs`, `WithURLPrefix`, `WithTargetSelector` and `WithMaxIDLength`, so its ids match `Mount`'s;
middleware and error-handler options are inert. Unlike `Mount`, it doesn't resolve request args or check
`Props`, `ServeHTTP` and component parameters for ambiguous dependencies.

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
have no current mount, so a type mounted twice (with differing ids) is ambiguous.

`PageContext` returns `ctx` carrying the tree, so the context functions and gsx filters work under a bare
`context.Background()` (SKILL.md §8).

## Context functions and gsx filters

```go
func URLFor(ctx context.Context, page any, args ...any) (string, error)
func ID(ctx context.Context, v any) (string, error)
func IDTarget(ctx context.Context, v any) (string, error)
func CurrentPage(ctx context.Context) *PageNode
func Within(ctx context.Context, page any) (bool, error)
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
(including `ServeHTTP` pages and `PageContext`).

`Within(ctx, page)` reports whether the current page is `page` or below it; use it for active navigation
instead of matching path prefixes (examples.md §15). `page` takes `URLFor`'s page forms; a `[]any` chain may
not carry string fragments. A group target matches every page under it, path-param routes included, with no
args. A `/{$}` index leaf matches only itself. It returns `false, nil` with no current page, and an error when
`page` doesn't resolve.

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

Pass values raw: each is escaped for the part of the URL its placeholder sits in. Path values are escaped per
segment, and `{path...}` wildcards keep their slashes. Query and fragment values are escaped as one component,
so `&`, `=`, `+` and `#` stay data. Don't `url.QueryEscape` first, since that escapes twice. Template text is used as
is, so put pre-encoded query text there (`"?" + values.Encode()`). Auto-filled values come from `r.PathValue`
(already decoded) and are escaped the same way. `{$}` is removed and
`WithURLPrefix` is prepended.

## ID and IDTarget

Input forms:

- **Method expression** `Index.UserList` or **bound method** `p.UserList` → `index-user-list` (page path +
  method).
- **Standalone function** `StatsWidget` → `<package>-stats-widget` (short package name prefix).
- **Chain** `[]any{adminRoot{}, dashboard{}, "Header"}` or `[]any{adminRoot{}, dashboard.Header}`. A trailing
  method expression whose receiver is the last chain step collapses into it; otherwise its receiver is
  resolved as one more child step.
- **`Ref`** `Ref("Index.UserList")`, or `Ref("UserList")` when unambiguous.
- **Plain string** → returned unchanged, by both functions (`IDTarget(ctx, "body")` is `body`, not `#body`).
  `ID`/`IDTarget` have no string-as-`Ref` sugar; wrap in `Ref(...)` for a lookup.

`IDTarget` prefixes `#` to every resolved id.

**Format.** The kebab-cased field-name path from the root (root excluded) joined with the method:
`admin-users-user-list`. Longer than the budget (`WithMaxIDLength`, default 40) it becomes the leaf page plus
method, `users-user-list`, with a 4-hex hash appended when another node shares the leaf page name. An id only changes when that node
is renamed or moved. Kebab conversion: `HTMLParser` → `html-parser`.

**Mount context.** During a page's own render the current mount is used, so `p.Header |> id` differs per
mount of the same struct. Without a current mount (another page, `sp.ID`) a type mounted twice with
differing ids is an error naming the mounts; disambiguate with a chain, a `Ref`, or a standalone
component.

## Options

| Option | Effect |
|---|---|
| `WithArgs(args ...any)` | DI registry, fixed at `Mount`. Each type once; duplicates fail `Mount`. An interface parameter takes a registered pointer to that interface, else the one registration implementing it. A `Props`/`ServeHTTP`/component interface parameter that two registrations (or two request args at one level) implement fails `Mount`; one nothing provides fails the request. |
| `WithRequestArgs(providers ...any)` | Per-request DI, mount-wide. Each provider is `func(*http.Request) (T, error)` or `func(*http.Request, *PageNode) (T, error)`; runs lazily, at most once per request, when a parameter of type `T` needs it. Matched like `WithArgs`. Another shape, or two providers of one type, fail `Mount`. A provider's error counts as the error of the method that needed it. |
| `WithErrorHandler(func(http.ResponseWriter, *http.Request, error))` | Renders every error from `Props`, `ServeHTTP`, component lookup and render. Default: plain 500. |
| `WithMiddlewares(mw ...MiddlewareFunc)` | Global middleware, first is outermost, runs before page `Middlewares`. |
| `WithTargetSelector(TargetSelector)` | Reads what each request wants into a `RenderTarget`. **No default**: without it every request wants `Page`. htmx apps pass `HTMXRenderTarget` (htmx 1/2) or `HTMXv4RenderTarget` (htmx 4). |
| `WithURLPrefix(prefix string)` | Prefix added to generated URLs when served behind `StripPrefix` or a proxy. Routing unchanged. |
| `WithMaxIDLength(n int)` | Id budget before the compact form. Ids only, never routes. |
| `WithWarnEmptyRoute(func(*PageNode))` | Called for pages with no handler and no children (skipped). `nil` prints a default warning; a no-op func silences it. |

## Page methods

Discovered on both the value and pointer type; promoted methods are skipped.

### Components

Any method returning exactly one value that implements `Render(context.Context, io.Writer) error` is a
component. In gsx, `component (p Index) Page(props indexProps)` generates
`func (p Index) Page(props indexProps) gsx.Node`. Parameters are filled by type from, in order: `Props`'
results (each used at most once), the per-request values (`http.ResponseWriter`, `*http.Request`,
`RenderTarget`, `*PageNode`), request args, then `WithArgs`. So one `Props` returning several values serves
several components, each declaring only what it needs.

Which component renders, when `Props` returns no error (or there is no `Props`): the framework asks
`target.Is(m)` for each of the page's components and renders the one it `Is`; if it `Is` none, `Page`; if it
`Is` several, the request errors. With an htmx selector, the target `Is` the component `HX-Target` names,
and an `HX-Target` matching none of the page's components renders `Page`. A page with no `Page` component
whose target `Is` none must return `RenderComponent` from `Props`, or the request errors.

### Props

```go
func (p Index) Props(r *http.Request, w http.ResponseWriter, t structpages.RenderTarget, s *store.Store) (indexProps, error)
```

Parameters are DI-matched in any order: `*http.Request`, `http.ResponseWriter`, `RenderTarget`,
`*PageNode`/`PageNode`, request args, and `WithArgs` values. Results: any number of values (filling the
rendered component's parameters by type), optionally a trailing `error`.

| `Props` returns | Result |
|---|---|
| `(results…, nil)` | normal flow: the component the target `Is`, else `Page`, rendered with `results` |
| `RenderComponent(c, more...)` | render `c`, then `more`, in order, as one response |
| `ErrSkipPageRender` | `Props` already responded (e.g. wrote a redirect); nothing renders |
| any other error | `WithErrorHandler` |

Calling `t.Is(...)` changes nothing; recognising a target and returning no error is still the normal flow.

The writer is **not** buffered. Headers and cookies are fine (set `Content-Type`, then `w.WriteHeader(status)`, before returning
`RenderComponent` to send a status: the framework can't set the header after the status); a body write is sent immediately and is not undone by a later error.

Only `Props` is invoked. Other `*Props` methods are recorded in `PageNode.Props` but never called.

### ServeHTTP

1. `ServeHTTP(w, r)` — `http.Handler`; direct writes.
2. `ServeHTTP(w, r) error` — buffered.
3. `ServeHTTP(w, r, deps...)` — DI, no result; direct writes.
4. `ServeHTTP(w, r, deps...) error` — DI, buffered.

Buffered forms: on error the buffer is reset, then `RenderComponent` errors render their components and
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

### RequestArgs

```go
func (p channelRoutes) RequestArgs(deps ...) []any
```

Request-arg providers (same shapes as `WithRequestArgs`) for the page and its descendants. Called once at
`Mount`, DI-matched from `WithArgs`, so providers can close over services. Nearest first: the page's own,
then each ancestor's, then `WithRequestArgs`; a nearer provider shadows an outer one of the same type.

## RenderTarget

```go
type RenderTarget interface {
    Is(component any) bool
}
type TargetSelector func(r *http.Request, pn *PageNode) (RenderTarget, error)
```

`Is` reports whether the request wants `component`: a method expression, a bound method or a standalone
function. It has no side effects. The framework calls it for each of the page's components to pick the one
to render; `Props` can call it to load only what the request wants.

**`ID`/`Is` symmetry:** a request targeting an element with id `ID(ctx, X)` has a target that `Is(X)`, for
the served page's components, standalone functions, and other pages' components.

### HTMXRenderTarget and HTMXv4RenderTarget

Opt-in selectors (`WithTargetSelector`).

- Not an HTMX request (`HX-Request` ≠ `true`) or no `HX-Target` → `Page`.
- Otherwise `HX-Target` is matched to one of the served page's components: an exact generated id, or a
  `page-prefix-component` or bare `component` match (checked together, so don't rely on precedence between
  them), then the longest suffix match. The target `Is` that component.
- No component match → the target `Is` none of the page's components (so `Page` renders unless `Props`
  breaks the flow), but answers `Is(fn)` for a standalone function by its package-qualified id (also the
  bare or page-prefixed function id), and `Is(Other.Method)` for another page's component by its exact
  generated id only. `Props` recognises such a shared region and renders it with `RenderComponent`.

`HTMXv4RenderTarget` additionally treats `HX-Request-Type: full` as a full page and reads the id from
htmx 4's `tag#id` `HX-Target` (falling back to the tag, so a `Form` component matches `hx-target="form"`).

### Custom selectors

A custom `TargetSelector` may return any `RenderTarget`; the framework only calls `Is`, so a custom target
selects the page's component it `Is` like the built-in ones (`Page` if none). Typically it handles its own
case and delegates the rest to `HTMXRenderTarget`/`HTMXv4RenderTarget`. `Props` can type-assert the target
to read what it carries.

## RenderComponent

```go
func RenderComponent(comp interface{ Render(context.Context, io.Writer) error },
    more ...interface{ Render(context.Context, io.Writer) error }) error
```

Returned as an error from `Props` or a `ServeHTTP` that returns `error`, it breaks the normal flow: `comp`
then each of `more` render in order, buffered, as one response; other `Props` results are ignored. Use it
for any override: a different component, an inline failure notice, a standalone or other page's component,
or several regions (htmx 4: wrap extras in `<hx-partial hx-target={X |> target}>`; htmx 2: `hx-swap-oob` on
the extra component's root).

Each argument is a constructed component: a page partial called with its arguments (`p.List(items)`,
`Index{}.List(items)` for another page), a gsx element literal (`<Widget n={1}/>`), or any value with
`Render(ctx, w)`. The compiler checks its arguments.


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

`structpages-lint` checks every URL, id and `Ref` against the page tree it rebuilds from the `route` tags
and `Mount` calls, so a renamed route, moved page or mistyped param key fails CI instead of becoming a 404
or an error page. The compiler can't catch these: `Ref` strings, param keys and hard-coded `href`s are all
just strings to it. The bigger the tree, the more this matters. Run it in CI next to `go test` and after
any route, page or partial rename.

```shell
go get -tool github.com/jackielii/structpages/tools/lint/cmd/structpages-lint@latest
go tool structpages-lint ./...            # exit 1 on findings, 2 if packages fail to load
go tool structpages-lint -tags dev ./...  # include call sites behind build tags
```

Run it from the module root (or pass the packages holding the `Mount` calls): it needs them to rebuild the
tree, and exits 0 with a warning when there are none.

Every `Mount` and `Parse` call builds its own tree, test fixtures included. As at runtime, a call is
checked against the trees that can serve it: those mounted by a binary (or test binary) that compiles the
call in. It passes if any of them resolves it, so a test tree that stubs a section out, or mounts it at
`/` instead of `/orbit`, doesn't flag production code. A type is ambiguous only when one tree mounts it
twice. A helper taking the page as `any` (`func serve(t *testing.T, pages any)`) builds the tree of each
concrete page passed to it, through chains of helpers and across packages.

| Category | Flags |
|---|---|
| `urlfor` | `URLFor` targets: unmounted or ambiguous type, unknown chain child, typed value after a fragment |
| `ref` | `Ref` strings (including ones stored in variables or struct fields) that resolve to no node |
| `params` | `URLFor` param keys that are not placeholders in the pattern (fragment included): literal map keys, and constant keys of the key/value form, `url("itemId", 7)` |
| `escape` | `URLFor` values pre-escaped with `url.QueryEscape`/`url.PathEscape`/`Values.Encode()`, which URLFor escapes again: pass raw; an encoded query goes in the template (`"?" + v.Encode()`) |
| `idfor` | `ID`/`IDTarget` method expressions whose receiver is not mounted, or whose method is missing on a chain leaf; `RenderTarget.Is` arguments whose receiver is not mounted, or that aren't a function at all (`t.Is(p.Content())` calls instead of naming; a `Ref`/string never matches) |
| `selector` | a `Mount` with no `WithTargetSelector` whose pages' `.gsx`/`.templ` templates use htmx (`hx-get`…`hx-select`, `<hx-partial>`). Flagged only when every option is a direct `structpages` call (inline or a literal slice); a variable or `opts...` spread isn't. `_test.go` skipped |
| `route-literal` | `.go` string literals equal to a concrete mounted route (comparisons, `Ref` args, tests and generated files skipped) |
| `url-attr` | hard-coded internal URLs in URL-bearing attributes (`href`, `action`, `formaction`, `hx-get`/`post`/`put`/`patch`/`delete`, `hx-push-url`/`hx-replace-url`) of `.gsx` and `.templ` files: literals, `+` concatenation, `fmt.Sprint*`, and gsx `` f`/items/@{id}` `` templates |

gsx templates get two passes. gsx lowers `|> url`, `|> id` and `|> target` to real
`URLFor`/`ID`/`IDTarget` calls in the generated `.x.go`, which the analyzer checks like hand-written Go
(`urlfor`, `ref`, `params`, `escape`, `idfor`). Findings point at the `.gsx` through gsx's `//line` directives.
`url-attr` parses the `.gsx` itself. Keep the `.x.go` files generated before linting.

The key/value check follows URLFor's own rules. With as many args as placeholders the call is positional,
and so is an odd count. A non-constant key is skipped. Those calls aren't checked. An unknown key is
reported even when no key matches: URLFor then falls back to positional fill, and `url("nope", 1)` against
`/items/{id}` silently renders `/items/nope`.

Suppress with `//structpages:lint:ignore <category>[,<category>]` on the line or the line above; with no
category it suppresses everything on that line. In `.gsx` the directive is a gsx comment: bare `// …`,
braced `{/* … */}`, or inside an attribute list. Avoid `<!-- … -->` there, because it renders into every
response. It covers every category, including findings on a `|> url`/`|> id`/`|> target` call; that needs
gsx v0.4.0 or later, whose `//line` directives put attribute holes on their real line.
