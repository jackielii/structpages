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

#### Props Method Resolution Rules

Structpages looks for Props methods in the following order:

1. **Component-specific Props method**: `<ComponentName>Props()` - e.g., `PageProps()`, `ContentProps()`, `SidebarProps()`
2. **Generic Props method**: `Props()` - used as a fallback if no component-specific method exists

**ResponseWriter Support**: Props methods can include `http.ResponseWriter` as a parameter to manipulate the response directly (e.g., setting cookies, custom headers). This must be the second parameter after `*http.Request`.

This allows you to have different props for different components:

```go
type dashboardPage struct{}

// Different props for different components
func (d dashboardPage) PageProps(r *http.Request, w http.ResponseWriter, store *Store) (PageData, error) {
    // Full page data including layout
    // Can set cookies or headers as needed
    http.SetCookie(w, &http.Cookie{Name: "dashboard_visited", Value: "true"})
    return PageData{User: store.GetUser(r), Stats: store.GetStats()}, nil
}

func (d dashboardPage) ContentProps(r *http.Request, store *Store) (ContentData, error) {
    // Just the content data for HTMX partial updates
    // Note: ResponseWriter is optional - only include if needed
    return ContentData{Stats: store.GetStats()}, nil
}

templ (d dashboardPage) Page(data PageData) {
    // Full page render
}

templ (d dashboardPage) Content(data ContentData) {
    // Partial content render
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

