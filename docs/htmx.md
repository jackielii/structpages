---
title: HTMX Integration
slug: /htmx
sidebar_position: 7
---

# HTMX Integration

All htmx requests for a page go to the SAME route; the framework picks which page component to render from the `HX-Target` header. That reading is opt-in: choose the target selector for your htmx version at `Mount`.

```go
sp, err := structpages.Mount(mux, pages{}, "/", "App",
    structpages.WithTargetSelector(structpages.HTMXRenderTarget),   // htmx 1.x and 2.x
    // structpages.WithTargetSelector(structpages.HTMXv4RenderTarget), // htmx 4
)
```

Without a selector, every request wants `Page`, so a partial request silently gets the full page. The [linter's `selector` check](./lint.md) flags a `Mount` without one when its pages' templates use htmx. This page covers the htmx side; [How a Request Is Rendered](./rendering.md) defines the flow it relies on.

## The central loop

**One method reference — e.g. `index.TodoList` — drives three sites that must agree, and the gsx `id`/`target` filters (`structpages.ID`/`IDTarget`) make them agree by construction:**

1. **Composition site** — where the page component is composed in, wrap it in an element with `id={index.TodoList |> id}`.
2. **Trigger site** — the element that fires the update points `hx-target={index.TodoList |> target}` at the page's own route.
3. **Server site** — the htmx target selector matches the `HX-Target` header back to the page component by id. Props asks the injected `RenderTarget` with `target.Is(p.TodoList)` and loads just that region's data; the framework then renders `TodoList` with it.

The filters are registered once in `gsx.toml` (see [gsx Patterns](./gsx.md#register-the-filters)); templ users call the functions directly (see [Using templ](./templ.md)).

```gsx
component (p index) Content(props IndexProps) {
    // Site 2 — trigger: target that id, hit the page's own route
    <input
        name="q"
        hx-get={index{} |> url}
        hx-target={index.TodoList |> target}
    />
    // Site 1 — composition: set the element id on the component's wrapper
    <div id={index.TodoList |> id}>
        <p.TodoList todos={props.Todos}/>
    </div>
}
```

```go
// Site 3 — server: Props loads what the request wants
func (p index) Props(target structpages.RenderTarget) (IndexProps, []Todo, error) {
    if target.Is(p.TodoList) {
        todos, err := getActiveTodos()
        return IndexProps{}, todos, err // the framework renders TodoList(todos)
    }
    todos, err := getAllTodos()
    return IndexProps{Todos: todos}, nil, err // renders Page or Content
}
```

`TodoList(todos []Todo)` and `Content(props IndexProps)` each take their own share of Props' results by type ([Components take what they need by type](./rendering.md#components-take-what-they-need-by-type)).

Because all three sites derive from the same method reference, renaming the method or moving the mount can't desynchronize them — there is no string id to drift. **Never hand-write the id at one site and generate it at another.**

## How ids are generated

`structpages.ID` / `structpages.IDTarget` (the `id` / `target` filters) generate deterministic element IDs from method references. The id is the page's **full field-name path from the root** joined with the method:

- `ID(ctx, index.TodoList)` → `"index-todo-list"` for a top-level page
- the same component on a page mounted at `admin.users` → `"admin-users-todo-list"`
- `IDTarget` prepends `#`

Including the ancestor path guarantees two different mounts of the same struct never collide. If the full id exceeds the length budget (default 40 chars, see `WithMaxIDLength`) it degrades to the compact leaf-only form with a stable hash suffix when the leaf name is shared.

**Components** (standalone `component` declarations) are prefixed by their package name: `ID(ctx, UserWidget)` → `"<package>-user-widget"`. Plain strings pass through unchanged — `"body" |> target` is `"body"`, not `"#body"` (literal CSS selectors are legitimate; literal URL paths are not).

**Self-render uses the current mount.** When `ID` runs inside a page's own render, the id derives from *that mount's* field name — the same struct mounted under different parents produces different ids per render context. **Cross-page references with multiple mounts must be unambiguous** — a bare method expression errors with the available mounts listed; disambiguate with the `[]any` chain form, a `Ref`, or a standalone function:

```go
// IDTarget(ctx, []any{adminRoot{}, dashboardPage{}, "Header"})  // chain + string
// IDTarget(ctx, []any{adminRoot{}, dashboardPage.Header})       // chain + method expr
// IDTarget(ctx, Ref("AdminDash.Header"))                        // by field name
// IDTarget(ctx, EntryOverlaySlot)                               // standalone func: package-prefixed id
```

## RenderTarget in Props

The `RenderTarget` parameter tells Props **what the request wants**, so it can load only that region's data. `target.Is(p.UserList)` is a plain question with no side effects; it works the same whichever selector the mount uses, so Props code doesn't depend on how the request was read.

When Props returns no error, the framework renders the page's component the target `Is`, or `Page` if it `Is` none of them, filling the component's parameters by type from Props' results. The shape that holds up in real pages: **partials get partial data, not the page props.** Return one value per pane, and let each component declare only the panes it renders:

```go
type UserPaneProps struct {
    Users           []UserWithGroups
    UserSearchQuery string
}

type GroupPaneProps struct {
    Groups           []db.GroupWithCounts
    GroupSearchQuery string
}

func (p TeamManagementView) Props(r *http.Request, t structpages.RenderTarget, appCtx *AppContext) (UserPaneProps, GroupPaneProps, error) {
    switch {
    case t.Is(p.UserList):
        users, err := p.userPane(r, appCtx)
        return users, GroupPaneProps{}, err // renders UserList

    case t.Is(p.GroupList):
        groups, err := p.groupPane(r, appCtx)
        return UserPaneProps{}, groups, err // renders GroupList

    default:
        // Full page, boosted Content swap, or anything unrecognised:
        // load everything — never return empty props here.
        users, err := p.userPane(r, appCtx)
        if err != nil {
            return UserPaneProps{}, GroupPaneProps{}, err
        }
        groups, err := p.groupPane(r, appCtx)
        return users, groups, err // renders Page
    }
}

// Plain helpers feeding both the partial branches and the full page.
// Only the method literally named Props is called by the framework.
func (p TeamManagementView) userPane(r *http.Request, appCtx *AppContext) (UserPaneProps, error) {
    q := r.FormValue("user-search")
    users, err := appCtx.Store.SearchUsersWithGroups(r.Context(), q)
    if err != nil {
        return UserPaneProps{}, fmt.Errorf("search users: %w", err)
    }
    return UserPaneProps{Users: users, UserSearchQuery: q}, nil
}

func (p TeamManagementView) groupPane(r *http.Request, appCtx *AppContext) (GroupPaneProps, error) {
    q := r.FormValue("group-search")
    groups, err := appCtx.Store.SearchGroupsWithCounts(r.Context(), q)
    if err != nil {
        return GroupPaneProps{}, fmt.Errorf("search groups: %w", err)
    }
    return GroupPaneProps{Groups: groups, GroupSearchQuery: q}, nil
}
```

```gsx
component (p TeamManagementView) Page(users UserPaneProps, groups GroupPaneProps) {
    <div class="team-management">
        <div class="user-pane">
            <input
                name="user-search"
                hx-get={TeamManagementView{} |> url}
                hx-target={TeamManagementView.UserList |> target}
            />
            <div id={TeamManagementView.UserList |> id}>
                <p.UserList pane={users}/>
            </div>
        </div>
        <div class="group-pane">
            <input
                name="group-search"
                hx-get={TeamManagementView{} |> url}
                hx-target={TeamManagementView.GroupList |> target}
            />
            <div id={TeamManagementView.GroupList |> id}>
                <p.GroupList pane={groups}/>
            </div>
        </div>
    </div>
}

component (p TeamManagementView) UserList(pane UserPaneProps) {
    // renders pane.Users, preserves pane.UserSearchQuery in the input
}

component (p TeamManagementView) GroupList(pane GroupPaneProps) {
    // renders pane.Groups
}
```

Each pane updates independently: typing in the user search box re-renders only `UserList`, with only the user query running. The composition site passes the same pane struct the partial branch builds — full render and partial re-render share one component signature. Each Props result fills at most one parameter, so give the panes distinct types.

If Props recognises a target with `Is` and then returns no error, that is still the normal flow: the component the target `Is` renders, or `Page`. Asking doesn't change what renders.

### Breaking the normal flow

To render something other than what the request wants, return `RenderComponent` with a constructed component — an empty state, an inline failure notice, another package's component:

```go
func (p search) Props(r *http.Request) ([]Result, error) {
    query := r.URL.Query().Get("q")
    if query == "" {
        return nil, structpages.RenderComponent(p.EmptyState())
    }
    return performSearch(query) // renders Results or Page, whichever the request wants
}
```

`RenderComponent(c, more...)` renders its arguments in order as one response, which is how one response updates several regions (htmx 4 `<hx-partial>`, htmx 2 `hx-swap-oob`). See [Breaking the flow on purpose](./rendering.md#breaking-the-flow-on-purpose) and [Updating several regions](./rendering.md#updating-several-regions).

### Standalone components shared across pages

A component (a standalone `component`) can be an htmx target without belonging to any page. Its id is package-prefixed and stable regardless of which pages embed it, and `target.Is(UserStatsWidget)` recognises a request for it. It isn't one of the page's components, so the normal flow would render `Page`: Props renders it with `RenderComponent`:

```gsx
component UserStatsWidget(stats UserStats) {
    <div id={UserStatsWidget |> id}>{ stats.ActiveUsers } active users</div>
}

func (p dashboardPage) Props(r *http.Request, target structpages.RenderTarget, store *Store) (DashboardProps, error) {
    if target.Is(UserStatsWidget) {
        stats, err := store.LoadUserStats(r.Context())
        if err != nil {
            return DashboardProps{}, err
        }
        return DashboardProps{}, structpages.RenderComponent(<UserStatsWidget stats={stats}/>)
    }
    return p.fullProps(r, store)
}
```

### Regions shared across pages

The same works for another page's component. When a page's markup reuses a region of another page, a request targeting it carries that page's generated id, and `t.Is(report.DetailPage.Content)` is true on the serving page — matched by the exact id `ID` and the `id` filter produce, never loosely. Props decides what to render; if it doesn't act, `Page` renders. See [Regions shared across pages](./rendering.md#regions-shared-across-pages), including the warning about a whole document landing in a region.

## Nested swap levels (Page → Content → Detail)

A page's page components can be composed into **nested swap levels**, each an independent HTMX target. The levels are *not* a tree the matcher walks — they're sibling page components on one page, each with its own id. Because `HX-Target` selects the page component whose generated id it names, targeting a given level re-renders *only* that level, even though `Page` composes `Content` composes `Detail`:

- **`Page`** — the full document. Cold loads and `hx-boost` body swaps. Composes the app layout around `Content`.
- **`Content`** — the page's main region. Holds the page chrome — heading, back-link, toolbar — around the inner level. Swapped on boosted nav between pages.
- **`Detail`** (or another inner name) — a region *inside* `Content` that must swap on its own. Holds **none** of the page chrome.

```gsx
component (d FooDetail) Page(props FooProps) {
    <Layout title={props.Foo.Name}>
        <main><d.Content props={props}/></main>
    </Layout>
}

component (d FooDetail) Content(props FooProps) {
    <div id={FooDetail.Content |> id}>
        <a href={FooList{} |> url}>&larr; Foos</a>
        <d.Detail props={props}/>
    </div>
}

component (d FooDetail) Detail(props FooProps) {
    <div id={FooDetail.Detail |> id}>
        // fields, actions — NO back-link, NO header
    </div>
}
```

**Why three levels, not two.** The trap is reusing `Content` as the swap fragment for an embedded region — e.g. a master-detail inspector pane hosting the standalone detail page's `Content`. That drags the page chrome into the pane. Splitting out `Detail` gives the embedded region a chrome-less partial while `Content` keeps the standalone-page chrome. **The level you embed/swap is the one with no chrome of its own.**

The rule generalizes: **one page component per independently-swappable region, outer wraps inner, embed/target the innermost that has no chrome above it.**

## Custom target selectors

The built-in selectors cover htmx. `HTMXRenderTarget` reads htmx 1.x/2.x headers (`HX-Request`, `HX-Target` as a bare id). For htmx 4 — which reshaped `HX-Target` to `"<tag>#<id>"` and added `HX-Request-Type` — use `HTMXv4RenderTarget`; it also wants `Page` for `HX-Request-Type: full` (boosted links, `hx-select`). Both match `HX-Target` against the served page's components by the generated id, the component name with or without the page prefix, or a suffix of it.

A selector can return any value implementing `RenderTarget` (`Is(component any) bool`). The framework only ever calls `Is`, so a custom target takes part in selection like the built-in ones: the page's component it `Is` renders, else `Page`. A custom selector typically handles its own case and delegates the rest to the htmx selector. A target that `Is` none of the page's components still reaches Props, which can type-assert it to read what it carries and break the flow with `RenderComponent`:

```go
type jsonTarget struct{ data any }

func (jsonTarget) Is(component any) bool { return false } // wants no page component

sp, err := structpages.Mount(mux, pages{}, "/", "App",
    structpages.WithTargetSelector(func(r *http.Request, pn *structpages.PageNode) (structpages.RenderTarget, error) {
        if r.Header.Get("Accept") == "application/json" {
            return jsonTarget{data: loadData(r, pn)}, nil
        }
        return structpages.HTMXRenderTarget(r, pn)
    }),
)

func (p dashboard) Props(w http.ResponseWriter, r *http.Request, sel structpages.RenderTarget) (dashboardProps, error) {
    if jt, ok := sel.(jsonTarget); ok {
        w.Header().Set("Content-Type", "application/json") // kept: text/html is only the default
        return dashboardProps{}, structpages.RenderComponent(jsonView(jt.data))
    }
    return p.fullProps(r)
}
```

`Is` must have no side effects: the framework asks it about every component of the page, and Props may ask it too. A target that `Is` more than one of the page's components is an error. The exact matching rules are in the [API reference](./api.md#htmxrendertarget).

## See also

- [How a Request Is Rendered](./rendering.md) for the full flow, including where parameter values come from.
- `examples/htmx`, `examples/todo`, and `examples/htmx-render-target` in the [repository](https://github.com/jackielii/structpages/tree/main/examples) for complete working code.
- [Error Handling](./error-handling.md) for HTMX-aware redirects (`HX-Location`).
