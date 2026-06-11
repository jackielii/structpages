---
title: HTMX Integration
slug: /htmx
sidebar_position: 7
---

# HTMX Integration

structpages has built-in HTMX support enabled by default through `HTMXRenderTarget`. All HTMX requests for a page go to the SAME route; the framework picks which page component to render from the `HX-Target` header.

## The central loop

**One method reference — e.g. `index.TodoList` — drives three sites that must agree, and `ID`/`IDTarget` make them agree by construction:**

1. **Composition site** — where the page component is composed in, wrap it in an element with `id={ structpages.ID(ctx, index.TodoList) }`.
2. **Trigger site** — the element that fires the update points `hx-target={ structpages.IDTarget(ctx, index.TodoList) }` at the page's own route.
3. **Server site** — structpages matches the `HX-Target` header back to the page component by id, and the Props method branches on the injected `RenderTarget` with `target.Is(p.TodoList)` to load just that region's data.

```templ
// Site 1 — composition: set the element ID on the component's wrapper
<div id={ structpages.ID(ctx, index.TodoList) }>
    @p.TodoList(props.Todos)
</div>

// Site 2 — trigger: target that id, hit the page's own route
<input hx-get={ structpages.URLFor(ctx, index{}) }
       hx-target={ structpages.IDTarget(ctx, index.TodoList) }
       hx-swap="outerHTML" />
```

```go
// Site 3 — server: Props branches on the injected RenderTarget
func (p index) Props(r *http.Request, target structpages.RenderTarget) (IndexProps, error) {
    if target.Is(p.TodoList) {
        todos, err := getActiveTodos()
        if err != nil {
            return IndexProps{}, err
        }
        return IndexProps{}, structpages.RenderComponent(p.TodoList(todos))
    }
    todos, err := getAllTodos()
    if err != nil {
        return IndexProps{}, err
    }
    return IndexProps{Todos: todos}, nil
}
```

Because all three sites derive from the same method reference, renaming the method or moving the mount can't desynchronize them — there is no string id to drift. **Never hand-write the id at one site and generate it at another.**

## How ids are generated

`structpages.ID` / `structpages.IDTarget` generate deterministic element IDs from method references. The id is the page's **full field-name path from the root** joined with the method:

- `ID(ctx, index.TodoList)` → `"index-todo-list"` for a top-level page
- the same component on a page mounted at `admin.users` → `"admin-users-todo-list"`
- `IDTarget` prepends `#`

Including the ancestor path guarantees two different mounts of the same struct never collide. If the full id exceeds the length budget (default 40 chars, see `WithMaxIDLength`) it degrades to the compact leaf-only form with a stable hash suffix when the leaf name is shared.

**Components** (standalone templ functions) are prefixed by their package name: `ID(ctx, UserWidget)` → `"<package>-user-widget"`. Plain strings pass through unchanged — `IDTarget("body")` is `"body"`, not `"#body"` (literal CSS selectors are legitimate; literal URL paths are not).

**Self-render uses the current mount.** When `ID` runs inside a page's own templ, the id derives from *that mount's* field name — the same struct mounted under different parents produces different ids per render context. **Cross-page references with multiple mounts must be unambiguous** — a bare method expression errors with the available mounts listed; disambiguate with the `[]any` chain form, a `Ref`, or a standalone function:

```go
// IDTarget(ctx, []any{adminRoot{}, dashboardPage{}, "Header"})  // chain + string
// IDTarget(ctx, []any{adminRoot{}, dashboardPage.Header})       // chain + method expr
// IDTarget(ctx, Ref("AdminDash.Header"))                        // by field name
// IDTarget(ctx, EntryOverlaySlot)                               // standalone func: package-prefixed id
```

## RenderTarget in Props

The `RenderTarget` parameter tells your Props method **which page component will render**, so it can load only that region's data. Whatever selector configuration you use, `target.Is()` works the same — Props code is decoupled from the selection mechanism.

The shape that holds up in real pages: **partials get partial data, not the page props.** The page props struct exists for the full-page render and is typically *composed of* per-pane sub-structs; each partial takes its own pane struct. When a partial branch matches, you build just that pane's data, hand the constructed component to `RenderComponent`, and the page-props value you return alongside is **ignored**:

```go
// Page props compose the panes; each partial takes its own pane struct.
type TeamManagementProps struct {
    UserPaneProps
    GroupPaneProps
}

type UserPaneProps struct {
    Users           []UserWithGroups
    UserSearchQuery string
}

type GroupPaneProps struct {
    Groups           []db.GroupWithCounts
    GroupSearchQuery string
}

func (p TeamManagementView) Props(r *http.Request, sel structpages.RenderTarget, appCtx *AppContext) (TeamManagementProps, error) {
    switch {
    case sel.Is(p.GroupList):
        groups, err := p.GroupListProps(r, appCtx)
        if err != nil {
            return TeamManagementProps{}, err
        }
        // Partial data only — the TeamManagementProps{} return is ignored.
        return TeamManagementProps{}, structpages.RenderComponent(p.GroupList(groups))

    case sel.Is(p.UserList):
        userPane, err := p.UserListProps(r, appCtx)
        if err != nil {
            return TeamManagementProps{}, err
        }
        return TeamManagementProps{}, structpages.RenderComponent(p.UserList(userPane))

    default:
        // Full page, boosted Content swap, or anything unrecognised:
        // fall back to the full props — never to empty props.
        return p.fullProps(r, appCtx)
    }
}

// Per-pane helper props methods. NOT auto-invoked (only the method literally
// named Props is) — they're plain methods feeding both the partial branches
// above and fullProps below.
func (p TeamManagementView) GroupListProps(r *http.Request, appCtx *AppContext) (GroupPaneProps, error) {
    groups, err := appCtx.Store.SearchGroupsWithCounts(r.Context(), r.FormValue("group-search"))
    if err != nil {
        return GroupPaneProps{}, fmt.Errorf("search groups: %w", err)
    }
    return GroupPaneProps{Groups: groups, GroupSearchQuery: r.FormValue("group-search")}, nil
}

func (p TeamManagementView) UserListProps(r *http.Request, appCtx *AppContext) (UserPaneProps, error) {
    users, err := appCtx.Store.SearchUsersWithGroups(r.Context(), r.FormValue("user-search"))
    if err != nil {
        return UserPaneProps{}, fmt.Errorf("search users: %w", err)
    }
    return UserPaneProps{Users: users, UserSearchQuery: r.FormValue("user-search")}, nil
}

func (p TeamManagementView) fullProps(r *http.Request, appCtx *AppContext) (TeamManagementProps, error) {
    userPane, err := p.UserListProps(r, appCtx)
    if err != nil {
        return TeamManagementProps{}, err
    }
    groupPane, err := p.GroupListProps(r, appCtx)
    if err != nil {
        return TeamManagementProps{}, err
    }
    return TeamManagementProps{UserPaneProps: userPane, GroupPaneProps: groupPane}, nil
}

templ (p TeamManagementView) Page(props TeamManagementProps) {
    <div class="team-management">
        <div class="user-pane">
            <input hx-get={ structpages.URLFor(ctx, TeamManagementView{}) }
                   hx-target={ structpages.IDTarget(ctx, TeamManagementView.UserList) }
                   name="user-search" />
            <div id={ structpages.ID(ctx, TeamManagementView.UserList) }>
                @p.UserList(props.UserPaneProps)
            </div>
        </div>

        <div class="group-pane">
            <input hx-get={ structpages.URLFor(ctx, TeamManagementView{}) }
                   hx-target={ structpages.IDTarget(ctx, TeamManagementView.GroupList) }
                   name="group-search" />
            <div id={ structpages.ID(ctx, TeamManagementView.GroupList) }>
                @p.GroupList(props.GroupPaneProps)
            </div>
        </div>
    </div>
}

templ (p TeamManagementView) UserList(pane UserPaneProps) {
    // renders pane.Users, preserves pane.UserSearchQuery in the input
}

templ (p TeamManagementView) GroupList(pane GroupPaneProps) {
    // renders pane.Groups
}
```

Each pane updates independently: typing in the user search box re-renders only `UserList`, with only the user query running. The composition site passes the same pane struct (`props.UserPaneProps`) the partial branch builds — full render and partial re-render share one component signature.

### Why `RenderComponent(p.X(args))` and not `RenderComponent(target, args)`

`p.GroupList(groups)` is a normal Go call — the compiler checks argument types and counts. The reflective forms (`RenderComponent(target, args)`, `RenderComponent(index.TodoList, args)`) defer those checks to runtime. Use the reflective method-expression form only for components whose parameters the framework should DI-inject; for everything else, construct the component.

### Overriding the selection

Props can render a different component than the one selected — return any constructed component:

```go
func (p search) Props(r *http.Request, target structpages.RenderTarget) (SearchProps, error) {
    query := r.URL.Query().Get("q")
    if query == "" {
        return SearchProps{}, structpages.RenderComponent(p.EmptyState())
    }
    results, err := performSearch(query)
    if err != nil {
        return SearchProps{}, err
    }
    if target.Is(p.Results) {
        return SearchProps{}, structpages.RenderComponent(p.Results(results))
    }
    return SearchProps{Results: results}, nil
}
```

### Standalone components shared across pages

A component (standalone templ function) can be an HTMX target without belonging to any page. `target.Is(UserStatsWidget)` matches it, and the package-prefixed id is stable regardless of which pages embed it:

```go
templ UserStatsWidget(stats UserStats) {
    <div id={ structpages.ID(ctx, UserStatsWidget) }>{ stats.ActiveUsers } active users</div>
}

func (p dashboardPage) Props(r *http.Request, target structpages.RenderTarget, store *Store) (DashboardProps, error) {
    if target.Is(UserStatsWidget) {
        stats, err := store.LoadUserStats(r.Context())
        if err != nil {
            return DashboardProps{}, err
        }
        return DashboardProps{}, structpages.RenderComponent(UserStatsWidget(stats))
    }
    // ... full page
}
```

## Nested swap levels (Page → Content → Detail)

A page's page components can be composed into **nested swap levels**, each an independent HTMX target. The levels are *not* a tree the matcher walks — they're sibling page components on one page, each with its own id. Because `HX-Target` selects the page component whose id it matches exactly, targeting a given level re-renders *only* that level, even though `Page` composes `Content` composes `Detail`:

- **`Page`** — the full document. Cold loads and `hx-boost` body swaps. Composes the app layout around `Content`.
- **`Content`** — the page's main region. Holds the page chrome — heading, back-link, toolbar — around the inner level. Swapped on boosted nav between pages.
- **`Detail`** (or another inner name) — a region *inside* `Content` that must swap on its own. Holds **none** of the page chrome.

```templ
templ (d FooDetail) Page(p Props)    { @layout(title) { <main>@d.Content(p)</main> } }
templ (d FooDetail) Content(p Props) { <div id={ structpages.ID(ctx, FooDetail.Content) }>
                                          <a href={ structpages.URLFor(ctx, FooList{}) }>&larr; Foos</a>
                                          @d.Detail(p)
                                        </div> }
templ (FooDetail) Detail(p Props)    { <div id={ structpages.ID(ctx, FooDetail.Detail) }>
                                          // fields, actions — NO back-link, NO header
                                        </div> }
```

**Why three levels, not two.** The trap is reusing `Content` as the swap fragment for an embedded region — e.g. a master-detail inspector pane hosting the standalone detail page's `Content`. That drags the page chrome into the pane. Splitting out `Detail` gives the embedded region a chrome-less partial while `Content` keeps the standalone-page chrome. **The level you embed/swap is the one with no chrome of its own.**

The rule generalizes: **one page component per independently-swappable region, outer wraps inner, embed/target the innermost that has no chrome above it.**

## Custom target selectors

The default `HTMXRenderTarget` covers HTMX 1.x/2.x. For htmx 4 — which reshaped `HX-Target` to `"<tag>#<id>"` and added `HX-Request-Type` — wire the v4 variant:

```go
sp, err := structpages.Mount(mux, pages{}, "/", "App",
    structpages.WithTargetSelector(structpages.HTMXv4RenderTarget),
)
```

For fully custom logic, return any value implementing `RenderTarget` (`Is(method any) bool`), typically delegating to `HTMXRenderTarget` for the cases you don't override. If your target also implements `Component() component`, then `RenderComponent(target)` (no args) renders it directly:

```go
type jsonTarget struct{ data any }

func (t jsonTarget) Is(method any) bool { return false }
func (t jsonTarget) Component() component {
    return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
        return json.NewEncoder(w).Encode(t.data)
    })
}

sp, err := structpages.Mount(mux, pages{}, "/", "App",
    structpages.WithTargetSelector(func(r *http.Request, pn *structpages.PageNode) (structpages.RenderTarget, error) {
        if r.Header.Get("Accept") == "application/json" {
            return jsonTarget{data: loadData(r, pn)}, nil
        }
        return structpages.HTMXRenderTarget(r, pn)
    }),
)
```

The exact matching algorithm (including the authoritative pass against real generated ids) is documented in the [API reference](./api.md#htmxrendertarget).

## See also

- `examples/htmx`, `examples/todo`, and `examples/htmx-render-target` in the [repository](https://github.com/jackielii/structpages/tree/main/examples) for complete working code.
- [Error Handling](./error-handling.md) for HTMX-aware redirects (`HX-Location`).
