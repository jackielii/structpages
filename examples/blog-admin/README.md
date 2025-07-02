# Blog Admin Example

This is a comprehensive example demonstrating advanced features of the structpages framework. It implements a full-featured blog platform with public-facing pages and an admin panel.

## Features Demonstrated

### 1. **Dependency Injection**
- Multiple services injected via `MountPages`: Store, AuthService, SessionManager, FormDecoder, Config
- Type-based injection with automatic parameter resolution
- Services available in Props methods, ServeHTTP handlers, and middleware

### 2. **Nested Routing Structure**
- Three-level deep routes (e.g., `/admin/posts/{id}/edit`)
- Struct embedding for hierarchical organization
- Clean URL patterns with path parameters

### 3. **Authentication & Authorization**
- Session-based authentication with bcrypt password hashing
- Role-based access control (admin, author, reader)
- Protected routes using middleware composition
- Login/logout flow with redirect support

### 4. **Advanced Middleware Patterns**
- Global middleware (session loading, logging)
- Per-route middleware (authentication, CSRF protection)
- Middleware composition with proper execution order
- Context-aware middleware with PageNode access

### 5. **Props Pattern**
- Type-safe props with complex data loading
- Dependency injection in Props methods
- Error handling and data validation
- Efficient database queries with relationship loading

### 6. **HTMX Integration**
- Partial component rendering
- Custom PageConfig for different HX-Target values
- Progressive enhancement patterns
- Real-time updates (publish/unpublish)
- Auto-save functionality
- Form submissions with URL updates

### 7. **Form Handling**
- Structured form parsing with go-playground/form
- Multi-value form fields (categories, tags)
- File upload handling (media library)
- Validation and error display

### 8. **Database Integration**
- SQLite with proper schema and indexes
- Transaction support for complex operations
- Efficient queries with pagination
- Analytics and view tracking

### 9. **Component Composition**
- Reusable Templ components (layouts, cards, forms)
- Conditional rendering based on user roles
- Dynamic component selection
- Shared UI components library

### 10. **URL Generation**
- Type-safe URL generation with `urlFor`
- Support for path parameters
- Query string building with `join` helper
- HTMX-aware URL handling

### 11. **Error Handling**
- Custom error handler with status codes
- Graceful degradation
- User-friendly error messages
- Proper HTTP status responses

### 12. **Advanced ServeHTTP Patterns**
- Error-returning ServeHTTP (buffered response)
- Standard http.Handler interface (direct write)
- API endpoints with JSON responses
- Mixed content types handling

## Project Structure

```
blog-admin/
├── main.go              # Application entry point
├── routes.go            # Route definitions and middleware
├── models.go            # Data models and database schema
├── store.go             # Database operations
├── auth.go              # Authentication service
├── components.templ     # Reusable UI components
├── pages.templ          # Public pages (home, post, search, login)
├── admin_pages.templ    # Admin dashboard and post management
├── admin_users.templ    # User management and settings
├── api_pages.templ      # API endpoints and advanced patterns
└── static/              # CSS and static assets
    ├── styles.css       # Public site styles
    └── admin.css        # Admin panel styles
```

## Running the Example

1. Install dependencies:
```bash
go mod download
```

2. Generate Templ files:
```bash
templ generate
```

3. Run the application:
```bash
go run .
```

4. Access the application:
- Public site: http://localhost:8080
- Admin panel: http://localhost:8080/admin
- Login credentials: admin / admin123

## Key Patterns to Study

### Dependency Injection
See how services are registered in `main.go` and used throughout the application:
```go
if err := sp.MountPages(r, pages{}, "/", "Blog", store, auth, sessionManager, formDecoder, config); err != nil {
    log.Fatal(err)
}
```

### Nested Routes with Middleware
Check `routes.go` for the hierarchical structure and middleware application:
```go
type adminPages struct {
    dashboard `route:"/{$} Dashboard"`
    posts     adminPostPages `route:"/posts Posts"`
}

func (a adminPages) Middlewares(auth *AuthService) []structpages.MiddlewareFunc {
    return []structpages.MiddlewareFunc{
        requireAuthMiddleware(auth),
        requireAdminMiddleware(auth),
    }
}
```

### Props Pattern with Data Loading
See `pages.templ` for examples of loading complex data:
```go
func (p postPage) Props(r *http.Request, store *Store, auth *AuthService) (postPageProps, error) {
    slug := r.PathValue("slug")
    post, err := store.GetPostBySlug(slug)
    // ... load related data
}
```

### HTMX Partial Rendering
Check `api_pages.templ` for custom PageConfig:
```go
func (s searchPage) PageConfig(r *http.Request) (string, error) {
    hxTarget := r.Header.Get("HX-Target")
    switch hxTarget {
    case "search-results":
        return "Results", nil
    default:
        return "Page", nil
    }
}
```

This example showcases how structpages can be used to build production-ready applications with minimal boilerplate while maintaining type safety and clean architecture.