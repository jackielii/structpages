---
title: gsx Patterns
slug: /gsx
sidebar_position: 6
---

# gsx Patterns

structpages renders any value with `Render(context.Context, io.Writer) error`, so it works with any renderer. [gsx](https://github.com/gsxhq/gsx) is the primary path and what every example in the repository uses. templ and `html/template` also work; see [templ and html/template](./templ.md).

## Register the filters

structpages exposes `URLFor`, `ID` and `IDTarget` as ordinary functions with a leading `ctx`. Register them once as gsx pipeline filters in the project's `gsx.toml` (gsx uses the nearest one walking up to the repository root):

```toml
[filters]
url    = "github.com/jackielii/structpages.URLFor"
id     = "github.com/jackielii/structpages.ID"
target = "github.com/jackielii/structpages.IDTarget"
```

gsx passes the render `ctx` and the piped value as the first two arguments and returns the filter's error from `Render`. So `{ page |> url(params) }` is `structpages.URLFor(ctx, page, params)`, `{ X |> id }` is `structpages.ID(ctx, X)` and `{ X |> target }` is `structpages.IDTarget(ctx, X)`. Holes accept `(T, error)` directly, so don't write a `must()` helper or `urlFor`/`idFor` wrappers.

## Page components and layout composition

A **page component** is a method component on a page struct. A **component** is a standalone `component`. Layouts are nothing special — just a component that takes `children gsx.Node`:

```gsx
type homePage struct{}

component (h homePage) Page() {
    <Layout title="Home">
        <h1>Welcome Home</h1>
        <p>This is the home page content.</p>
    </Layout>
}

// Shared layout — a plain component with children
component Layout(title string, children gsx.Node) {
    <!DOCTYPE html>
    <html>
        <head>
            <title>{ title }</title>
        </head>
        <body>
            { children }
        </body>
    </html>
}
```

## The Props pattern

The **Props method** loads data; the **props struct** it returns flows into the page components:

```gsx
type productPage struct{}

type productPageProps struct {
    Product         Product
    RelatedProducts []Product
    IsInStock       bool
}

// Parameters are matched by type via DI. http.ResponseWriter is injectable
// too — useful for setting headers or cookies before the render.
func (p productPage) Props(r *http.Request, w http.ResponseWriter, store *Store) (productPageProps, error) {
    productID := r.PathValue("productId")
    product, err := store.LoadProduct(productID)
    if err != nil {
        return productPageProps{}, err
    }

    w.Header().Set("X-Product-ID", productID)

    related, err := store.LoadRelatedProducts(productID)
    if err != nil {
        return productPageProps{}, err
    }

    return productPageProps{
        Product:         product,
        RelatedProducts: related,
        IsInStock:       product.Stock > 0,
    }, nil
}

component (p productPage) Page(props productPageProps) {
    <Layout title={props.Product.Name}>
        <h1>{ props.Product.Name }</h1>
        <p>{ props.Product.Description }</p>
        { if props.IsInStock {
            <button>Add to Cart</button>
        } else {
            <span>Out of Stock</span>
        } }
        <RelatedProductsList products={props.RelatedProducts}/>
    </Layout>
}
```

Never write the response body or call `http.Error` inside Props. Props gets the unbuffered writer, so a body write is sent as is and the error page is appended to it. Return the error and let the global handler render it (see [Error Handling](./error-handling.md)).

### Props method resolution

Only the method literally named `Props` is auto-invoked by the framework. Methods whose names *end* in `Props` (e.g. `UserListProps`, `ContentProps`) are **not** auto-resolved — they are conventional helpers you call yourself from inside `Props`.

**Parameter resolution is by type, not position.** All of these signatures work:

```go
func (d dashboardPage) Props(r *http.Request, store *Store) (DashboardData, error)
func (d dashboardPage) Props(store *Store, r *http.Request) (DashboardData, error)  // any order
func (d dashboardPage) Props(r *http.Request, w http.ResponseWriter, store *Store) (DashboardData, error)
func (d dashboardPage) Props(r *http.Request, target structpages.RenderTarget, store *Store) (DashboardData, error)
```

The injectable types are: `*http.Request`, `http.ResponseWriter`, `structpages.RenderTarget`, `*structpages.PageNode`, any type a [request arg](./dependency-injection.md#request-args) provides, and any type registered via `WithArgs`. Page components can take the same values, after Props' results.

## Partials load only their data

When a page has independently-updatable regions, inject `RenderTarget` and ask `target.Is` what the request wants. Return each region's data as its own value: when Props returns no error, the framework renders the page component the target `Is` (or `Page`), filling its parameters by type from Props' results.

```go
type dashboardPage struct{}

func (d dashboardPage) Props(r *http.Request, target structpages.RenderTarget, store *Store) (User, Stats, error) {
    if target.Is(d.StatsWidget) {
        stats, err := store.GetStats(r.Context())
        return User{}, stats, err // renders StatsWidget(stats)
    }
    // Full page — load everything
    user, err := store.GetUser(r)
    if err != nil {
        return User{}, Stats{}, err
    }
    stats, err := store.GetStats(r.Context())
    return user, stats, err // renders Page(user, stats)
}
```

```gsx
component (d dashboardPage) Page(user User, stats Stats) {
    <Layout title="Dashboard">
        <h1>Hello, { user.Name }</h1>
        <div id={dashboardPage.StatsWidget |> id}>
            <d.StatsWidget stats={stats}/>
        </div>
    </Layout>
}

component (d dashboardPage) StatsWidget(stats Stats) { … }
```

Partial page components take ONLY their specific data (`StatsWidget(stats Stats)`), not the full props struct. Each result fills at most one parameter, so give each share its own type. To render something other than what the request wants, Props returns `RenderComponent(d.StatsUnavailable(err))` — a constructed component, a normal Go call the compiler checks. The htmx side, including the `WithTargetSelector` option that makes `HX-Target` select the partial, is in [HTMX Integration](./htmx.md); the full flow is in [How a Request Is Rendered](./rendering.md).

## Cross-page rendering

A handler method that needs to respond with *another* page's component constructs it the same way — page structs are stateless, so a zero-value receiver works:

```go
func (a addTodo) ServeHTTP(w http.ResponseWriter, r *http.Request, store *Store) error {
    if err := store.Add(r.Context(), r.FormValue("text")); err != nil {
        return err
    }
    todos, err := store.List(r.Context())
    if err != nil {
        return err
    }
    return structpages.RenderComponent(index{}.TodoList(todos))
}
```

For a standalone component, a handler written in a `.gsx` file can return markup directly, since an element literal is a Go expression there: `structpages.RenderComponent(<TodoList todos={todos}/>)`. In a `.go` file call the generated function: `TodoList(todos)`.

## JavaScript and interpolated attributes

Filters also work inside the `@{}` holes of `js` and `f` literals on an attribute, so ids, selectors and URLs reach JavaScript, Alpine and htmx values the same way; the error still propagates from `Render`:

```gsx
component (p Index) Canvas() {
    <div
        x-data="{ open: false }"
        @keydown.escape=js`htmx.ajax('GET', @{Index{} |> url}, @{Index.Canvas |> target})`
    >
        <button
            hx-get={Index{} |> url}
            hx-target=f`closest @{Index.PickerList |> target}`
            hx-vals=js`{"pane": @{Index.Canvas |> id}}`
        >Reload</button>
    </div>
}
```

Keep the literal on the native element that consumes it. A wrapper component takes plain values and builds the literal on its own element:

```gsx
component OpenButton(slot string, href string, children gsx.Node) {
    <button @click=js`htmx.ajax('GET', @{href}, @{slot})`>{ children }</button>
}

component (p Index) Toolbar() {
    <OpenButton slot={Index.Canvas |> target} href={Detail{} |> url}>Open</OpenButton>
}
```

A literal assigned inside a `{{ }}` block can't use these filters ([why](./urlfor.md#no-must-helpers)). Never splice a generated value in with `gsx.RawJS`.

## Testing

Rendering components in unit tests with `Parse` + `PageContext`, and requests through `Mount`, are covered in [Testing](./testing.md).
