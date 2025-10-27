# URLFor Functionality

Generate type-safe URLs for your pages:

### Setup for Templ Templates

First, create a wrapper function for use in templ files:

```go
// urlFor wraps structpages.URLFor for templ templates
func urlFor(ctx context.Context, page any, args ...any) (templ.SafeURL, error) {
    url, err := structpages.URLFor(ctx, page, args...)
    return templ.URL(url), err
}
```

### Basic Usage

```templ
// Simple page references without parameters
<a href={ urlFor(ctx, index{}) }>Home</a>
<a href={ urlFor(ctx, product{}) }>Products</a>
<a href={ urlFor(ctx, team{}) }>Our Team</a>
```

### With Path Parameters

```go
// Route definition
type pages struct {
    userProfile `route:"/users/{id} User Profile"`
    blogPost    `route:"/blog/{year}/{month}/{slug} Blog Post"`
}

// In Go code (e.g., in handlers or middleware)
url, err := structpages.URLFor(ctx, userProfile{}, "123")
// Returns: /users/123
```

```templ
// Single parameter - positional
<a href={ urlFor(ctx, userProfile{}, "123") }>View User</a>

// Multiple parameters - as key-value pairs
<a href={ urlFor(ctx, blogPost{}, "year", "2024", "month", "06", "slug", "my-post") }>
    Read Post
</a>

// Using a map
<a href={ urlFor(ctx, blogPost{}, map[string]any{
    "year": "2024",
    "month": "06",
    "slug": "my-post",
}) }>Read Post</a>
```

### With Query Parameters

Use the `join` helper to add query parameters:

```go
// Helper function
func join(page any, pattern string) []any {
    return []any{page, pattern}
}
```

```templ
// Add query parameters with template placeholders
<a href={ urlFor(ctx, join(product{}, "?page={page}"), "page", "2") }>
    Page 2
</a>

// Multiple query parameters
<form hx-post={ urlFor(ctx, join(toggle{}, "?redirect={url}"), 
    "id", todoId, 
    "url", currentURL) }>
    <button>Toggle</button>
</form>

// Complex example with path and query parameters
<a href={ urlFor(ctx, join(jobDetail{}, "?tab={tab}"), 
    "id", jobId, 
    "tab", "overview") }>
    Job Overview
</a>
```

### Automatic URL Parameter Extraction

When calling `URLFor` within a handler, URL parameters from the current request are automatically available and will be used to fill matching parameters in the generated URL. This is particularly useful when generating URLs for related pages that share the same parameters.

```go
// Route definitions
type pages struct {
    viewProduct `route:"GET /product/{id} View Product"`
    editProduct `route:"GET /product/{id}/edit Edit Product"`
}

// In the view handler
func (v viewProduct) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Generate edit URL - the {id} parameter is automatically extracted from current request
    editURL, _ := structpages.URLFor(r.Context(), editProduct{})
    // If current URL is /product/123, editURL will be "/product/123/edit"
    
    // You can still override extracted parameters if needed
    differentURL, _ := structpages.URLFor(r.Context(), editProduct{}, "456")
    // differentURL will be "/product/456/edit"
}
```

This feature works with multiple parameters as well:

```go
type pages struct {
    viewPost `route:"GET /blog/{year}/{month}/{slug} View Post"`
    editPost `route:"GET /blog/{year}/{month}/{slug}/edit Edit Post"`
}

// In a template
templ (v viewPost) Page() {
    // All parameters (year, month, slug) are automatically available
    <a href={ urlFor(ctx, editPost{}) }>Edit this post</a>
    
    // Override just one parameter while keeping others
    <a href={ urlFor(ctx, viewPost{}, map[string]any{"slug": "different-post"}) }>
        View different post in same month
    </a>
}
```

This automatic extraction eliminates the need to manually pass parameters that are already present in the current request context, making URL generation more convenient and less error-prone.

### IDFor - Consistent HTML IDs

The `IDFor` function generates consistent HTML IDs from component method references, helping maintain consistency between template IDs, HTMX targets, and component names.

#### The Problem

When building HTMX applications, you often need to keep three things in sync:
1. The ID attribute in your component template
2. The HTMX target selector in your requests
3. The component method name

```templ
// If you change "UserList" to "UserTable", you need to manually update:
<div id="user-list">...</div>  // Manual ID
<button hx-target="#user-list">Refresh</button>  // Manual target reference
templ (p TeamManagementView) UserList(users []User) { ... }  // Component name
```

#### The Solution

Use `IDFor` with method expressions to generate IDs automatically:

```templ
// In your component template
templ (p TeamManagementView) UserList(users []User) {
    <div id={ structpages.IDFor(p.UserList) }>
        <!-- content -->
    </div>
}

// In HTMX attributes
@PrimaryButton(templ.Attributes{
    "hx-get":    "/api/users",
    "hx-target": "#" + structpages.IDFor(TeamManagementView{}.UserList),
})
```

Now when you rename `UserList` to `UserTable` using your IDE's refactoring tools, all references including `p.UserList` and `TeamManagementView{}.UserList` will be automatically updated!

#### With Suffixes

For compound IDs like `user-modal-container` or `group-search-input`:

```templ
// Generate "user-modal-container"
<div id={ structpages.IDFor(p.UserModal, "container") }>
    <!-- modal content -->
</div>

// Generate "group-search-input"
<input
    id={ structpages.IDFor(p.GroupSearch, "input") }
    name="search"
/>
```

#### Naming Convention

`IDFor` converts CamelCase/PascalCase to kebab-case:

- `IDFor(p.UserList)` → `user-list`
- `IDFor(p.GroupMembers)` → `group-members`
- `IDFor(p.HTMLParser)` → `html-parser`
- `IDFor(p.UserModal, "container")` → `user-modal-container`

See [IDFOR_USAGE.md](./IDFOR_USAGE.md) for more detailed examples and usage patterns.

