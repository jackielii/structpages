# Supported Request Flows

This document explains how structpages processes different types of requests and routes them to your components.

## Quick Reference

| Flow | When To Use | Entry Point | Component Selection | Example |
|------|-------------|-------------|---------------------|---------|
| **1** | Full control over request/response | `ServeHTTP(w, r)` | N/A | File uploads, WebSockets |
| **2** | Actions that render components | `ServeHTTP(w, r) error` | Return `RenderComponent(method)` | Add todo → render todo list |
| **3** | Actions with database/logger | `ServeHTTP(w, r, db, logger) error` | Return `RenderComponent(method)` | CRUD operations |
| **4** | Standard pages with HTMX | `Props(r, sel)` + Component methods | Automatic via `HTMXPageConfig` + `RenderTarget` | **Primary pattern** ⭐ |

## Flow 4: Component-Based Rendering (Recommended)

This is the primary way to build pages in structpages. It handles both full page loads and HTMX partial updates automatically.

### How It Works

```
1. Request arrives
   ↓
2. Component Selection (HTMXPageConfig decides which component to render)
   ├─ Regular request → "Page" component
   ├─ HTMX request with HX-Target: "content" → "Content" component
   └─ HTMX request with HX-Target: "index-todo-list" → "TodoList" component
   ↓
3. Props runs (with RenderTarget injected)
   - Knows which component will render
   - Returns appropriate data
   ↓
4. Component renders with props data
```

### Request Type Variations

| Request Type | HX-Request | HX-Target | Selected Component | Props Receives | Use Case |
|-------------|-----------|-----------|-------------------|----------------|----------|
| **Browser navigation** | ❌ No | N/A | `Page` | `sel.Is(index.Page) == true` | Initial page load |
| **HTMX boost** | ✅ Yes | ❌ Empty | `Page` | `sel.Is(index.Page) == true` | Progressive enhancement |
| **Simple HTMX target** | ✅ Yes | `"content"` | `Content` | `sel.Is(index.Content) == true` | Direct component name |
| **IDFor target** ⭐ | ✅ Yes | `"index-todo-list"` | `TodoList` | `sel.Is(index.TodoList) == true` | **Primary pattern** |
| **Unknown target** | ✅ Yes | `"nonexistent"` | `Page` (fallback) | `sel.Is(index.Page) == true` | Graceful degradation |

### Complete Example: Flow 4 with RenderTarget

```go
type index struct {
    add `route:"POST /add"`
}

// Props knows which component will render via RenderTarget
func (p index) Props(r *http.Request, sel *structpages.RenderTarget) ([]Todo, error) {
    switch {
    case sel.Is(index.TodoList):
        // Only load active todos for TodoList component
        return getActiveTodos(), nil

    case sel.Is(index.Page):
        // Load everything for full page
        return getAllTodos(), nil

    default:
        return nil, nil
    }
}

templ (p index) Page(todos []Todo) {
    @html() {
        <form hx-post={ structpages.URLFor(ctx, add{}) }
              hx-target={ structpages.IDFor(ctx, index.TodoList) }>
            <input name="text" />
            <button>Add</button>
        </form>
        <div id={ structpages.IDFor(ctx, structpages.IDParams{Method: index.TodoList, RawID: true}) }>
            @p.TodoList(todos)
        </div>
    }
}

templ (p index) TodoList(todos []Todo) {
    for _, todo := range todos {
        <div>{ todo.Text }</div>
    }
}
```

**What happens:**
1. **Initial page load**: Browser requests `/` → Props gets `sel.Is(index.Page) == true` → loads all todos → renders full page
2. **Add todo via HTMX**: Form submits → Add handler runs → returns `RenderComponent(index.TodoList)` → Props gets `sel.Is(index.TodoList) == true` → loads active todos → renders just TodoList component → HTMX swaps it in

**Benefits:**
- ✅ Props efficiently loads only needed data
- ✅ No duplicate component selection logic
- ✅ Type-safe with compile-time checks
- ✅ Zero configuration - works out of the box

---

### Complex Props Pattern (Real-World Pages)

In real applications, the full page often needs complex props with many fields, while individual components only need a subset. Here's the recommended pattern:

```go
// Complex props structure for the full page
type IndexProps struct {
    Users      []User
    Picklists  []Picklist
    Search     string
    TotalCount int
}

type index struct {
    addUser    `route:"POST /add-user"`
    searchUser `route:"GET /search"`
}

// Props returns the full IndexProps structure
// This matches the Page component signature
func (p index) Props(r *http.Request, sel *RenderTarget) (IndexProps, error) {
    switch {
    case sel.Is(index.Page):
        // Full page load - get everything
        return IndexProps{
            Users:      getAllUsers(),
            Picklists:  getPicklists(),
            Search:     r.URL.Query().Get("q"),
            TotalCount: getUserCount(),
        }, nil

    default:
        // For action handlers, return minimal data
        // The action will use RenderComponent to select what to render
        return IndexProps{}, nil
    }
}

// Page component receives the full props
templ (p index) Page(props IndexProps) {
    @html() {
        <div>
            <input hx-get={ structpages.URLFor(ctx, searchUser{}) }
                   hx-target={ structpages.IDFor(ctx, index.UserList) }
                   name="q" />
            <span>Total: { strconv.Itoa(props.TotalCount) }</span>
        </div>

        <div id={ structpages.IDFor(ctx, structpages.IDParams{Method: index.UserList, RawID: true}) }>
            @p.UserList(props.Users)
        </div>

        <div id={ structpages.IDFor(ctx, structpages.IDParams{Method: index.PicklistDropdown, RawID: true}) }>
            @p.PicklistDropdown(props.Picklists)
        </div>
    }
}

// Individual components receive only what they need
templ (p index) UserList(users []User) {
    for _, user := range users {
        <div>{ user.Name }</div>
    }
}

templ (p index) PicklistDropdown(picklists []Picklist) {
    <select>
        for _, item := range picklists {
            <option value={ item.ID }>{ item.Name }</option>
        }
    </select>
}

// Action handler: extract specific data and render specific component
func (a addUser) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
    name := r.FormValue("name")

    // Add the user
    addUser(name)

    // Get fresh user list
    users := getAllUsers()

    // Render just the UserList component with only the users data
    return structpages.RenderComponent(index.UserList, users)
}

// Search handler: dynamically load data and render
func (s searchUser) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
    query := r.URL.Query().Get("q")

    // Search users
    users := searchUsers(query)

    // Render UserList with search results
    return structpages.RenderComponent(index.UserList, users)
}
```

**Key Points:**

1. **Props returns full structure**: The `Props` method returns `IndexProps` to match the `Page` component signature
2. **Components receive subsets**: Individual components like `UserList([]User)` receive only what they need
3. **Action handlers use RenderComponent**: When rendering a specific component, extract the needed data and pass it to `RenderComponent`
4. **Type safety maintained**: The component signatures enforce what data is needed at compile time

**When to use this pattern:**
- ✅ Complex pages with multiple sections/components
- ✅ Different components need different subsets of data
- ✅ Action handlers that update specific parts of the page
- ✅ Dynamic data loading (search, filters, pagination)

**Why it works:**
- Props and Page stay in sync (both use `IndexProps`)
- Components are reusable with simple signatures
- Action handlers have full control over what to render
- No need to return different types from Props for different components

---

## Other Flows (Advanced Usage)

### Flow 1: Standard `http.Handler`
```
Request → ServeHTTP(w, r) → Page writes response directly
```

**When to use:** Need complete control over the response (WebSockets, SSE, file downloads)

```go
type fileUpload struct{}

func (f fileUpload) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    file, header, _ := r.FormFile("upload")
    defer file.Close()
    // Process file...
    fmt.Fprintf(w, "Uploaded: %s", header.Filename)
}
```

---

### Flow 2: Action Handlers That Render

**When to use:** Form submissions, button clicks that need to render a component

```go
type add struct{}

func (a add) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
    text := r.FormValue("text")
    if text == "" {
        return fmt.Errorf("text is required")
    }
    addTodo(text)

    // Tell structpages to render the TodoList component
    return structpages.RenderComponent(index.TodoList)
}
```

**Flow:**
- ✅ Perform action (add todo)
- ✅ Return `RenderComponent(method)` to render a component
- ✅ Props runs with `RenderTarget` for that component
- ✅ Component renders with fresh data

---

### Flow 3: Action Handlers With Dependencies

**When to use:** Actions that need database, logger, or other services

```go
type userManager struct{}

func (u userManager) ServeHTTP(w http.ResponseWriter, r *http.Request,
                                db *sql.DB, logger *Logger) error {
    id := r.PathValue("id")

    // Use injected dependencies
    user, err := db.QueryUser(id)
    if err != nil {
        logger.Error("Failed to query user", err)
        return err
    }

    // Render user list component
    return structpages.RenderComponent(userManager.UserList)
}

// Pass dependencies when mounting pages
mux := http.NewServeMux()
sp, err := structpages.Mount(mux, pages{}, "/", "App",
    structpages.WithArgs(db, logger),
)
if err != nil {
    log.Fatal(err)
}
```

**Same as Flow 2, but with injected dependencies available**

---

## Key Takeaways

### RenderTarget Benefits

Props receives RenderTarget to know which component will render:

```go
func (p index) Props(r *http.Request, sel *RenderTarget) (interface{}, error) {
    switch {
    case sel.Is(index.TodoList):
        return getTodos(), nil
    case sel.Is(index.Page):
        return getAllData(), nil
    }
}
```

**Benefits:**
- Component selection happens once (before Props)
- Type-safe method expressions
- Efficient data loading (load only what's needed)
- Refactoring-safe (compile-time checks)

### Execution Order

Flow 4 executes in this order:
```
1. findComponent() → Determines which component (e.g., "TodoList")
2. Create RenderTarget with selected component
3. Props() → Receives RenderTarget, returns data
4. Render component with data from Props
```

**Key insight:** Component selection happens BEFORE Props runs, so Props knows what it's loading data for.

### Props Override Pattern

Props can override the selected component:

```go
func (p search) Props(r *http.Request, sel *RenderTarget) ([]Result, error) {
    query := r.URL.Query().Get("q")

    // Override component selection based on logic
    if query == "" {
        return nil, structpages.RenderComponent(search.EmptyState)
    }

    // Normal flow uses RenderTarget
    switch {
    case sel.Is(search.Results):
        return performSearch(query), nil
    }
}
```

---

## See Also

- [RenderTarget Guide](./component-selection.md) - Detailed RenderTarget documentation
- [IDFor Usage](../IDFOR_USAGE.md) - Type-safe ID generation
- [examples/todo](../examples/todo) - Complete working example
