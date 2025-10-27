# HTMX Integration

Structpages has built-in HTMX support enabled by default through `HTMXPageConfig`. This makes `IDFor` work seamlessly with HTMX partial rendering out of the box.

### How It Works

When an HTMX request is detected (via `HX-Request` header), the framework automatically:

1. Reads the `HX-Target` header value
2. Converts it from kebab-case to a component method name
3. Renders that specific component instead of the full page

For example:
- `HX-Target: "content"` → calls `Content()` method
- `HX-Target: "index-todo-list"` → calls `TodoList()` method on the index page (strips page prefix automatically)
- No HX-Target or non-existent component → falls back to `Page()` method

This works automatically with `IDFor`:

```go
// In your template
<div id={ structpages.IDFor(ctx, structpages.IDParams{Method: index.TodoList, RawID: true}) }>
    @p.TodoList()
</div>

// In HTMX attributes
hx-target={ structpages.IDFor(ctx, index.TodoList) }  // Generates "#index-todo-list"
```

The HTMX request will automatically extract the component name from the target ID and render just that component.

### Custom Component Selector

If you need different behavior, you can override the default:

```go
sp := structpages.New(
    structpages.WithDefaultComponentSelector(func(r *http.Request, pn *PageNode) (string, error) {
        // Your custom logic
        return "Page", nil
    }),
)
```

See `examples/htmx/main.go` and `examples/todo/main.go` for complete working examples.

