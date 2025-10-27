# Routing Patterns and Struct Tags

### Basic Route Definition

Routes are defined using struct tags with the `route:` prefix. Each struct field with a route tag becomes a route in your application.

```go
type pages struct {
    home    `route:"/ Home"`           // ALL / with title "Home"
    about   `route:"/about About Us"`  // ALL /about with title "About Us"
    contact `route:"/contact"`         // ALL /contact without title
}
```

### Route Tag Format

The route tag supports several formats:

1. **Path only**: `route:"/path"`
   - Matches all HTTP methods
   - No page title

2. **Path with title**: `route:"/path Page Title"`
   - Matches all HTTP methods
   - Sets page title to "Page Title"

3. **Method and path**: `route:"POST /path"`
   - Matches only specified HTTP method
   - No page title

4. **Full format**: `route:"PUT /path Update Page"`
   - Matches only PUT requests
   - Sets page title to "Update Page"

Supported HTTP methods: `GET`, `HEAD`, `POST`, `PUT`, `PATCH`, `DELETE`, `CONNECT`, `OPTIONS`, `TRACE`. A special `ALL` method can be used to match all methods.

### Path Parameters

Path parameters use Go 1.22+ `http.ServeMux` syntax:

```go
type pages struct {
    userProfile `route:"/users/{id} User Profile"`
    blogPost    `route:"/blog/{year}/{month}/{slug}"`
}

// Access parameters in your Props method:
func (p userProfile) Props(r *http.Request) (UserProfileProps, error) {
    userID := r.PathValue("id") // "123" if URL is /users/123
    // Pass the userID via the props to the Page renderer
    return UserProfileProps{UserID: userID}, nil
}

templ (p userProfile) Page(props UserProfileProps) {
    @layout() {
        <h1>User Profile for { props.UserID }</h1>
        // Render user details
    }
}
```

### Nested Routes

Create hierarchical URL structures by nesting structs:

```go
type pages struct {
    admin adminPages `route:"/admin Admin Panel"`
}

type adminPages struct {
    dashboard `route:"/ Dashboard"`        // Becomes /admin/
    users     `route:"/users User List"`   // Becomes /admin/users
    settings  `route:"/settings Settings"` // Becomes /admin/settings
}
```

