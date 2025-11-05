# HTMX Integration

Structpages has built-in HTMX support enabled by default through `HTMXRenderTarget`. This makes `ID` and `IDTarget` work seamlessly with HTMX partial rendering out of the box.

### How It Works

When an HTMX request is detected (via `HX-Request` header), the framework automatically:

1. Reads the `HX-Target` header value
2. Matches it to a component method or standalone function
3. Renders that specific component instead of the full page

For example:
- `HX-Target: "content"` → calls `Content()` method
- `HX-Target: "index-todo-list"` → calls `TodoList()` method on the index page (strips page prefix automatically)
- `HX-Target: "dashboard-page-user-stats-widget"` → calls `UserStatsWidget` standalone function
- No HX-Target or non-existent component → falls back to `Page()` method

This works automatically with `ID` and `IDTarget`:

```go
// In your template
<div id={ structpages.ID(ctx, index.TodoList) }>
    @p.TodoList()
</div>

// In HTMX attributes
hx-target={ structpages.IDTarget(ctx, index.TodoList) }  // Generates "#index-todo-list"
```

The HTMX request will automatically extract the component name from the target ID and render just that component.

---

## RenderTarget and Props Integration

The real power of HTMX integration comes from the `RenderTarget` parameter in your `Props` method. RenderTarget tells your Props method **which component will be rendered**, allowing you to:

- ✅ Load only the data needed for that specific component
- ✅ Optimize database queries for partial updates
- ✅ Override component selection based on application logic
- ✅ Maintain type safety throughout the flow
- ✅ Use standalone function components shared across pages

**Important:** While `HTMXRenderTarget` is configurable (you can customize how components are selected from HTMX requests), `RenderTarget.Is()` works regardless of your configuration. Whatever component selection logic you use, the `RenderTarget` passed to Props will correctly identify which component was selected, making your Props code independent of the selection mechanism.

### How Component Selection Works

When an HTMX request arrives:

```
1. Request arrives with HX-Target header
   ↓
2. HTMXRenderTarget extracts target ID (e.g., "index-todo-list")
   ↓
3. Component is determined (e.g., TodoList method or UserStatsWidget function)
   ↓
4. RenderTarget is created with that component (lazy evaluation for functions)
   ↓
5. Props(r, target) is called with the RenderTarget
   ↓
6. Props loads appropriate data based on target.Is(component)
   ↓
7. Component renders with the data
```

### Basic Pattern: Conditional Data Loading

Use `RenderTarget` to load only what you need:

```go
type index struct{}

type IndexProps struct {
    Todos      []Todo
    Stats      DashboardStats
    UserInfo   UserInfo
}

func (p index) Props(r *http.Request, target structpages.RenderTarget) (IndexProps, error) {
    switch {
    case target.Is(p.TodoList):
        // HTMX is updating just the todo list - only load todos
        return IndexProps{
            Todos: getTodos(),
        }, nil

    case target.Is(p.Page):
        // Full page load - load everything
        return IndexProps{
            Todos:    getTodos(),
            Stats:    getDashboardStats(),
            UserInfo: getCurrentUser(),
        }, nil

    default:
        // Fallback
        return IndexProps{}, nil
    }
}

templ (p index) Page(props IndexProps) {
    <div class="dashboard">
        <div class="header">{ props.UserInfo.Name }</div>
        <div class="stats">{ props.Stats.String() }</div>

        <div id={ structpages.ID(ctx, index.TodoList) }>
            @p.TodoList(props.Todos)
        </div>
    </div>
}

templ (p index) TodoList(todos []Todo) {
    for _, todo := range todos {
        <div>{ todo.Text }</div>
    }
}
```

**What happens:**
- Initial page load → `target.Is(index.Page)` is true → loads all data
- HTMX updates todo list → `target.Is(index.TodoList)` is true → loads only todos
- Database queries are minimized for partial updates ⚡

### Advanced Pattern: RenderComponent Override

Sometimes you need to render a different component than what was selected, or you want to pass specific data to a component. Use `RenderComponent` within Props:

```go
type TeamManagementView struct{}

type TeamManagementProps struct {
    UserPaneProps  UserPaneProps
    GroupPaneProps GroupPaneProps
}

type UserPaneProps struct {
    Users           []UserWithGroups
    UserSearchQuery string
}

type GroupPaneProps struct {
    Groups           []Group
    GroupSearchQuery string
}

func (p TeamManagementView) Props(r *http.Request, target structpages.RenderTarget) (TeamManagementProps, error) {
    switch {
    case target.Is(p.GroupList):
        // Load only group data
        groups, err := loadGroups(r)
        if err != nil {
            return TeamManagementProps{}, err
        }
        // Override: render GroupList with just the groups data
        return TeamManagementProps{}, structpages.RenderComponent(target, groups)

    case target.Is(p.UserList):
        // Load only user data
        users, err := loadUsers(r)
        if err != nil {
            return TeamManagementProps{}, err
        }
        // Override: render UserList with just the users data
        return TeamManagementProps{}, structpages.RenderComponent(target, users)

    case target.Is(p.Page), target.Is(p.Content):
        // Full page - load everything
        users, err := loadUsers(r)
        if err != nil {
            return TeamManagementProps{}, err
        }

        groups, err := loadGroups(r)
        if err != nil {
            return TeamManagementProps{}, err
        }

        return TeamManagementProps{
            UserPaneProps: UserPaneProps{
                Users:           users,
                UserSearchQuery: r.FormValue("user-search"),
            },
            GroupPaneProps: GroupPaneProps{
                Groups:           groups,
                GroupSearchQuery: r.FormValue("group-search"),
            },
        }, nil

    default:
        // Fallback to full props
        // ... load everything
    }
}

templ (p TeamManagementView) Page(props TeamManagementProps) {
    <div class="team-management">
        <div class="user-pane">
            <input hx-get="/search-users"
                   hx-target={ structpages.IDTarget(ctx, TeamManagementView.UserList) }
                   name="user-search" />

            <div id={ structpages.ID(ctx, TeamManagementView.UserList) }>
                @p.UserList(props.UserPaneProps.Users)
            </div>
        </div>

        <div class="group-pane">
            <input hx-get="/search-groups"
                   hx-target={ structpages.IDTarget(ctx, TeamManagementView.GroupList) }
                   name="group-search" />

            <div id={ structpages.ID(ctx, TeamManagementView.GroupList) }>
                @p.GroupList(props.GroupPaneProps.Groups)
            </div>
        </div>
    </div>
}

templ (p TeamManagementView) UserList(users []UserWithGroups) {
    for _, user := range users {
        <div>{ user.Name }</div>
    }
}

templ (p TeamManagementView) GroupList(groups []Group) {
    for _, group := range groups {
        <div>{ group.Name }</div>
    }
}
```

**Key Points:**

1. **Props returns full structure** (`TeamManagementProps`) for the Page component
2. **Individual components** have simpler signatures (`UserList([]UserWithGroups)`)
3. **RenderComponent override** passes specific data to specific components
4. **Type safety** is maintained - component signatures enforce correct data types

**When to use RenderComponent in Props:**
- ✅ Complex pages with multiple independent sections
- ✅ Different components need different data structures
- ✅ Want to avoid returning empty/partial complex props
- ✅ Need to optimize data loading per component

### Complete Example: Search with Dynamic Rendering

```go
type search struct {
    query `route:"GET /search"`
}

func (p search) Props(r *http.Request, target structpages.RenderTarget) ([]Result, error) {
    query := r.URL.Query().Get("q")

    // Override based on application logic
    if query == "" {
        // No search query - show empty state instead of results
        return nil, structpages.RenderComponent(p.EmptyState)
    }

    // Check which component was selected
    switch {
    case target.Is(p.Results):
        // Perform search and return results
        return performSearch(query), nil

    case target.Is(p.Page):
        // Full page with recent searches
        return performSearch(query), nil

    default:
        return nil, nil
    }
}

templ (p search) Page(results []Result) {
    <div class="search-page">
        <input hx-get={ structpages.URLFor(ctx, query{}) }
               hx-target={ structpages.IDTarget(ctx, search.Results) }
               name="q"
               placeholder="Search..." />

        <div id={ structpages.ID(ctx, search.Results) }>
            @p.Results(results)
        </div>
    </div>
}

templ (p search) Results(results []Result) {
    if len(results) == 0 {
        <p>No results found</p>
    }
    for _, result := range results {
        <div>{ result.Title }</div>
    }
}

templ (p search) EmptyState() {
    <div class="empty-state">
        <p>Enter a search query to get started</p>
    </div>
}
```

**What happens:**
- User types → HTMX sends request with HX-Target: "search-results"
- If query is empty → Props returns `RenderComponent(search.EmptyState)`
- If query exists → Props loads results and renders Results component
- Component selection can be overridden based on business logic ✨

---

## Common Patterns Summary

### Pattern 1: Simple Conditional Loading
```go
func (p index) Props(r *http.Request, target structpages.RenderTarget) (Props, error) {
    if target.Is(p.Component) {
        return loadMinimalData(), nil
    }
    return loadFullData(), nil
}
```
**Use when:** Single props type works for all components, just need to load different amounts of data.

### Pattern 2: RenderComponent Override
```go
func (p index) Props(r *http.Request, target structpages.RenderTarget) (Props, error) {
    if target.Is(p.Component) {
        data := loadSpecificData()
        return Props{}, structpages.RenderComponent(target, data)
    }
    return loadFullProps(), nil
}
```
**Use when:** Individual components need different data types than the full page props.

### Pattern 3: Dynamic Component Selection
```go
func (p index) Props(r *http.Request, target structpages.RenderTarget) (Props, error) {
    if someCondition {
        return Props{}, structpages.RenderComponent(p.AlternateComponent)
    }
    // Normal flow
    return loadData(), nil
}
```
**Use when:** Need to change which component renders based on request data or application state.

### Pattern 4: Standalone Function Components
```go
// Shared widget component (standalone function)
templ UserStatsWidget(stats UserStats) {
    <div>{ stats.ActiveUsers } active users</div>
}

func (p DashboardPage) Props(r *http.Request, target structpages.RenderTarget) (DashboardProps, error) {
    // Check against standalone function
    if target.Is(UserStatsWidget) {
        stats := loadUserStats()
        return DashboardProps{}, structpages.RenderComponent(target, stats)
    }
    return loadFullData(), nil
}
```
**Use when:** Need to share components across multiple pages without creating wrapper methods.

---

### Custom Target Selector

The default `HTMXRenderTarget` works for most use cases, but you can customize the component selection logic if needed:

```go
mux := http.NewServeMux()
sp, err := structpages.Mount(mux, pages{}, "/", "My App",
    structpages.WithTargetSelector(func(r *http.Request, pn *PageNode) (structpages.RenderTarget, error) {
        // Your custom logic
        // For example, select based on custom headers, query params, etc.
        if customTarget := r.Header.Get("X-Custom-Target"); customTarget != "" {
            // Look up component by custom header
            if method, ok := pn.Components[customTarget]; ok {
                return structpages.NewMethodRenderTarget(customTarget, method), nil
            }
        }
        // Fall back to default HTMX behavior
        return structpages.HTMXRenderTarget(r, pn)
    }),
)
if err != nil {
    log.Fatal(err)
}
```

**Key insight:** No matter how you configure component selection (whether using the default `HTMXRenderTarget` or a custom selector), your Props method receives a `RenderTarget` that correctly identifies the selected component. Your Props code using `target.Is(component)` remains the same and works with any component selection strategy.

This separation of concerns means:
- ✅ You can change component selection logic without modifying Props
- ✅ Props code is decoupled from HTMX request details
- ✅ The pattern works whether requests come from HTMX, regular navigation, or custom clients

See `examples/htmx/main.go`, `examples/todo/main.go`, and `examples/htmx-render-target/` for complete working examples.

