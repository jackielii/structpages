# Templ Patterns

### Basic Page Pattern

```templ
// Define your page struct
type homePage struct{}

// Implement the Page method returning a templ component
templ (h homePage) Page() {
    @layout() {
        <h1>Welcome Home</h1>
        <p>This is the home page content.</p>
    }
}

// Shared layout component
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

### Props Pattern

Pass data to your components using typed Props:

```go
type productPage struct{}

// Define typed props for better type safety
type productPageProps struct {
    Product Product
    RelatedProducts []Product
    IsInStock bool
}

// Props method returns typed props and can receive injected dependencies
// You can also include http.ResponseWriter to set headers, cookies, etc.
func (p productPage) Props(r *http.Request, w http.ResponseWriter, store *Store) (productPageProps, error) {
    productID := r.PathValue("id")
    product, err := store.LoadProduct(productID)
    if err != nil {
        return productPageProps{}, err
    }
    
    // You can manipulate the response if needed
    w.Header().Set("X-Product-ID", productID)
    
    related, _ := store.LoadRelatedProducts(productID)
    
    return productPageProps{
        Product: product,
        RelatedProducts: related,
        IsInStock: product.Stock > 0,
    }, nil
}

// Page method receives typed props
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

#### Props Method Resolution

Only the method literally named `Props` is auto-invoked by the framework. Methods whose names *end* in `Props` (e.g. `UserListProps`, `PageProps`, `ContentProps`) are stored in the page node but **not** auto-resolved — they are conventional helpers you call yourself from inside `Props`. The earlier per-component-Props auto-resolution (`PageProps()`, `ContentProps()`, etc.) was removed in favor of the simpler `Props` + `RenderComponent` pattern below.

**Parameter resolution is by type, not position.** All of these signatures work and the framework matches each parameter by its type:

```go
func (d dashboardPage) Props(r *http.Request, store *Store) (DashboardData, error) { ... }
func (d dashboardPage) Props(store *Store, r *http.Request) (DashboardData, error) { ... }   // any order
func (d dashboardPage) Props(r *http.Request, w http.ResponseWriter, store *Store) (DashboardData, error) { ... }
func (d dashboardPage) Props(r *http.Request, target structpages.RenderTarget, store *Store) (DashboardData, error) { ... }
```

The injectable types are: `*http.Request`, `http.ResponseWriter`, `structpages.RenderTarget`, `*structpages.PageNode`, and any type registered via `WithArgs`. Position doesn't matter — the framework fills each parameter by looking up its type.

To run different data-loading paths for different components, use the `RenderTarget` parameter and `RenderComponent`:

```go
type dashboardPage struct{}

func (d dashboardPage) Props(r *http.Request, w http.ResponseWriter, target structpages.RenderTarget, store *Store) (DashboardData, error) {
    // Full page or full content — load everything
    if target.Is(d.Page) || target.Is(d.Content) {
        http.SetCookie(w, &http.Cookie{Name: "dashboard_visited", Value: "true"})
        return DashboardData{User: store.GetUser(r), Stats: store.GetStats()}, nil
    }
    // HTMX partial — render just the stats widget
    if target.Is(d.Content) {
        return DashboardData{}, structpages.RenderComponent(d.Content, ContentData{Stats: store.GetStats()})
    }
    return DashboardData{}, nil
}
```

### Cross-Page Component Rendering

Structpages provides the ability to render components from different pages using the `RenderComponent` function. This is useful when you want to conditionally redirect rendering to a component from another page, such as error pages or shared components.

#### RenderComponent

The `RenderComponent` function can be used in Props methods to render a component from a different page:

```go
type errorPage struct{}

templ (e errorPage) ErrorComponent(message string) {
    <div class="error">
        <h2>Error</h2>
        <p>{ message }</p>
    </div>
}

type productPage struct{}

func (p productPage) Props(r *http.Request, store *Store) (string, error) {
    productID := r.PathValue("id")
    product, err := store.LoadProduct(productID)
    if err != nil {
        // Instead of returning an error, render the ErrorComponent from errorPage
        return "", structpages.RenderComponent(errorPage{}.ErrorComponent, "Product not found")
    }
    return product.Name, nil
}

templ (p productPage) Page(productName string) {
    <h1>{ productName }</h1>
    <p>Product details...</p>
}
```

#### Parameters

- `targetOrMethod`: A method expression (e.g., `errorPage{}.ErrorComponent`) or RenderTarget
- `args`: Optional arguments to pass to the component method (these replace the original Props return values)

#### Behavior

When `RenderComponent` is returned as an error from a Props method:

1. The framework resolves the method expression to the component
2. Calls the component with the provided arguments
3. Renders the component instead of the original page's component

This pattern is particularly useful for:
- Error handling and displaying error pages
- Conditional rendering based on authentication or permissions
- Redirecting to maintenance or unavailable pages
- Sharing common components across different pages

