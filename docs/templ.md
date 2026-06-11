---
title: Templ Patterns
slug: /templ
sidebar_position: 6
---

# Templ Patterns

## Page components and layout composition

A **page component** is a templ method on a page struct. A **component** is a standalone templ block. Layouts are nothing special — just a component that takes `{ children... }`:

```templ
type homePage struct{}

templ (h homePage) Page() {
    @layout() {
        <h1>Welcome Home</h1>
        <p>This is the home page content.</p>
    }
}

// Shared layout — a plain component with children
templ layout() {
    <!DOCTYPE html>
    <html>
        <head>
            <title>My App</title>
        </head>
        <body>
            { children... }
        </body>
    </html>
}
```

## The Props pattern

The **Props method** loads data; the **props struct** it returns flows into the page components:

```go
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

templ (p productPage) Page(props productPageProps) {
    @layout() {
        <h1>{ props.Product.Name }</h1>
        <p>{ props.Product.Description }</p>
        if props.IsInStock {
            <button>Add to Cart</button>
        } else {
            <span>Out of Stock</span>
        }
        @relatedProductsList(props.RelatedProducts)
    }
}
```

Never write the response body or call `http.Error` inside Props — it runs against a buffered writer and a returned error discards the buffer. Return the error and let the global handler render it (see [Error Handling](./error-handling.md)).

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

The reflective method-expression form — `RenderComponent(index.TodoList)` with no constructed component — exists for page components whose parameters the framework should DI-inject rather than you supplying them. Prefer direct construction whenever you're loading the data yourself anyway.

## Testing renders with a bare context

Unit tests that render templ components directly — without an HTTP server — need a page tree in the context so `URLFor` / `ID` / `IDTarget` resolve. Use `structpages.Parse` (builds the tree, no mux) and `sp.PageContext`:

```go
func TestProductPageRenders(t *testing.T) {
    sp, err := structpages.Parse(pages{}, "/", "App",
        structpages.WithArgs(fakeStore),
    )
    if err != nil {
        t.Fatal(err)
    }
    ctx := sp.PageContext(context.Background())

    buf := &bytes.Buffer{}
    props := productPageProps{Product: sampleProduct}
    if err := (productPage{}).Page(props).Render(ctx, buf); err != nil {
        t.Fatal(err)
    }
    if !strings.Contains(buf.String(), sampleProduct.Name) {
        t.Errorf("rendered page missing product name")
    }
}
```

`Parse` accepts the same options as `Mount`; mux-shaped options (middlewares) are accepted but inert since no handlers register.
