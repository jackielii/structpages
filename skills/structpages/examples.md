# structpages Real-World Patterns & Examples

## Helpers used in this document

These helpers are app-level conveniences referenced throughout. They are NOT part of the framework — define them in your own package once if you want them.

```go
// Generic must — panics on error. Useful when you need a plain string in a context
// that doesn't accept (string, error), e.g. inside templ.Attributes.
func must[T any](v T, err error) T {
    if err != nil { panic(err) }
    return v
}
```

For appending query strings to a generated URL, the framework accepts a `[]any` slice as the page argument. There is no `join()` function — pass the slice directly:

```go
url, err := structpages.URLFor(ctx,
    []any{MyPage{}, "?page={page}&q={q}"},
    "page", pageNum, "q", query,
)
```

Some apps wrap this into a small `join` helper to read more nicely:

```go
func join(parts ...any) []any { return parts }

// then:
structpages.URLFor(ctx, join(MyPage{}, "?page={page}"), "page", pageNum)
```

---

## 1. Complete Page with HTMX Partials

This is the most common pattern: a page with multiple sections that can be independently updated via HTMX.

### Route Definition

```go
// ui/pages.go
type DashboardPages struct {
    NcrAnalyticsPage `route:"/ncr-analytics NCR Analytics"`
}
```

### Props with RenderTarget

```go
// ui/dashboard_ncr_pages.go
type NcrDashboardProps struct {
    Filter      NcrFilter
    TotalCounts NcrTotalCounts
    ChartData   []NcrChartPoint
    Items       []db.NcrItem
    Pagination  *PaginationProps
}

func (p NcrAnalyticsPage) Props(r *http.Request, appCtx *AppContext, sel structpages.RenderTarget) (NcrDashboardProps, error) {
    filter := p.parseFilter(r)
    var props NcrDashboardProps
    props.Filter = filter

    // HTMX partial: table only
    if sel.Is(NcrAnalyticsPage.NcrTable) {
        p.loadTableData(r.Context(), appCtx.Store, filter, &props)
        return props, structpages.RenderComponent(NcrAnalyticsPage.NcrTable, props)
    }

    // Load full data (charts, totals, table)
    p.loadAllData(r.Context(), appCtx.Store, filter, &props)

    // HTMX partial: content area (filters + table + charts)
    if sel.Is(NcrAnalyticsPage.NcrContent) {
        return props, structpages.RenderComponent(NcrAnalyticsPage.NcrContent, props)
    }

    // Full page render
    return props, nil
}
```

### Template with ID

```templ
templ (p NcrAnalyticsPage) Page(props NcrDashboardProps) {
    @DashboardLayout("ncr-analytics") {
        @p.Content(props)
    }
}

templ (p NcrAnalyticsPage) Content(props NcrDashboardProps) {
    <div class="flex gap-6">
        // Filter sidebar — targets the content area
        <form hx-get={ structpages.URLFor(ctx, NcrAnalyticsPage{}) }
              hx-target={ structpages.IDTarget(ctx, NcrAnalyticsPage.NcrContent) }
              hx-swap="innerHTML"
              hx-trigger="change delay:300ms"
              hx-push-url="true">
            @p.FilterSection(props)
        </form>

        // Content area with unique ID
        <div id={ structpages.ID(ctx, NcrAnalyticsPage.NcrContent) }>
            @p.NcrContent(props)
        </div>
    </div>
}

templ (p NcrAnalyticsPage) NcrContent(props NcrDashboardProps) {
    @p.ChartSection(props)
    @p.NcrTable(props)
}

templ (p NcrAnalyticsPage) NcrTable(props NcrDashboardProps) {
    <div id={ structpages.ID(ctx, NcrAnalyticsPage.NcrTable) }>
        // table content...
    </div>
}
```

### Pagination using `[]any` for query strings

```go
func (p NcrAnalyticsPage) buildPagination(filter NcrFilter, page, nPages int) *PaginationProps {
    return &PaginationProps{
        Page:   page,
        NPages: nPages,
        GetAttrs: func(ctx context.Context, pg int) (templ.Attributes, error) {
            url, err := structpages.URLFor(ctx,
                []any{NcrAnalyticsPage{}, "?page={page}"},
                "page", pg,
            )
            if err != nil {
                return nil, err
            }
            // Append additional filter params
            if filter.Status != "" {
                url += "&status=" + filter.Status
            }
            target, err := structpages.IDTarget(ctx, NcrAnalyticsPage.NcrTable)
            if err != nil {
                return nil, err
            }
            return templ.Attributes{
                "href":      url,
                "hx-get":    url,
                "hx-target": target,
                "hx-swap":   "outerHTML",
            }, nil
        },
    }
}
```

---

## 2. Team Management (Two-Pane with Independent Partials)

A complex page where each pane updates independently.

### Route & Props

```go
// Route
type TeamManagementPages struct {
    TeamManagementView    `route:"/{$}      Team Management"`
    TeamManagementAddUser `route:"POST /add Add User to Group"`
    // ...
}

// Props with RenderTarget for each pane.
// Note: userListData / groupListData are plain helper methods, NOT auto-resolved by the framework.
func (p TeamManagementView) Props(r *http.Request, appCtx *AppContext, sel structpages.RenderTarget) (TeamManagementProps, error) {
    switch {
    case sel.Is(TeamManagementView.GroupList):
        groups, err := p.groupListData(r, appCtx)
        if err != nil { return TeamManagementProps{}, err }
        return TeamManagementProps{}, structpages.RenderComponent(TeamManagementView.GroupList, groups)

    case sel.Is(TeamManagementView.UserList):
        users, err := p.userListData(r, appCtx)
        if err != nil { return TeamManagementProps{}, err }
        return TeamManagementProps{}, structpages.RenderComponent(TeamManagementView.UserList, users)

    case sel.Is(TeamManagementView.Page), sel.Is(TeamManagementView.Content):
        users, _ := p.userListData(r, appCtx)
        groups, _ := p.groupListData(r, appCtx)
        return TeamManagementProps{
            UserPaneProps:  UserPaneProps{Users: users},
            GroupPaneProps: GroupPaneProps{Groups: groups},
        }, nil
    }
    return TeamManagementProps{}, nil
}
```

### Helper Props Methods

Each section has a dedicated helper returning only its data. These are *just methods* — the framework only auto-invokes the method literally named `Props`.

```go
func (p TeamManagementView) userListData(r *http.Request, appCtx *AppContext) ([]UserWithGroups, error) {
    search := r.FormValue("user-search")
    return appCtx.Store.SearchUsers(r.Context(), search)
}

func (p TeamManagementView) groupListData(r *http.Request, appCtx *AppContext) ([]db.GroupWithCounts, error) {
    search := r.FormValue("group-search")
    return appCtx.Store.SearchGroups(r.Context(), search)
}
```

### Partial Templates

Each partial takes ONLY its specific data:

```templ
templ (p TeamManagementView) UserList(users []UserWithGroups) {
    <div id="user-list">
        for _, u := range users {
            <div>{ u.Name }</div>
        }
    </div>
}

templ (p TeamManagementView) GroupList(groups []db.GroupWithCounts) {
    <div id="group-list">
        for _, g := range groups {
            <div>{ g.Name }</div>
        }
    </div>
}
```

### POST Handler with HTMX Trigger

```go
func (p TeamManagementAddUser) ServeHTTP(w http.ResponseWriter, r *http.Request, appCtx *AppContext) error {
    email := r.FormValue("email")
    groupID := r.FormValue("group_id")
    if err := appCtx.Store.AddUserToGroup(r.Context(), email, groupID); err != nil {
        return err
    }
    // Trigger both panes to refresh via HTMX events
    w.Header().Set("HX-Trigger", "refresh-groups, refresh-users")
    w.WriteHeader(http.StatusNoContent)
    return nil
}
```

### Search inputs listen for refresh events

```templ
<input name="user-search"
       hx-get={ structpages.URLFor(ctx, TeamManagementView{}) }
       hx-target="#user-list"
       hx-trigger="keyup changed delay:300ms, refresh-users from:body" />
```

---

## 3. Index Page with View Mode Switching (ServeHTTP + RenderTarget)

When a page needs `ServeHTTP` but also supports HTMX partials, the DI form of `ServeHTTP` can take a `RenderTarget`:

```go
func (p *IndexPage) ServeHTTP(w http.ResponseWriter, r *http.Request, appCtx *AppContext, target structpages.RenderTarget) error {
    viewMode := r.FormValue("view")
    if viewMode == "table" {
        return p.renderTable(r, appCtx, target)
    }
    return p.renderCards(r, appCtx, target)
}

func (p IndexPage) renderTable(r *http.Request, appCtx *AppContext, target structpages.RenderTarget) error {
    tableProps, err := p.buildTableViewProps(r, appCtx)
    if err != nil { return err }
    if target.Is(IndexPage.TableView) {
        return structpages.RenderComponent(IndexPage.TableView, tableProps)
    }
    return structpages.RenderComponent(IndexPage.TablePage, tableProps)
}
```

View mode switching in templates:

```templ
<a href={ structpages.URLFor(ctx, []any{IndexPage{}, "?view={view}"}, "view", "card") }
   hx-target={ structpages.IDTarget(ctx, IndexPage.CardContent) }>
   Card View
</a>
<a href={ structpages.URLFor(ctx, []any{IndexPage{}, "?view={view}"}, "view", "table") }
   hx-target={ structpages.IDTarget(ctx, IndexPage.TableView) }>
   Table View
</a>
```

---

## 4. Entity CRUD Pages (Standard Pattern)

### Route Structure

```go
type EntityPages struct {
    EntityDetailPage `route:"/entity/{entity_id}        Entity Detail"`
    EntityEditPage   `route:"/entity/{entity_id}/edit   Entity Edit"`
    EntityDeletePage `route:"DELETE /entity/{entity_id} Delete Entity"`
}
```

### Detail Page

```go
func (p EntityDetailPage) Props(r *http.Request, appCtx *AppContext) (EntityDetailProps, error) {
    id := r.PathValue("entity_id")
    entity, err := appCtx.Store.GetEntity(r.Context(), id)
    if err != nil {
        return EntityDetailProps{}, err
    }
    return EntityDetailProps{Entity: entity}, nil
}

templ (p EntityDetailPage) Page(props EntityDetailProps) {
    @AppShellLayout() {
        if props.Entity == nil {
            @ErrorPage(404, "Not found", "Entity not found")
        } else {
            @p.Content(props)
        }
    }
}

templ (p EntityDetailPage) Content(props EntityDetailProps) {
    @PageHeaderWithBack(EntityListPage{}, "Back to List", props.Entity.Name, "Entity details")
    // detail content...
}
```

### Delete Handler (no HTML, redirect)

```go
func (p EntityDeletePage) ServeHTTP(w http.ResponseWriter, r *http.Request, appCtx *AppContext) error {
    id := r.PathValue("entity_id")
    if err := appCtx.Store.DeleteEntity(r.Context(), id); err != nil {
        return err
    }
    listURL, err := structpages.URLFor(r.Context(), EntityListPage{})
    if err != nil { return err }
    http.Redirect(w, r, listURL, http.StatusSeeOther)
    return nil
}
```

---

## 5. Lazy-Loaded Partials (Separate Routes)

For sections that load independently:

```templ
<div id={ structpages.ID(ctx, ListActionsPartial.Page) }
     hx-get={ structpages.URLFor(ctx, ListActionsPartial{}, "entity_type", entityType, "entity_id", entityID) }
     hx-trigger="load, refresh-actions from:body"
     hx-swap="morph:innerHTML"
     hx-target="this">
    Loading...
</div>
```

---

## 6. Mounting with Options

```go
sp, err := structpages.Mount(mux, ui.TopPages{}, "/", "App",
    structpages.WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
        log.Printf("error: %v", err)
        http.Error(w, "Something went wrong", http.StatusInternalServerError)
    }),
    structpages.WithMiddlewares(
        loggingMiddleware,
        sessionMiddleware,
        flashMiddleware,
    ),
    structpages.WithArgs(appCtx),  // DI: makes *AppContext available everywhere
)
if err != nil {
    log.Fatal(err)
}
appCtx.Pages = sp  // Store *StructPages for URL/ID generation outside request context
```

---

## 7. Middleware Patterns

### Auth middleware on a page group

`Middlewares` returns `[]structpages.MiddlewareFunc`. The signature is `func(http.Handler, *PageNode) http.Handler` — second arg gives middleware access to route metadata.

```go
type RequiresAuth struct {
    IndexPage `route:"/{$} Home"`
    // all children require auth
}

func (RequiresAuth) Middlewares(appCtx *AppContext) []structpages.MiddlewareFunc {
    return []structpages.MiddlewareFunc{
        func(next http.Handler, pn *structpages.PageNode) http.Handler {
            return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                if !isAuthenticated(r) {
                    http.Redirect(w, r, "/login", http.StatusSeeOther)
                    return
                }
                next.ServeHTTP(w, r)
            })
        },
    }
}
```

---

## 8. Common UI Patterns

### Button with HTMX attributes (uses `must` for plain string)

```templ
@PrimaryButton(templ.Attributes{
    "hx-get":    must(structpages.URLFor(ctx, UserNewModal{})),
    "hx-target": "#modal-container",
    "hx-swap":   "innerHTML",
}) {
    + New User
}
```

### Links using URLFor

```templ
@PrimaryButtonLink(DetailPage{}, item.ID) {
    View Details
}
```

### Query params with `[]any` in templ

```templ
<a href={ structpages.URLFor(ctx,
    []any{TeamManagementRemoveUser{}, "?email={email}&group_id={groupId}"},
    "email", user.Email,
    "groupId", group.ID) }>
    Remove
</a>
```

---

## 9. RenderComponent Variants

```go
// 1. Method expression (cross-page or same-page)
return structpages.RenderComponent(MyPage.ItemList, items)

// 2. Pre-built templ component (no args allowed when passing a component instance)
comp := p.Dialog(entityType, entityID, users)
return nil, structpages.RenderComponent(comp)

// 3. Via RenderTarget (after target.Is() matched)
return MyPageProps{}, structpages.RenderComponent(sel, users)

// 4. Render a literal nothing
return structpages.RenderComponent(templ.NopComponent)

// 5. Bound method expression on receiver (works the same as the unbound form)
return structpages.RenderComponent(p.EditSection, props)

// 6. Custom RenderTarget that implements Component() — framework calls Component()
//    automatically when no args are given. Useful for custom TargetSelectors.
type myTarget struct{ data string }
func (t myTarget) Is(method any) bool   { /* ... */ }
func (t myTarget) Component() component { return MyComponent(t.data) }
// Then: return Props{}, structpages.RenderComponent(target)  // no args
```

---

## 10. Search Picklist with Positional Args

Positional args fill placeholders left-to-right: `{field}` gets `props.Field`, `{q}` gets `props.Query`, `{page}` gets `props.Page+1`.

```go
<button hx-get={ structpages.URLFor(ctx,
    []any{SearchPicklist{}, "?field={field}&q={q}&page={page}"},
    props.Field, props.Query, props.Page+1) }>
    Load More
</button>
```

The `URLFor` argument forms (in order of detection):

- **Positional**: arg count exactly matches placeholder count.
- **Key-value pairs**: even arg count, all even-indexed args are strings, AND at least one matches a placeholder name. (E.g. `"id", 123, "slug", "x"`.)
- **Map**: a single `map[string]any` first arg.
- **Auto-fill from request**: any unfilled placeholders that match the *current request's* path params get filled automatically.
