# Structpages Examples

This directory contains examples demonstrating various features of the structpages library.

## Available Examples

### 1. Simple
Basic routing and page structure demonstration.
- Simple struct-based routing
- Basic templ integration
- URL generation with `urlFor`

### 2. HTMX
HTMX integration with partial rendering.
- HTMX partial component rendering
- Custom error handling for HTMX requests
- Middleware usage
- Dynamic content updates

### 3. Todo
Complete CRUD application with forms.
- Full CRUD operations
- Form handling with struct tags
- Custom ServeHTTP handlers
- In-memory data storage
- HTMX interactions

### 4. Blog-Admin
Advanced blog platform with admin panel demonstrating:
- **Dependency Injection**: Multiple services (DB, Auth, Session, Config)
- **Nested Routing**: 3-level deep route structure (`/admin/posts/{id}/edit`)
- **Authentication & Authorization**: Session-based auth with role-based access
- **Middleware Patterns**: Global and per-route middleware
- **Props Pattern**: Type-safe data loading with complex queries
- **Advanced HTMX**: Custom PageConfig, partial rendering, auto-save
- **Form Handling**: Structured form parsing with validation
- **Database Integration**: SQLite with transactions and indexes
- **Component Composition**: Reusable Templ components
- **API Endpoints**: JSON APIs alongside HTML pages
- **Query Parameters**: Pagination, filtering, search
- **Real-time Features**: Auto-save, live updates
- **Error Handling**: Custom error pages and graceful degradation

## Running the Examples

Each example is a standalone Go module. To run an example:

```shell
# Navigate to example directory
cd examples/simple/

# Download dependencies
go mod download

# Generate templ files
templ generate

# Run the server
go run .

# Or use templ's watch mode for development
templ generate --watch --proxy="http://localhost:8080" --cmd="go run ."
```

Open http://localhost:8080 in your browser.

## Learning Path

1. Start with **simple** to understand basic routing
2. Move to **htmx** to learn about partial rendering
3. Study **todo** for form handling and CRUD operations  
4. Explore **blog-admin** for production-ready patterns

Each example builds on concepts from the previous ones, demonstrating increasingly sophisticated use of structpages features.