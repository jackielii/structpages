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

For appending query strings to a generated URL, the framework accepts a `[]any` slice as the page argument. There is no `join()` function — pass the slice directly. Use `map[string]any` for placeholders (recommended over positional or key/value-pair forms):

```go
url, err := structpages.URLFor(ctx,
    []any{MyPage{}, "?page={page}&q={q}"},
    map[string]any{"page": pageNum, "q": query},
)
```

Some apps wrap this into a small `join` helper to read more nicely:

```go
func join(parts ...any) []any { return parts }

// then:
structpages.URLFor(ctx, join(MyPage{}, "?page={page}"), map[string]any{"page": pageNum})
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
    if sel.Is(p.NcrTable) {
        p.loadTableData(r.Context(), appCtx.Store, filter, &props)
        return props, structpages.RenderComponent(p.NcrTable(props))
    }

    // Load full data (charts, totals, table)
    p.loadAllData(r.Context(), appCtx.Store, filter, &props)

    // HTMX partial: content area (filters + table + charts)
    if sel.Is(p.NcrContent) {
        return props, structpages.RenderComponent(p.NcrContent(props))
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
    case sel.Is(p.GroupList):
        groups, err := p.groupListData(r, appCtx)
        if err != nil { return TeamManagementProps{}, err }
        return TeamManagementProps{}, structpages.RenderComponent(p.GroupList(groups))

    case sel.Is(p.UserList):
        users, err := p.userListData(r, appCtx)
        if err != nil { return TeamManagementProps{}, err }
        return TeamManagementProps{}, structpages.RenderComponent(p.UserList(users))

    case sel.Is(p.Page), sel.Is(p.Content):
        users, err := p.userListData(r, appCtx)
        if err != nil { return TeamManagementProps{}, err }
        groups, err := p.groupListData(r, appCtx)
        if err != nil { return TeamManagementProps{}, err }
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
    if target.Is(p.TableView) {
        return structpages.RenderComponent(p.TableView(tableProps))
    }
    return structpages.RenderComponent(p.TablePage(tableProps))
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

Redirects go through the `Redirect` control-flow signal (see §13), never `http.Redirect` — an HTMX XHR follows a 3xx and swaps the target page's body into the partial's swap target:

```go
func (p EntityDeletePage) ServeHTTP(w http.ResponseWriter, r *http.Request, appCtx *AppContext) error {
    id := r.PathValue("entity_id")
    if err := appCtx.Store.DeleteEntity(r.Context(), id); err != nil {
        return err
    }
    listURL, err := structpages.URLFor(r.Context(), EntityListPage{})
    if err != nil { return err }
    return Redirect{To: listURL}
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
    structpages.WithErrorHandler(errorHandler), // see §13 for the status-aware version
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
                    // Middleware is outside the error-return path, so do the
                    // HTMX check here: a 3xx would be swapped into the partial.
                    loginURL, err := structpages.URLFor(r.Context(), LoginPage{})
                    if err != nil {
                        // http.Error is ok here because it's outside of structpages' error handling.
                        // This is a fallback for framework-level errors.
                        http.Error(w, "internal error", http.StatusInternalServerError)
                        return
                    }
                    if r.Header.Get("HX-Request") == "true" {
                        w.Header().Set("HX-Location", loginURL) // ajax navigation; status must stay 2xx
                        return
                    }
                    http.Redirect(w, r, loginURL, http.StatusSeeOther)
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

`RenderComponent` accepts several shapes. They fall into two groups: **direct construction** (no reflection, compile-time-checked) and **reflective dispatch** (framework looks up the method and applies DI). Prefer direct construction — page structs are stateless, so a zero-value receiver constructs another page's component too. Reach for reflective dispatch only when the method's parameters should be DI-injected by the framework.

### Preferred: direct construction

```go
// Same-page method — receiver is in scope, just call it.
return MyPageProps{}, structpages.RenderComponent(p.UserList(users))

// Another page's method — zero-value receiver works; pages are stateless.
return structpages.RenderComponent(MyPage{}.ItemList(items))

// Standalone function component — call it directly.
return MyPageProps{}, structpages.RenderComponent(UserStatsWidget(stats))

// Pre-built templ component captured in a variable.
comp := p.Dialog(entityType, entityID, users)
return nil, structpages.RenderComponent(comp)

// Render literally nothing.
return structpages.RenderComponent(templ.NopComponent)
```

### Reflective dispatch (when params need framework DI)

```go
// Method expression — framework finds the mounted page, DI-injects the
// method's params (e.g. *http.Request, *AppContext), and invokes it.
// Explicit args fill the non-injected params, checked at runtime.
return structpages.RenderComponent(MyPage.ItemList, items)

// Bound method expression — equivalent to the unbound form; useful when the
// receiver came from somewhere other than `p` (e.g. a parent's child field).
return structpages.RenderComponent(other.EditSection, props)

// Via RenderTarget — still works, but `RenderComponent(p.X(args))` is usually
// clearer when `p` is in scope. Required only if the target was produced by a
// custom TargetSelector and the call site genuinely doesn't know which method
// it refers to.
return MyPageProps{}, structpages.RenderComponent(sel, users)
```

### Extension point: custom `RenderTarget` with `Component()`

```go
// A custom TargetSelector can return a RenderTarget that also implements
// Component() — RenderComponent(target) will then call Component() directly.
type myTarget struct{ data string }
func (t myTarget) Is(method any) bool   { /* ... */ }
func (t myTarget) Component() component { return MyComponent(t.data) }
// Then: return Props{}, structpages.RenderComponent(target)  // no args
```

---

## 10. html/template Instead of templ

structpages is render-engine agnostic — any value with a `Render(ctx context.Context, w io.Writer) error` method works as a page output. The pattern below is what `examples/html-template/` demonstrates.

### Atomic-design layout

Slash-namespaced template names mirror the directory tree. Only `body` is reused (one per per-page parsed set):

```
templates/
  layout/public.html         {{ define "layout/public" }}
  ui/atoms/button.html       {{ define "ui/atoms/button" }}
  ui/molecules/card.html     {{ define "ui/molecules/card" }}
  post/comments-list.html    {{ define "post/comments-list" }}  (organism, HTMX-targetable)
  post/page.html             {{ define "body" }}                (page-specific)
  pages/home.html            {{ define "body" }}
```

### Renderable type + helpers

```go
//go:embed templates
var tmplFS embed.FS
var pageTmpls map[string]*template.Template // populated in main

type tpl struct {
    page  string
    entry string
    data  any
}

func (p tpl) Render(_ context.Context, w io.Writer) error {
    t, ok := pageTmpls[p.page]
    if !ok {
        return fmt.Errorf("unknown page %q", p.page)
    }
    return t.ExecuteTemplate(w, p.entry, p.data)
}

// args is a Hugo/Sprig-style helper for passing multiple inputs to a
// partial. Defined in user code (not provided by the framework).
func args(kv ...any) (map[string]any, error) {
    if len(kv)%2 != 0 {
        return nil, fmt.Errorf("args: odd number of arguments (%d)", len(kv))
    }
    m := make(map[string]any, len(kv)/2)
    for i := 0; i < len(kv); i += 2 {
        k, ok := kv[i].(string)
        if !ok {
            return nil, fmt.Errorf("args: key at position %d is %T", i, kv[i])
        }
        m[k] = kv[i+1]
    }
    return m, nil
}
```

### Parse in `main` after `Mount`

The key move: parse templates AFTER `Mount` so `urlFor` can close over `sp.URLFor`. The FuncMap is bound once and the same parsed `*template.Template` serves every request — no Clone, no per-render rebinding.

```go
func main() {
    mux := http.NewServeMux()
    sp, err := structpages.Mount(mux, root{}, "/", "App",
        structpages.WithTargetSelector(structpages.HTMXv4RenderTarget))
    if err != nil { log.Fatal(err) }

    funcs := template.FuncMap{
        "urlFor": func(name string, a ...any) (string, error) {
            return sp.URLFor(structpages.Ref(name), a...)
        },
        "args": args,
    }
    parseSet := func(body string) *template.Template {
        return template.Must(template.New("").Funcs(funcs).ParseFS(tmplFS,
            "templates/layout/public.html",
            "templates/ui/atoms/*.html",
            "templates/ui/molecules/*.html",
            "templates/post/*.html",
            "templates/"+body,
        ))
    }
    pageTmpls = map[string]*template.Template{
        "home": parseSet("pages/home.html"),
        "post": parseSet("post/page.html"),
    }

    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

Trade-off: `sp.URLFor` doesn't have access to per-request URL params extracted by structpages middleware, so this pattern works for routes whose URLs don't need request-bound params (top-level nav). For `/users/{id}`-style routes that need to generate URLs from the *current* request's path params, switch to ctx-bound funcs by Cloning inside `Render`:

```go
func (p tpl) Render(ctx context.Context, w io.Writer) error {
    base := pageTmpls[p.page]
    t, err := base.Clone()
    if err != nil { return err }
    t.Funcs(template.FuncMap{
        "urlFor": func(name string, a ...any) (string, error) {
            return structpages.URLFor(ctx, structpages.Ref(name), a...)
        },
    })
    return t.ExecuteTemplate(w, p.entry, p.data)
}
```

### Pages, Props, organisms

Page methods all return `tpl` with different `entry` names. Props loads once per request; the matched component method receives it as an argument.

```go
type postProps struct {
    Title    string
    Body     string
    Comments []string
}
type post struct{}

func (post) Props() postProps { /* load from store */ }

func (post) Page(p postProps) tpl {
    return tpl{page: "post", entry: "layout/public", data: p}
}
func (post) Main(p postProps) tpl {
    return tpl{page: "post", entry: "body", data: p}
}
// HTMX-targetable organism — name matches <section id="comments">
func (post) Comments(p postProps) tpl {
    return tpl{page: "post", entry: "post/comments-list", data: p.Comments}
}
```

`HTMXv4RenderTarget` resolves `HX-Target: section#comments` to the `Comments` method via kebab-cased name matching — same mechanism that works with templ.

### Templates

Atoms/molecules receive ad-hoc data via `args` (no framework helpers visible inside — pure presentation). Organisms get whatever data slice they need; `urlFor` is callable inside any template since the FuncMap is parse-time-bound.

```html
{{ define "layout/public" }}
<!DOCTYPE html>
<html><body>
  <nav><a hx-get="{{ urlFor "post" }}" hx-target="main">Post</a></nav>
  <main>{{ template "body" . }}</main>
</body></html>
{{ end }}

{{ define "body" }}
<h1>{{ .Title }}</h1>
{{ range .Recent }}
  {{ template "ui/molecules/card" (args "Title" .Title "Body" .Excerpt) }}
{{ end }}
{{ template "post/comments-list" .Comments }}
{{ end }}

{{ define "post/comments-list" }}
<section id="comments">
  <ul>{{ range . }}<li>{{ . }}</li>{{ end }}</ul>
</section>
{{ end }}
```

---

## 11. Search Picklist with Positional Args

Prefer a `map[string]any` — explicit and refactor-safe — over positional fills:

```go
<button hx-get={ structpages.URLFor(ctx,
    []any{SearchPicklist{}, "?field={field}&q={q}&page={page}"},
    map[string]any{
        "field": props.Field,
        "q":     props.Query,
        "page":  props.Page + 1,
    }) }>
    Load More
</button>
```

The `URLFor` argument forms (in order of detection):

- **Map** (recommended): a single `map[string]any` first arg. Refactor-safe and self-documenting.
- **Positional**: arg count exactly matches placeholder count. Brittle if placeholders are added or reordered.
- **Key-value pairs**: even arg count, all even-indexed args are strings, AND at least one matches a placeholder name. (E.g. `"id", 123, "slug", "x"`.) Equivalent to the map form but spread across positional args.
- **Auto-fill from request**: any unfilled placeholders that match the *current request's* path params get filled automatically.

---

## 12. Module-Owned Static Assets

When a feature package owns a chunk of CSS/JS/images, mount its file server **as a field on the same struct as its pages** instead of in a separate `mux.Handle` call. The whole module — pages and assets — wires up by one struct field on the root type, and the static URL prefix tracks the module's mount path automatically.

### The pattern

```go
// modules/profile/profile.go
package profile

import (
    "embed"
    "io/fs"
    "net/http"
)

// Root is what the root struct embeds with `route:"/profile Profile"`.
// Listing Assets here means /profile and /profile/static/* register
// together — no separate pub.Handle("/profile/static/", …) in main.
type Root struct {
    Me     mePage      `route:"GET /me Me"`
    View   viewPage    `route:"GET /{userID} Profile"`
    Assets staticFiles `route:"GET /static/{path...} Assets"`
}

//go:embed all:static
var staticFS embed.FS

// fs.Sub strips the leading "static/" so the request path resolves
// directly. Computed once at init.
var staticRoot = func() fs.FS {
    sub, err := fs.Sub(staticFS, "static")
    if err != nil {
        panic(err) // unreachable: directory is //go:embed'd above
    }
    return sub
}()

// staticFiles serves the embedded /static/ directory. The {path...}
// wildcard in the route tag captures everything after /profile/static/,
// so r.PathValue("path") IS the file path inside the embedded FS — no
// http.StripPrefix needed, no need for the handler to know it's mounted
// under /profile.
type staticFiles struct{}

func (staticFiles) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    http.ServeFileFS(w, r, staticRoot, r.PathValue("path"))
}
```

### Why `{path...}` and not a trailing slash

`route:"GET /static/"` would *look* right (Go ServeMux treats trailing-slash patterns as prefix matches), but structpages joins parent and child routes with `path.Join`, which strips trailing slashes. The resulting pattern becomes `GET /admin/static` — an exact match, not a prefix — and subpath requests get 404. **Always use `{path...}` for prefix subtrees.**

### Linking to an asset from a templ page

There is no `URLFor` for arbitrary asset filenames — assets aren't pages. Use a plain string in the template:

```templ
<link rel="stylesheet" href="/profile/static/profile.css"/>
<img src="/profile/static/avatar-default.svg" alt=""/>
```

The pattern eliminates the *handler-side* duplication (no second mount call in main). Link-side duplication (knowing the URL string) is a separate concern, typically handled by a build-time manifest (e.g. Vite/esbuild fingerprinting).

### Middleware applies to assets too

If the module has `Middlewares()` (e.g. `RequireAdmin`), it gates the static subtree as well — Assets is just another child of the page struct. Move Assets out of the gated struct (sibling instead of child) if you want public assets under a private module's URL space.

### Mounting in main

The root struct treats every module identically — pages and assets are bundled:

```go
type webPages struct {
    Home    home.Index   `route:"/{$} HIS"`
    Patient patient.Root `route:"/patient Patient"`
    Profile profile.Root `route:"/profile Profile"`  // brings /profile/static/* with it
}

// main.go:
structpages.Mount(pub, webPages{}, "/", "HIS",
    structpages.WithArgs(profiles),
)
// No separate pub.Handle("/profile/static/", ...) needed.
```

---

## 13. Error Handling in `ServeHTTP` and `Props`

The error-returning forms of `ServeHTTP` and every `Props` method run against a **buffered** `http.ResponseWriter`. When the method returns a non-nil error the framework **discards the buffer** and hands the error to the `WithErrorHandler` callback. This has three consequences that decide how you write handlers.

### Rule 1 — never call `http.Error` (or otherwise write `w`) in an error-returning handler

This AI-generated style is wrong:

```go
// ANTI-PATTERN — do not do this
func (Submit) ServeHTTP(w http.ResponseWriter, r *http.Request, svc *Service) error {
    if err := r.ParseForm(); err != nil {
        http.Error(w, "invalid form", http.StatusBadRequest)
        return nil
    }
    patient, err := svc.GetPatientByMRN(r.Context(), mrn)
    switch {
    case errors.Is(err, ErrNotFound):
        http.Error(w, "patient not found", http.StatusNotFound)
        return nil
    case err != nil:
        return fmt.Errorf("GetPatientByMRN: %w", err)
    }
    // ...
}
```

It is broken either way the control flow goes:

- `http.Error(w, …); return nil` — the write *does* land (the buffer flushes on `nil`), but it bypasses `WithErrorHandler` entirely: no consistent HTML/HTMX error page, no `HX-Retarget`, no tracing. The framework also thinks the handler *succeeded*.
- `http.Error(w, …); return err` — the buffer is **reset before the error handler runs**, so your `http.Error` write is silently thrown away. Pure dead code.

The error-returning handler's only job is to **return an error**. Rendering is the error handler's job.

### Rule 2 — for a specific status code, return a typed error

Define one error type that carries the status, and have the global handler inspect it with `errors.As`. This is the only thing that gives a handler control over the status code.

```go
// errors.go
type ErrorWithStatus struct {
    Status  int
    Title   string
    Message string
}

func (e ErrorWithStatus) Error() string {
    return fmt.Sprintf("Error %d: %s", e.Status, e.Title)
}
```

The corrected handler — no `w` writes, just typed returns:

```go
func (Submit) ServeHTTP(w http.ResponseWriter, r *http.Request, svc *Service) error {
    if err := r.ParseForm(); err != nil {
        return ErrorWithStatus{Status: http.StatusBadRequest, Title: "Bad request", Message: "invalid form"}
    }

    patient, err := svc.GetPatientByMRN(r.Context(), mrn)
    switch {
    case errors.Is(err, ErrNotFound):
        return ErrorWithStatus{Status: http.StatusNotFound, Title: "Not found", Message: "patient not found at this facility"}
    case errors.Is(err, authz.ErrDenied):
        return ErrorWithStatus{Status: http.StatusForbidden, Title: "Forbidden", Message: "patient at a different facility"}
    case err != nil:
        return fmt.Errorf("scheduling.book: GetPatientByMRN: %w", err) // plain error -> 500
    }
    // ... success: redirect to the detail page via the control-flow signal
    return Redirect{To: detailURL}
}
```

Redirects ride the same error-return path as a control-flow signal — **never call `http.Redirect` from a handler**: during an HTMX request the XHR follows the 3xx and swaps the redirect target's body into the partial's swap target. The signal type:

```go
// Redirect is control flow, not a real error — it implements error only to
// ride the error-return path, which is what unwinds the render flow without
// writing the ResponseWriter directly.
type Redirect struct{ To string }

func (Redirect) Error() string { return "redirect" }
```

The matching global handler, wired once at `Mount`:

```go
structpages.WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
    if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
        w.WriteHeader(499) // client closed request — expected, don't log as error
        return
    }
    var redir Redirect
    if errors.As(err, &redir) {
        if r.Header.Get("HX-Request") == "true" {
            // Ajax navigation, like a boosted link. The status must stay 2xx:
            // htmx does not process response headers on 3xx responses.
            w.Header().Set("HX-Location", redir.To)
            return
        }
        http.Redirect(w, r, redir.To, http.StatusSeeOther)
        return
    }
    status, title, message := http.StatusInternalServerError, "Server error", err.Error()
    var se ErrorWithStatus
    if errors.As(err, &se) {
        status, title, message = se.Status, se.Title, se.Message
    } else {
        slog.Error("unhandled error rendering page", "error", err, "path", r.URL.Path)
    }
    // One place that knows how to render: HTMX-aware retarget, AppShell vs bare page, tracing.
    renderHTTPError(w, r, status, title, message)
})
```

(Use `HX-Redirect` instead of `HX-Location` only when the destination genuinely needs a full browser load — a non-htmx endpoint, or a page with different `<head>` content/scripts. `HX-Location` also accepts a JSON object — `{"path": "...", "target": "..."}` — for finer swap control.)

`errors.As` unwraps, so `fmt.Errorf("...: %w", ErrorWithStatus{...})` still resolves to its status. A plain `error` (a wrapped DB failure, say) falls through to a logged 500 — exactly what you want for an unexpected fault.

### Rule 3 — API endpoints use the *no-error* `ServeHTTP` form

For endpoints that serve JSON (or any non-HTML response), do **not** use the error-returning form. Two reasons:

1. The error-returning form buffers the whole response in memory before anything reaches the client.
2. `WithErrorHandler` renders an **HTML** error page. An API client expects a JSON body or a bare status code, not an AppShell document.

Use signature #3 — `ServeHTTP(w, r, deps...)` with **no return value**. The framework hands it the raw `w` (no structpages buffering wrapper), and because no error flows back, you own status codes yourself — and the error *bodies*: a JSON API returns JSON errors. Don't reach for `http.Error`; its `text/plain` body is the wrong shape for an API client (the Rule 1 prohibition covers the buffered forms; here it's wrong for content-type reasons instead).

```go
type TrackTime struct{}

// No error return: direct unbuffered writes, framework's HTML error handler stays out of it.
func (TrackTime) ServeHTTP(w http.ResponseWriter, r *http.Request, appCtx *AppContext) {
    var body struct {
        ViewID    int64 `json:"view_id"`
        TimeSpent int32 `json:"time_spent"`
    }
    if err := json.UnmarshalRead(r.Body, &body); err != nil {
        writeJSONError(w, http.StatusBadRequest, "invalid request: "+err.Error())
        return
    }
    if err := appCtx.Store.UpdateTimeSpent(r.Context(), body.ViewID, body.TimeSpent); err != nil {
        writeJSONError(w, http.StatusInternalServerError, "update failed")
        return
    }
    w.WriteHeader(http.StatusOK)
}

// The API's single error shape, defined once:
func writeJSONError(w http.ResponseWriter, status int, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
```

### Rule 4 — for streaming (SSE), flush with `http.ResponseController`

Picking the no-return form is *not* enough to guarantee writes reach the client immediately: the `w` you get may still be wrapped by upstream middleware (observability, response-writer wrappers, etc.). For **truly guaranteed unbuffered delivery** — Server-Sent Events, progress streams — use `http.ResponseController`, which walks the `Unwrap()` chain to find a flusher and drains everything in its path.

This also means you *can* stream from the error-returning DI form: structpages' buffering wrapper implements `FlushError()` and `Unwrap()`, so `http.ResponseController.Flush()` pushes the buffer straight to the wire. That lets a handler validate-and-error-render up front (Rules 1–2), then commit to streaming:

```go
func (p EdmImportUpload) ServeHTTP(w http.ResponseWriter, r *http.Request, appCtx *AppContext) error {
    if err := r.ParseMultipartForm(32 << 20); err != nil {
        // still buffered here — render an HTML error partial and return
        return renderImportError(w, r, "Failed to parse upload form", err)
    }
    // ... more validation that returns errors ...

    // commit to streaming
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("X-Accel-Buffering", "no")

    rc := http.NewResponseController(w) // works through the buffered wrapper via FlushError/Unwrap
    fmt.Fprint(w, ": connected\n\n")
    rc.Flush()                          // drains the buffer to the client now

    for update := range progressChan {
        fmt.Fprintf(w, "event: progress\ndata: %s\n\n", update)
        rc.Flush()                      // each event reaches the client immediately
    }
    return nil
}
```

Once you've started flushing a stream, returning a non-nil error can no longer produce a clean error page (headers and body bytes are already on the wire) — send an `event: error` SSE frame instead and `return nil`.

### Which form to use

| Handler does…                                  | `ServeHTTP` signature              | Errors via                          |
|-------------------------------------------------|------------------------------------|-------------------------------------|
| Renders HTML / HTMX partial, may redirect       | `(w, r, deps...) error`            | `return ErrorWithStatus{…}` / `return err`; redirects via `return Redirect{To: …}` |
| Serves JSON / API (one-shot response)           | `(w, r, deps...)` *(no return)*    | write `w` directly with a JSON error body (`writeJSONError`) |
| Streams (SSE, progress)                         | either form, flush via `http.NewResponseController(w)` | SSE `event: error` frame, then `return nil` |

`Props` methods always follow the first row — they are buffered and their error flows to `WithErrorHandler`, so return `ErrorWithStatus{…}` for status-coded failures, never write `w`.

## 14. Validating URLs (no dangling URLs in production)

`structpages-lint` is the primary guard — it statically validates `URLFor`/`Ref` calls, params, and hard-coded routes in CI (see SKILL.md §3). For what static analysis can't see (URLs assembled from runtime data, refs behind dynamic dispatch), a boot-time inventory of `URLFor` calls kills the startup with the list of what's dangling — same dynamic as a database migration check:

```go
func validateURLs(sp *structpages.StructPages) error {
    var errs []error
    check := func(label string, gen func() (string, error)) {
        if _, err := gen(); err != nil {
            errs = append(errs, fmt.Errorf("%s: %w", label, err))
        }
    }
    check("components detail", func() (string, error) {
        return sp.URLFor([]any{componentsRoot{}, entryPage{}}, map[string]any{"slug": "sample"})
    })
    check("admin settings", func() (string, error) {
        return sp.URLFor(structpages.Ref("Admin.Settings"))
    })
    return errors.Join(errs...)
}
```

Call it from `main` after `Mount` (fail the boot) and from a one-line test (coverage in CI). For end-to-end assurance, an integration test that mounts the tree, renders real pages, and asserts expected `href`s in the body also catches call sites that bypass your helpers. Full runnable pattern: `examples/url-validation/` in the repo.
