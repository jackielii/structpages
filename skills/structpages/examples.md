# structpages Patterns and Examples

Markup is gsx with the `url`/`id`/`target` filters registered (SKILL.md, "gsx setup"). Go code in a
`.gsx` file may use element literals (`<Widget n={1}/>`); in a `.go` file call the generated function
instead. `AppContext`, `store` and `ui` stand for application code.

---

## 1. Page with independently refreshed regions

### Routes and props

```go
type DashboardPages struct {
    Ncr NcrAnalyticsPage `route:"/ncr-analytics NCR Analytics"`
}

type NcrDashboardProps struct {
    Filter NcrFilter
    Charts NcrCharts
    Table  NcrTableProps
}

type NcrTableProps struct {
    Filter NcrFilter
    Items  []db.NcrItem
    Page   int
    NPages int
}

func (p NcrAnalyticsPage) Props(r *http.Request, appCtx *AppContext, sel structpages.RenderTarget) (NcrDashboardProps, error) {
    filter := p.parseFilter(r)
    switch {
    case sel.Is(p.NcrTable):
        table, err := p.tableProps(r.Context(), appCtx.Store, filter)
        if err != nil {
            return NcrDashboardProps{}, err
        }
        return NcrDashboardProps{}, structpages.RenderComponent(p.NcrTable(table))
    case sel.Is(p.NcrContent):
        props, err := p.fullProps(r.Context(), appCtx.Store, filter)
        if err != nil {
            return NcrDashboardProps{}, err
        }
        return NcrDashboardProps{}, structpages.RenderComponent(p.NcrContent(props))
    default:
        return p.fullProps(r.Context(), appCtx.Store, filter)
    }
}
```

### Markup

```gsx
component (p NcrAnalyticsPage) Page(props NcrDashboardProps) {
    <DashboardLayout current="ncr-analytics">
        <p.Content props={props}/>
    </DashboardLayout>
}

component (p NcrAnalyticsPage) Content(props NcrDashboardProps) {
    <div class="flex gap-6">
        <form
            hx-get={NcrAnalyticsPage{} |> url}
            hx-target={NcrAnalyticsPage.NcrContent |> target}
            hx-trigger="change delay:300ms"
            hx-push-url="true"
        >
            <p.FilterSection filter={props.Filter}/>
        </form>
        <div id={NcrAnalyticsPage.NcrContent |> id}>
            <p.NcrContent props={props}/>
        </div>
    </div>
}

component (p NcrAnalyticsPage) NcrContent(props NcrDashboardProps) {
    <p.ChartSection charts={props.Charts}/>
    <div id={NcrAnalyticsPage.NcrTable |> id}>
        <p.NcrTable table={props.Table}/>
    </div>
}

component (p NcrAnalyticsPage) NcrTable(table NcrTableProps) {
    <table>…</table>
    <Pager current={table.Page} total={table.NPages} status={table.Filter.Status}/>
}
```

### Pagination links

A component builds the links from filters; no attribute map and no helper that has to unwrap errors.

```gsx
component Pager(current int, total int, status string) {
    <nav class="pager">
        { for n := 1; n <= total; n++ {
            <a
                href={[]any{NcrAnalyticsPage{}, "?page={page}&status={status}"} |> url(map[string]any{"page": n, "status": status})}
                hx-get={[]any{NcrAnalyticsPage{}, "?page={page}&status={status}"} |> url(map[string]any{"page": n, "status": status})}
                hx-target={NcrAnalyticsPage.NcrTable |> target}
                { if n == current { aria-current="page" } }
            >
                { n }
            </a>
        } }
    </nav>
}
```

---

## 2. Two panes refreshed independently

```go
type TeamManagementPages struct {
    View    TeamManagementView    `route:"/{$} Team Management"`
    AddUser TeamManagementAddUser `route:"POST /add Add User to Group"`
}

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

func (p TeamManagementView) Props(r *http.Request, appCtx *AppContext, sel structpages.RenderTarget) (TeamManagementProps, error) {
    switch {
    case sel.Is(p.GroupList):
        pane, err := p.groupPane(r, appCtx)
        if err != nil {
            return TeamManagementProps{}, err
        }
        return TeamManagementProps{}, structpages.RenderComponent(p.GroupList(pane))
    case sel.Is(p.UserList):
        pane, err := p.userPane(r, appCtx)
        if err != nil {
            return TeamManagementProps{}, err
        }
        return TeamManagementProps{}, structpages.RenderComponent(p.UserList(pane))
    default:
        users, err := p.userPane(r, appCtx)
        if err != nil {
            return TeamManagementProps{}, err
        }
        groups, err := p.groupPane(r, appCtx)
        if err != nil {
            return TeamManagementProps{}, err
        }
        return TeamManagementProps{UserPaneProps: users, GroupPaneProps: groups}, nil
    }
}

func (p TeamManagementView) userPane(r *http.Request, appCtx *AppContext) (UserPaneProps, error) {
    q := r.FormValue("user-search")
    users, err := appCtx.Store.SearchUsers(r.Context(), q)
    if err != nil {
        return UserPaneProps{}, fmt.Errorf("search users: %w", err)
    }
    return UserPaneProps{Users: users, UserSearchQuery: q}, nil
}
```

Each pane component takes only its pane struct and is wrapped where it is composed:

```gsx
component (p TeamManagementView) Content(props TeamManagementProps) {
    <section>
        <input
            name="user-search"
            value={props.UserSearchQuery}
            hx-get={TeamManagementView{} |> url}
            hx-target={TeamManagementView.UserList |> target}
            hx-trigger="input changed delay:300ms, refresh-users from:body"
        />
        <div id={TeamManagementView.UserList |> id}>
            <p.UserList pane={props.UserPaneProps}/>
        </div>
    </section>
    <section>
        <div id={TeamManagementView.GroupList |> id}>
            <p.GroupList pane={props.GroupPaneProps}/>
        </div>
    </section>
}

component (p TeamManagementView) UserList(pane UserPaneProps) {
    { for _, u := range pane.Users {
        <div>{ u.User.Name }</div>
    } }
}
```

An action that affects both panes asks the client to refresh them:

```go
func (TeamManagementAddUser) ServeHTTP(w http.ResponseWriter, r *http.Request, appCtx *AppContext) error {
    if err := appCtx.Store.AddUserToGroup(r.Context(), r.FormValue("email"), r.FormValue("group_id")); err != nil {
        return err
    }
    w.Header().Set("HX-Trigger", "refresh-groups, refresh-users")
    w.WriteHeader(http.StatusNoContent)
    return nil
}
```

---

## 3. ServeHTTP with RenderTarget (view modes)

```go
func (p IndexPage) ServeHTTP(w http.ResponseWriter, r *http.Request, appCtx *AppContext, sel structpages.RenderTarget) error {
    if r.FormValue("view") == "table" {
        rows, err := p.tableRows(r, appCtx)
        if err != nil {
            return err
        }
        if sel.Is(p.TableView) {
            return structpages.RenderComponent(p.TableView(rows))
        }
        return structpages.RenderComponent(p.TablePage(rows))
    }
    return p.renderCards(r, appCtx, sel)
}
```

```gsx
<a
    href={[]any{IndexPage{}, "?view={view}"} |> url(map[string]any{"view": "card"})}
    hx-target={IndexPage.CardContent |> target}
>Cards</a>
<a
    href={[]any{IndexPage{}, "?view={view}"} |> url(map[string]any{"view": "table"})}
    hx-target={IndexPage.TableView |> target}
>Table</a>
```

---

## 4. CRUD pages

```go
type EntityPages struct {
    List   EntityListPage   `route:"/{$} Entities"`
    Detail EntityDetailPage `route:"/{entityId} Entity"`
    Edit   EntityEditPage   `route:"/{entityId}/edit Edit Entity"`
    Delete EntityDeletePage `route:"DELETE /{entityId} Delete Entity"`
}

func (p EntityDetailPage) Props(r *http.Request, appCtx *AppContext) (EntityDetailProps, error) {
    entity, err := appCtx.Store.GetEntity(r.Context(), r.PathValue("entityId"))
    if errors.Is(err, store.ErrNotFound) {
        return EntityDetailProps{}, ErrorWithStatus{Status: http.StatusNotFound, Title: "Not found", Message: "No such entity"}
    }
    if err != nil {
        return EntityDetailProps{}, err
    }
    return EntityDetailProps{Entity: entity}, nil
}

func (p EntityDeletePage) ServeHTTP(w http.ResponseWriter, r *http.Request, appCtx *AppContext) error {
    if err := appCtx.Store.DeleteEntity(r.Context(), r.PathValue("entityId")); err != nil {
        return err
    }
    listURL, err := structpages.URLFor(r.Context(), EntityListPage{})
    if err != nil {
        return err
    }
    return Redirect{To: listURL} // see §13
}
```

```gsx
component (p EntityDetailPage) Content(props EntityDetailProps) {
    <a href={EntityListPage{} |> url}>&larr; Entities</a>
    <h1>{ props.Entity.Name }</h1>
    <a href={EntityEditPage{} |> url}>Edit</a>
    <button hx-delete={EntityDeletePage{} |> url} hx-confirm="Delete this entity?">Delete</button>
}
```

`EntityEditPage{} |> url` needs no params here: `{entityId}` is auto-filled from the current request.

---

## 5. Lazy-loaded region on its own route

```gsx
<div
    id={ListActionsPartial.Page |> id}
    hx-get={ListActionsPartial{} |> url(map[string]any{"entityType": entityType, "entityId": entityID})}
    hx-trigger="load, refresh-actions from:body"
    hx-target="this"
>
    Loading…
</div>
```

---

## 6. Mounting with options

```go
sp, err := structpages.Mount(mux, ui.TopPages{}, "/", "App",
    structpages.WithErrorHandler(errorHandler), // §13
    structpages.WithMiddlewares(loggingMiddleware, sessionMiddleware),
    structpages.WithTargetSelector(structpages.HTMXv4RenderTarget), // htmx 4 front end
    structpages.WithArgs(appCtx),
)
if err != nil {
    log.Fatal(err)
}
if err := validateURLs(sp); err != nil { // §14
    log.Fatal(err)
}
```

---

## 7. Middleware

```go
type RequiresAuth struct {
    Home IndexPage `route:"/{$} Home"`
}

func (RequiresAuth) Middlewares(appCtx *AppContext) []structpages.MiddlewareFunc {
    return []structpages.MiddlewareFunc{
        func(next http.Handler, pn *structpages.PageNode) http.Handler {
            return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                if appCtx.Sessions.User(r) != nil {
                    next.ServeHTTP(w, r)
                    return
                }
                loginURL, err := structpages.URLFor(r.Context(), LoginPage{})
                if err != nil {
                    // Middleware is outside the structpages error path.
                    http.Error(w, "internal error", http.StatusInternalServerError)
                    return
                }
                if r.Header.Get("HX-Request") == "true" {
                    w.Header().Set("HX-Location", loginURL) // must stay 2xx for htmx to act
                    return
                }
                http.Redirect(w, r, loginURL, http.StatusSeeOther)
            })
        },
    }
}
```

---

## 8. Components that forward attributes

A shared button forwards an attrs bag, so call sites put filters straight on it; errors still propagate.

```gsx
component PrimaryButton(children gsx.Node, attrs gsx.Attrs) {
    <button type="button" class="btn btn-primary" { attrs... }>{ children }</button>
}

component (p UsersPage) Toolbar() {
    <PrimaryButton
        hx-get={UserNewModal{} |> url}
        hx-target={ui.ModalSlot |> target}
    >+ New user</PrimaryButton>
}
```

A component that links to a page takes the URL (or the page value) as a parameter and sets it on the native
element:

```gsx
component BackLink(href string, label string) {
    <a class="back-link" href={href}>&larr; { label }</a>
}

<BackLink href={EntityListPage{} |> url} label="Entities"/>
```

---

## 9. RenderComponent forms in practice

```go
// Same page: receiver in scope.
return MyPageProps{}, structpages.RenderComponent(p.UserList(users))

// Another page: zero-value receiver, pages are stateless.
return structpages.RenderComponent(MyPage{}.ItemList(items))

// Standalone component (.gsx file).
return MyPageProps{}, structpages.RenderComponent(<UserStatsWidget stats={stats}/>)

// Nothing.
return structpages.RenderComponent(gsx.Text(""))

// Parameters DI-injected by the framework (e.g. *http.Request, *AppContext).
return structpages.RenderComponent(MyPage.ItemList)
```

---

## 10. html/template instead of gsx

Any type with `Render(ctx, io.Writer) error` is a component; `examples/html-template/` uses this shape:

```go
type tpl struct {
    page, entry string
    data        any
}

func (t tpl) Render(_ context.Context, w io.Writer) error {
    return pageTmpls[t.page].ExecuteTemplate(w, t.entry, t.data)
}

func (post) Page(p postProps) tpl     { return tpl{page: "post", entry: "layout/public", data: p} }
func (post) Comments(p postProps) tpl { return tpl{page: "post", entry: "post/comments-list", data: p.Comments} }
```

Parse templates after `Mount` so a template func can close over `sp`:

```go
funcs := template.FuncMap{
    "urlFor": func(name string, a ...any) (string, error) {
        return sp.URLFor(structpages.Ref(name), a...)
    },
}
```

`html/template` has no request context in a FuncMap bound at parse time, so this resolves without
auto-filled request params. For routes that need them, `Clone` the template inside `Render` and bind
`structpages.URLFor(ctx, ...)`. Element ids are hand-written here (`<section id="comments">`), which is why
gsx's `id`/`target` filters are preferred.

---

## 11. JavaScript, Alpine and htmx values

Filters inside `js`/`f` literal holes work when the literal is written on the native element:

```gsx
component (p Index) Canvas() {
    <div
        x-data="{ open: false }"
        @keydown.escape=js`htmx.ajax('GET', @{Index{} |> url}, @{Index.Canvas |> target})`
    >
        <button
            hx-get={Index{} |> url}
            hx-target=f`closest @{Index.PickerList |> target}`
            hx-vals=js`{"pane": @{Index.Canvas |> id}}`
        >Reload</button>
    </div>
}
```

A wrapper component that needs a JavaScript handler takes ordinary values and builds the literal on its
own element:

```gsx
component OpenButton(slot string, href string, children gsx.Node) {
    <button @click=js`htmx.ajax('GET', @{href}, @{slot})`>{ children }</button>
}

<OpenButton slot={Index.Canvas |> target} href={Detail{} |> url}>Open</OpenButton>
```

Assigning such a literal inside a `{{ }}` block fails to generate when a hole uses an error-returning
filter, because a Go statement has no error channel.

---

## 12. Module-owned static assets

Mount a module's file server as a field next to its pages, so `/profile` and `/profile/static/*` register
together and follow the module's mount path.

```go
package profile

type Root struct {
    Me     mePage      `route:"GET /me Me"`
    View   viewPage    `route:"GET /{userID} Profile"`
    Assets staticFiles `route:"GET /static/{path...} Assets"`
}

//go:embed all:static
var staticFS embed.FS

var staticRoot = func() fs.FS {
    sub, err := fs.Sub(staticFS, "static")
    if err != nil {
        panic(err) // the directory is embedded above
    }
    return sub
}()

type staticFiles struct{}

func (staticFiles) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    http.ServeFileFS(w, r, staticRoot, r.PathValue("path"))
}
```

- Use `{path...}`: `route:"GET /static/"` would be joined to an exact `GET /profile/static`.
- `r.PathValue("path")` is the file path, so no `StripPrefix` is needed.
- The module's `Middlewares` gate the assets too; make `Assets` a sibling of the gated struct for public
  assets.
- Asset files are not pages: link them with a plain path or a build manifest (e.g. Vite), not `url`.

---

## 13. Error handling

### Typed status errors and the redirect signal

```go
type ErrorWithStatus struct {
    Status  int
    Title   string
    Message string
}

func (e ErrorWithStatus) Error() string { return fmt.Sprintf("%d %s: %s", e.Status, e.Title, e.Message) }

// Redirect is control flow carried on the error path.
type Redirect struct{ To string }

func (Redirect) Error() string { return "redirect" }
```

### Handlers return, they don't write

```go
// Wrong: http.Error then return nil bypasses the error handler; then return err is discarded.
func (Submit) ServeHTTP(w http.ResponseWriter, r *http.Request, svc *Service) error {
    if err := r.ParseForm(); err != nil {
        http.Error(w, "invalid form", http.StatusBadRequest)
        return nil
    }
    // …
}

// Right.
func (Submit) ServeHTTP(w http.ResponseWriter, r *http.Request, svc *Service) error {
    if err := r.ParseForm(); err != nil {
        return ErrorWithStatus{Status: http.StatusBadRequest, Title: "Bad request", Message: "invalid form"}
    }
    patient, err := svc.GetPatientByMRN(r.Context(), r.FormValue("mrn"))
    switch {
    case errors.Is(err, ErrNotFound):
        return ErrorWithStatus{Status: http.StatusNotFound, Title: "Not found", Message: "patient not found"}
    case err != nil:
        return fmt.Errorf("book: %w", err) // logged 500
    }
    detailURL, err := structpages.URLFor(r.Context(), PatientPage{}, map[string]any{"patientId": patient.ID})
    if err != nil {
        return err
    }
    return Redirect{To: detailURL}
}
```

`Props` follows the same rule, for a different reason: its writer is not buffered, so a body write reaches
the client and the error handler's output is appended to it. Set headers there if needed, return errors,
and return `ErrSkipPageRender` only when `Props` deliberately wrote the whole response.

### The global handler

```go
structpages.WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
    if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
        w.WriteHeader(499) // client went away
        return
    }
    var redir Redirect
    if errors.As(err, &redir) {
        if r.Header.Get("HX-Request") == "true" {
            w.Header().Set("HX-Location", redir.To) // htmx ignores headers on 3xx
            return
        }
        http.Redirect(w, r, redir.To, http.StatusSeeOther)
        return
    }
    status, title, msg := http.StatusInternalServerError, "Server error", "Something went wrong"
    var se ErrorWithStatus
    if errors.As(err, &se) {
        status, title, msg = se.Status, se.Title, se.Message
    } else {
        slog.ErrorContext(r.Context(), "render failed", "error", err, "path", r.URL.Path)
    }
    w.WriteHeader(status)
    if r.Header.Get("HX-Request") == "true" {
        _ = ErrorPanel(title, msg).Render(r.Context(), w)
        return
    }
    _ = ErrorPage(status, title, msg).Render(r.Context(), w)
})
```

Use `HX-Redirect` instead of `HX-Location` only when the destination needs a full browser load.

### JSON endpoints: the no-error form

```go
func (TrackTime) ServeHTTP(w http.ResponseWriter, r *http.Request, appCtx *AppContext) {
    var body trackTimeRequest
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        writeJSONError(w, http.StatusBadRequest, "invalid request")
        return
    }
    if err := appCtx.Store.UpdateTime(r.Context(), body); err != nil {
        writeJSONError(w, http.StatusInternalServerError, "update failed")
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
```

### Streaming (SSE)

Validate first while still buffered, then flush through `http.ResponseController`:

```go
func (p ImportUpload) ServeHTTP(w http.ResponseWriter, r *http.Request, appCtx *AppContext) error {
    if err := r.ParseMultipartForm(32 << 20); err != nil {
        return ErrorWithStatus{Status: http.StatusBadRequest, Title: "Upload failed", Message: err.Error()}
    }
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    rc := http.NewResponseController(w)
    for update := range appCtx.Imports.Run(r.Context(), r.MultipartForm) {
        fmt.Fprintf(w, "event: progress\ndata: %s\n\n", update)
        if err := rc.Flush(); err != nil {
            return nil // client gone; headers are already sent
        }
    }
    return nil
}
```

Once bytes are flushed an error can no longer become an error page; send an `event: error` frame instead.

| Handler does | Signature | Errors via |
|---|---|---|
| HTML page or partial, may redirect | `(w, r, deps...) error` | `return ErrorWithStatus{…}`, `return err`, `return Redirect{…}` |
| JSON API | `(w, r, deps...)` | JSON error body written directly |
| SSE stream | either, flushed with `http.NewResponseController` | `event: error` frame after streaming starts |

---

## 14. Validating URLs at boot

`structpages-lint` covers static call sites. For URLs built from runtime data or behind dynamic dispatch,
resolve an inventory after `Mount` and fail the boot:

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

Call it from `main` and from a test. `examples/url-validation/` in the repository has the runnable version,
including an integration test that renders pages and asserts their `href`s.
