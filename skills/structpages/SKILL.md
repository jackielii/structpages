---
name: structpages
description: >
  Guide for building Go web applications with the structpages framework (struct-based routing + gsx + HTMX).
  Use when writing routes, pages, page groups, Props methods, handler methods (ServeHTTP), page components,
  partials, HTMX partial rendering and nested swap levels, URL and element-id generation (the gsx
  `url`/`id`/`target` filters, URLFor/ID/IDTarget), RenderTarget/RenderComponent patterns, middleware or
  dependency injection with structpages. Also use when the user asks about structpages patterns, conventions,
  vocabulary, or debugging structpages issues. Covers templ and html/template users too.
---

# structpages

structpages routes HTTP requests with struct tags on top of `http.ServeMux`. A page is a struct; its
methods load data (`Props`), render (`Page`, `Content`, partials) or handle the request imperatively
(`ServeHTTP`). HTMX partial rendering, type-safe URLs and element ids are built in.

- [reference.md](reference.md) — exact API signatures and semantics, lint categories.
- [examples.md](examples.md) — larger worked patterns (two-pane pages, CRUD, error handling, static assets).
- [templ.md](templ.md) — read only if the project renders with templ instead of gsx.

## Renderers

The framework treats any value with `Render(context.Context, io.Writer) error` as a component, so it is
renderer-agnostic. **gsx** (`github.com/gsxhq/gsx`) is the primary path and what every example in the
repository uses; this guide is written for it. templ and `html/template` also work (see
[templ.md](templ.md) and examples.md §10).

### gsx setup: register the filters once

structpages exposes `URLFor`, `ID` and `IDTarget` as ordinary functions with a leading `ctx`. Register them
as gsx pipeline filters in the project's `gsx.toml` (gsx uses the nearest `gsx.toml` walking up to the
repository root):

```toml
[filters]
url    = "github.com/jackielii/structpages.URLFor"
id     = "github.com/jackielii/structpages.ID"
target = "github.com/jackielii/structpages.IDTarget"
```

gsx passes the render `ctx` and the piped value as the first two arguments, and the filter's error is
returned from `Render`. So `{ page |> url(params) }` is `structpages.URLFor(ctx, page, params)`,
`{ X |> id }` is `structpages.ID(ctx, X)` and `{ X |> target }` is `structpages.IDTarget(ctx, X)`.

```gsx
<a href={Detail{} |> url(map[string]any{"itemId": it.ID})}>{ it.Name }</a>
<form hx-post={Add{} |> url} hx-target={Index.TodoList |> target}>…</form>
<div id={Index.TodoList |> id}><p.TodoList todos={props.Todos}/></div>
<button @click=js`openIn(@{Index.Canvas |> target})`>Open</button>
<input hx-include=f`@{Index.Filters |> target} input`/>
```

Do not write a `must()` helper or app-level `urlFor`/`idFor`/`idForTarget` wrappers: holes and filters
already accept `(T, error)` and hoist the error out of `Render`. Full rules in §3.

## Vocabulary

| Term | What it is |
|---|---|
| **page** | a route-tagged struct, a node in the route tree |
| **page group** | a page with no render of its own (no `Page`, `Props` or `ServeHTTP`), only child pages; served through its `/{$}` child |
| **component** | a standalone `component Foo(...)`: mount-independent, package-prefixed id |
| **page component** | a method component `component (p Page) Foo(...)`: mount-aware, includes `Page` and `Content`. Composed inside another page component, or returned alone as a partial |
| **partial** | a page component (or component) rendered alone as an HTMX response; a role, not a kind |
| **Props method** | `Props(...)`: loads data through DI and returns the **props struct** handed to the page component |
| **handler method** | `ServeHTTP(...)`: mutate, redirect, serve JSON, or render via `RenderComponent` |
| **Middlewares method** | `Middlewares(...)`: middleware for the page and its descendants |

A layout is just a component that takes `children gsx.Node`; there is no layout route. `Content` is a
naming convention for a page's main region, not a framework concept.

## Request lifecycle

Route match → middleware → **TargetSelector** builds a `RenderTarget` from the request (default
`HTMXRenderTarget`) → **Props** (with the `RenderTarget` injectable) → render the selected page component
(`Page` for a full load, the component whose id matches `HX-Target` for a partial). A `Props` that returns
`RenderComponent(...)` renders that instead. A page with `ServeHTTP` skips Props and components entirely.

## 1. Routes

`route:"[METHOD] /path [Title]"`. No method means all methods.

```go
type pages struct {
    home   homePage   `route:"/{$} Home"`               // exact root
    about  aboutPage  `route:"/about About"`
    create createPage `route:"POST /items Create"`
    item   itemPage   `route:"/items/{itemId} Item"`
    files  filesPage  `route:"GET /files/{path...} Files"`
    admin  adminPages `route:"/admin Admin"`            // nested: children under /admin
}
type adminPages struct {
    dashboard dashboardPage `route:"/{$} Dashboard"` // /admin/
    users     usersPage     `route:"/users Users"`   // /admin/users
}
```

- **Name path params specifically** (`{itemId}`, not `{id}`): nested routes compose into one pattern and
  ServeMux rejects a repeated wildcard name.
- **Prefix subtrees use `{path...}`, not a trailing slash.** Routes are joined with `path.Join`, which
  drops the trailing slash, so `route:"/static/"` registers an exact match. See examples.md §12.
- Children register before parents; promoted (embedded) methods are ignored; only the `route:` tag is read.

## 2. Page shapes

### Props + Page (rendering page)

```gsx
type itemPage struct{}

type itemProps struct {
    Item store.Item
}

func (p itemPage) Props(r *http.Request, s *store.Store) (itemProps, error) {
    it, err := s.Item(r.Context(), r.PathValue("itemId"))
    if err != nil {
        return itemProps{}, err
    }
    return itemProps{Item: it}, nil
}

component (p itemPage) Page(props itemProps) {
    <Layout title={props.Item.Name}>
        <p.Content props={props}/>
    </Layout>
}

component (p itemPage) Content(props itemProps) {
    <h1>{ props.Item.Name }</h1>
}
```

`Props` parameters are matched by type (`*http.Request`, `http.ResponseWriter`, `RenderTarget`,
`*PageNode`, anything registered with `WithArgs`). Its non-error results are passed to the component.
Only the method named exactly `Props` is invoked; `UserListProps`-style helpers are ordinary methods you
call yourself. A page can also omit `Props`, or have only `Props` and pick what to render with
`RenderComponent`.

### Handler methods (`ServeHTTP`)

| Signature | Writer | Use for |
|---|---|---|
| `ServeHTTP(w, r)` | direct | plain `http.Handler` |
| `ServeHTTP(w, r) error` | buffered | HTML actions: return a partial, an error or a redirect signal |
| `ServeHTTP(w, r, deps...)` | direct | JSON/API endpoints that own their status codes |
| `ServeHTTP(w, r, deps...) error` | buffered | HTML actions that need DI (`RenderTarget` is injectable too) |

The canonical HTMX form action mutates, then returns the refreshed partial:

```gsx
type addTodo struct{}

func (addTodo) ServeHTTP(w http.ResponseWriter, r *http.Request, s *store.Store) error {
    if err := s.Add(r.Context(), r.FormValue("text")); err != nil {
        return err
    }
    return structpages.RenderComponent(<TodoList todos={s.List(r.Context())}/>)
}
```

In a `.gsx` file an element literal is a Go expression; in a `.go` file call the generated function
(`TodoList(todos)`, or `index{}.TodoList(todos)` for a page component; page structs are stateless, so a
zero-value receiver works across pages).

### Errors, redirects and the writer

- **Buffered `ServeHTTP` forms:** never write `w` and then return an error; the buffer is discarded before
  `WithErrorHandler` runs. Return the error. For a status code, return a typed error
  (`ErrorWithStatus{...}`) that the error handler unwraps with `errors.As`.
- **`Props` gets the unbuffered writer.** Setting headers or cookies is fine. Anything written to the body
  is sent as is and the error handler appends to it, so return errors instead of writing. If `Props`
  writes a complete response itself, return `structpages.ErrSkipPageRender`; that sentinel is honoured
  only from `Props`.
- **Redirects:** in an HTMX app return a control-flow error such as `Redirect{To: url}` and let the error
  handler send `HX-Location` for HTMX requests and `303` otherwise; `http.Redirect` inside an HTMX request
  makes the XHR follow the 3xx and swap the wrong page into the target. Build the URL with
  `structpages.URLFor(r.Context(), Page{}, params)`, never a string literal.
- **JSON endpoints** use the no-error DI form and write JSON error bodies themselves (no `http.Error`).
- **Streaming (SSE):** flush with `http.NewResponseController(w)`; it works through the buffered writer.

Worked versions of all of these, including the `WithErrorHandler` body: examples.md §13.

## 3. URLs

The recommended shape is `page |> url(params)` in markup and `structpages.URLFor(ctx, page, params)` in
Go, with `params` a `map[string]any` that fills both path and query placeholders.

| Page form | gsx | Use when |
|---|---|---|
| typed page | `{Detail{} \|> url(map[string]any{"itemId": id})}` | the type is mounted once |
| typed chain | `{[]any{components{}, entry{}} \|> url(params)}` | the same type is mounted under several parents |
| chain + fragment | `{[]any{list{}, "?page={page}&q={q}"} \|> url(params)}` | appending a query template |
| `Ref` / string | `{structpages.Ref("Admin.Settings") \|> url}` | the type cannot be imported (package cycle) |

- **Strict lookup:** a bare type mounted more than once is an error listing the matches; disambiguate with
  the chain. Leading typed values in `[]any` are chain steps; once a string appears, everything after it is
  a literal URL fragment.
- **Page groups resolve to their index:** `Section{} |> url` gives `/section/` (the `/{$}` child), which
  serves 200 directly. Don't hand-append slashes.
- **Auto-fill:** unfilled placeholders that are path params of the *current request's* route are filled
  from the request.
- A domain helper that chooses between pages may return `(string, error)` and sit in the hole directly:
  `action={postFormAction(ctx, post)}`.
- In Go code (Props, handlers, middleware) call `structpages.URLFor(r.Context(), ...)` and handle the
  error. Outside a request use `sp.URLFor(...)` on the `*StructPages` returned by `Mount`/`Parse`; it
  cannot auto-fill request params.

Never write an in-app URL as a literal; `structpages-lint` (reference.md §Lint) checks `URLFor`/`Ref`
calls and route literals in CI, and examples.md §14 adds a boot-time check for URLs it cannot see.

## 4. HTMX partials: one reference, three sites

One method or function reference drives three sites that must agree:

1. **Composition** — wrap the region: `<div id={Index.UserList |> id}>…</div>`.
2. **Trigger** — `hx-get={Index{} |> url}` (the page's own route) with `hx-target={Index.UserList |> target}`.
3. **Server** — `HX-Target` is matched back to the component; `Props` branches with `sel.Is(p.UserList)`.

```gsx
component (p Index) Content(props indexProps) {
    <input
        name="q"
        hx-get={Index{} |> url}
        hx-target={Index.UserList |> target}
        hx-trigger="input changed delay:300ms"
    />
    <div id={Index.UserList |> id}>
        <p.UserList pane={props.UserPane}/>
    </div>
}

func (p Index) Props(r *http.Request, s *store.Store, sel structpages.RenderTarget) (indexProps, error) {
    if sel.Is(p.UserList) {
        pane, err := p.userPane(r, s)
        if err != nil {
            return indexProps{}, err
        }
        return indexProps{}, structpages.RenderComponent(p.UserList(pane))
    }
    return p.fullProps(r, s)
}
```

Renaming the method or moving the mount cannot desynchronise the sites because none holds a string id.
Never hand-write the id at one site and generate it at another.

**Id format.** A page component's id is the page's field-name path from the root plus the method, kebab
cased: `index-user-list`, or `admin-users-user-list` when nested. Over the length budget (default 40,
`WithMaxIDLength`) it degrades to the leaf form (`user-list`) plus a stable hash when the leaf is shared.
A standalone component is prefixed by its Go package (`dashboard-stats-widget`), so same-named components
in different packages never collide. `target` prepends `#`. Plain strings pass through both unchanged:
`"body" |> target` is `body`.

**Mounts.** Inside a page's own render, `p.X |> id` uses the current mount, so one struct mounted twice
yields different ids per mount. From outside, a type mounted twice is ambiguous and errors; disambiguate
with a chain (`[]any{adminRoot{}, dashboard.Header}`), `structpages.Ref("AdminDash.Header")`, or make the
slot a standalone component.

**htmx 4.** `HTMXRenderTarget` reads htmx 1/2 headers. With htmx 4 (`HX-Target: tag#id`,
`HX-Request-Type`), mount with `structpages.WithTargetSelector(structpages.HTMXv4RenderTarget)`.

### JavaScript and interpolated attributes

Filters also work inside the `@{}` holes of `js` and `f` literals on an attribute; the error still
propagates from `Render`:

```gsx
<button @click=js`openIn(@{Index.Canvas |> target})`>Open</button>
<button hx-vals=js`{"pane": @{p.Detail |> id}}`>Load</button>
<tr hx-target=f`closest @{Index.Row |> target}`>…</tr>
```

Keep the literal on the native element that consumes it. A wrapper component takes the plain value
(`slot={Index.Canvas |> target}`) and builds the literal on its own element. A literal assigned inside a
`{{ }}` block has no error channel, so gsx rejects error-returning filters there. Never splice a generated
value in with `gsx.RawJS`.

## 5. RenderTarget and RenderComponent

For pages with several independently updated regions:

- **Partials get partial data**, not the page props struct. Each branch builds just its region's data and
  returns the constructed component; the props value returned alongside is ignored.
- **The page props struct composes per-region structs** (`indexProps{UserPane, GroupPane}`) so full render
  and partial render share one component signature.
- **The default branch loads full props**, never empty props: browser navigation, boosted swaps and
  unrecognised targets all need the whole page.

```go
func (p Index) Props(r *http.Request, s *store.Store, sel structpages.RenderTarget) (indexProps, error) {
    switch {
    case sel.Is(p.UserList):
        pane, err := p.userPane(r, s)
        if err != nil {
            return indexProps{}, err
        }
        return indexProps{}, structpages.RenderComponent(p.UserList(pane))
    case sel.Is(StatsWidget): // standalone component
        return indexProps{}, structpages.RenderComponent(StatsWidget(loadStats()))
    default:
        return p.fullProps(r, s)
    }
}
```

`RenderComponent` forms, preferred first:

1. **Constructed component** — `RenderComponent(p.UserList(pane))`, `RenderComponent(<StatsWidget s={s}/>)`,
   `RenderComponent(other{}.Row(row))`. Compile-time checked.
2. **Method expression** — `RenderComponent(Index.ItemList)` or `RenderComponent(Index.ItemList, items)`:
   the framework finds the mounted page and DI-injects parameters you don't pass. Use only when the
   component's parameters should be injected; arguments are checked at runtime.
3. **Via target** — `RenderComponent(sel, args...)` after `sel.Is(fn)` matched. Required for a target from
   a custom selector whose function you don't know statically; `Is` stores the function on match.

### Nested swap levels

Give each independently swappable region its own page component, outer wrapping inner:

- `Page` — full document (layout around `Content`). Cold loads and body swaps.
- `Content` — the page's main region, with its chrome (heading, back link, toolbar).
- `Detail` (or another name) — an inner region that swaps on its own and has **no** chrome.

```gsx
component (d fooDetail) Page(p fooProps) {
    <Layout><d.Content p={p}/></Layout>
}

component (d fooDetail) Content(p fooProps) {
    <a href={fooList{} |> url}>&larr; Foos</a>
    <div id={fooDetail.Detail |> id}><d.Detail p={p}/></div>
}

component (d fooDetail) Detail(p fooProps) {
    <dl>…fields, actions…</dl>
}
```

A master-detail list renders a mount with `id={fooDetail.Detail |> id}`; rows `hx-get` the detail route
targeting `fooDetail.Detail |> target`, and actions on the detail re-render `Detail`, never `Content`.
Embed or target the innermost level that has no chrome above it.

## 6. Middleware

```go
type MiddlewareFunc func(http.Handler, *structpages.PageNode) http.Handler
```

Global: `structpages.WithMiddlewares(a, b)` (first is outermost). Per subtree: a `Middlewares` method,
which can take DI arguments and applies to the page and all descendants:

```go
func (adminPages) Middlewares(auth *Auth) []structpages.MiddlewareFunc {
    return []structpages.MiddlewareFunc{auth.Require}
}
```

Middleware runs outside the error-return path, so it handles HTMX itself (set `HX-Location` rather than
sending a 3xx). See examples.md §7.

## 7. Dependency injection

```go
sp, err := structpages.Mount(mux, pages{}, "/", "App",
    structpages.WithArgs(store, logger),
    structpages.WithErrorHandler(errorHandler),
)
```

Registered values are matched by type into `Props`, `ServeHTTP`, `Middlewares`, `Init` and DI-injected
components. Each type may be registered once (use named types to register two values of one type).
Pointer and value forms coerce, and interface parameters are filled by any registered value that
implements them. `*PageNode` is always injectable. Application services go here, not in globals.

## 8. Testing renders with a bare context

`URLFor`, `ID`, `IDTarget` (and so the filters) need the page tree in the context. In unit tests build it
without a mux:

```go
sp, err := structpages.Parse(pages{}, "/", "App", structpages.WithArgs(fakeStore))
if err != nil {
    t.Fatal(err)
}
ctx := sp.PageContext(context.Background())

var buf bytes.Buffer
if err := (itemPage{}).Page(props).Render(ctx, &buf); err != nil {
    t.Fatal(err)
}
```

Parse the canonical root even when the test exercises one module, so URLs to sibling modules resolve.
`structpages.CurrentPage(ctx)` is nil under `PageContext`; it is set only while serving a `Props`/component
page (not a `ServeHTTP` page).

## Key rules

1. Read path params with `r.PathValue("itemId")`.
2. Never hand-build an in-app URL or a partial's id: use `url`, `id`, `target` (or `URLFor`/`ID`/`IDTarget`
   in Go) from page types and method references.
3. No `must()`, no `urlFor`/`idFor` wrappers, no `gsx.RawJS` around generated values: holes and filters
   take `(T, error)` directly.
4. Partials take only their region's data; the default `Props` branch returns full props.
5. Prefer constructed components in `RenderComponent`; method expressions only for DI-injected parameters.
6. `RenderComponent` is returned as an error; other return values are then ignored.
7. Don't write the body in `Props` or a buffered `ServeHTTP`; return errors. `ErrSkipPageRender` only from
   `Props`.
8. Strict lookups: disambiguate repeated mounts with a `[]any` chain; `Ref` only across import cycles.
9. Plain strings pass through `id`/`target` unchanged (`"body" |> target` is `body`).
10. htmx 4 needs `WithTargetSelector(HTMXv4RenderTarget)`.
