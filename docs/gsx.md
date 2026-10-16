---
title: gsx Patterns
slug: /gsx
sidebar_position: 6
---

# gsx Patterns

structpages renders any value with `Render(context.Context, io.Writer) error`, so it works with any renderer. [gsx](https://github.com/gsxhq/gsx) is the primary path and what every example in the repository uses. templ and `html/template` also work; see [Using templ](./templ.md).

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

The injectable types are: `*http.Request`, `http.ResponseWriter`, `structpages.RenderTarget`, `*structpages.PageNode`, and any type registered via `WithArgs`.

## Partials load only their data

When a page has independently-updatable regions, inject `RenderTarget` and branch with `target.Is`. **Construct the component and hand it to `RenderComponent`** — a normal Go call the compiler checks:

```go
type dashboardPage struct{}

func (d dashboardPage) Props(r *http.Request, target structpages.RenderTarget, store *Store) (DashboardData, error) {
    if target.Is(d.StatsWidget) {
        stats, err := store.GetStats(r.Context())
        if err != nil {
            return DashboardData{}, err
        }
        return DashboardData{}, structpages.RenderComponent(d.StatsWidget(stats))
    }
    // Full page — load everything
    user, err := store.GetUser(r)
    if err != nil {
        return DashboardData{}, err
    }
    stats, err := store.GetStats(r.Context())
    if err != nil {
        return DashboardData{}, err
    }
    return DashboardData{User: user, Stats: stats}, nil
}
```

Partial page components take ONLY their specific data (`StatsWidget(stats Stats)`), not the full props struct. The full pattern, including how `HX-Target` selects the partial, is in [HTMX Integration](./htmx.md).

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

## Testing renders with a bare context

Unit tests that render components directly — without an HTTP server — need a page tree in the context so the `url` / `id` / `target` filters (`URLFor` / `ID` / `IDTarget`) resolve. Use `structpages.Parse` (builds the tree, no mux) and `sp.PageContext`:

```go
func TestProductPageRenders(t *testing.T) {
    sp, err := structpages.Parse(pages{}, "/", "App",
        structpages.WithArgs(fakeStore),
    )
    if err != nil {
        t.Fatal(err)
    }
    ctx := sp.PageContext(context.Background())

    var buf bytes.Buffer
    props := productPageProps{Product: sampleProduct}
    if err := (productPage{}).Page(props).Render(ctx, &buf); err != nil {
        t.Fatal(err)
    }
    if !strings.Contains(buf.String(), sampleProduct.Name) {
        t.Errorf("rendered page missing product name")
    }
}
```

`Parse` accepts the same options as `Mount`; mux-shaped options (middlewares) are accepted but inert since no handlers register.
