**Disclaimer: AI helped, but I typed most of the words**

Some background so this doesn't read as a drive-by hot take: I've been a React full-stack developer since Redux first came out over 11 years ago — my [GitHub repo](https://github.com/jackielii/simplest-redux-example) can vouch for me. About 2.5 years ago I read the Hypermedia Systems book, built a Slack-like chat app as a POC, then took the approach into medium-size green-field projects at work. Now we're pushing hard onto our team to switch from React + JSON API to Go + HTMX for all of our new projects.

Recent big project is an enterprise learning & knowledge platform (CRUD-heavy with analytics dashboards, XLSX import, email workflows, LLM embeddings for search, RBAC, SSO).

- ~207k lines of Go + ~72k lines of templ
- ~6,700 lines of client-side JS total. Alpine for dropdowns/tabs/modals, echarts for dashboards, that's basically it
- 34 npm packages total (incl. dev deps).
- 56 direct Go deps

**What 2.5 years of it actually gets you.** The meme guy makes three claims in [when-to-use-hypermedia](https://htmx.org/essays/when-to-use-hypermedia/) that I think are all true:

*Less complexity, fewer problems.* There's just less to go wrong. No client-side state to drift out of sync with the server. Server is the source of truth. We just transfer the state to the browser directly

*Refactoring is aggressively easy.* Because the stack is thin, the UI and the data change at the same time. I've done more than once almost whole repo rewrite because the client changed the requirements or data model.

*Best tool for the job, and off the treadmill.* Go serves, the browser renders HTML, and we stick to the basics. No need to keep up with the latest javascript framework. Write plain Go and templ, and use javascript when you need to. I don't shy away from JS, I use build systems. I take the good parts

Concrete example of that last point: hypermedia-first doesn't mean JS-free. For the highly interactive parts we still write real JavaScript components — our column filter dropdown is a SolidJS component compiled to a web component (via solid-element): multiple filter operators, multi-select with removable badges, and a server-filtered autocomplete that fetches options as you type. The server-rendered page just drops in a `<filter-dropdown>` tag with attributes for props, and when you apply the filter it hands off to HTMX so the filtering itself stays server-side. Solid is a great fit for these islands — compiles down small, no VDOM, doesn't try to take over the page. And the web component wrapper isn't just packaging: it's the lifecycle management. HTMX swaps DOM in and out all the time, and custom elements get `connectedCallback`/`disconnectedCallback` for free — so when a swap replaces an island, the browser tells the component it's gone and Solid's reactive scope is disposed and cleaned up. No leaked effects, no stale listeners, no manual teardown hooks to wire against htmx events.

Same philosophy for the build setup. In dev, Vite runs as a dev server: HMR, instant feedback on JS/CSS/Tailwind changes. In prod, Vite outputs fingerprinted assets + a manifest, and the Go binary embeds them via `go:embed` — so the deployable is still a single self-contained binary with far-future cache headers, and there's no Node anywhere in production. That's the recurring theme for us: take the best parts of the JS ecosystem (Vite's DX, Solid for islands, Tailwind), and keep them out of the runtime path.

Here's my actual hot take after 2.5 years. Most of complains about HTMX is probably that it doesn't scale. It's a complaint about hypermedia. It can be mostly solved by good templating and partial rendering support on the server side.

So I wrote a lib for this part. Standard `http.ServeMux` underneath, but routes are struct tags, so your whole site is one tree:

```go
type RequiresAuth struct {
    IndexPage       `route:"/{$}            Home"`
    NptPages        `route:"/npt            NPT"`
    EntityPages     `route:"/entities       Entities"`
    DashboardPages  `route:"/dashboard      Analytics"`
    RequiresAdmin   `route:"/admin          Admin"`
}

func (RequiresAuth) Middlewares(appCtx *AppContext) []structpages.MiddlewareFunc {
    return []structpages.MiddlewareFunc{RequireAuth(appCtx)}
}
```

Each page implements a `Props` method (dependency-injected, returns the view model) and a templ `Page()`.

```go
type TeamPage struct{}

// Page props compose per-pane structs; each partial takes only its own pane.
type TeamProps struct {
    UserPane
    GroupPane
}

func (p TeamPage) Props(r *http.Request, target structpages.RenderTarget, app *AppContext) (TeamProps, error) {
    switch {
    case target.Is(p.UserList): // typing in user search → load users only
        pane, err := p.userPane(r, app)
        if err != nil {
            return TeamProps{}, err
        }
        return TeamProps{}, structpages.RenderComponent(p.UserList(pane))

    case target.Is(p.GroupList): // typing in group search → load groups only
        pane, err := p.groupPane(r, app)
        if err != nil {
            return TeamProps{}, err
        }
        return TeamProps{}, structpages.RenderComponent(p.GroupList(pane))

    default: // cold load or boosted nav → everything
        return p.fullProps(r, app)
    }
}
```

```templ
templ (p TeamPage) Page(props TeamProps) {
    @AppShell() {
        <input name="user-search"
            hx-get={ structpages.URLFor(ctx, TeamPage{}) }
            hx-target={ structpages.IDTarget(ctx, TeamPage.UserList) }/>
        <div id={ structpages.ID(ctx, TeamPage.UserList) }>
            @p.UserList(props.UserPane)
        </div>
        // …group pane, same shape
    }
}
```

Cross-component updates are events, so components stay detached. When adding a user should refresh both panes, the write handler doesn't know about either — it just fires events: `w.Header().Set("HX-Trigger", "refresh-users, refresh-groups")`.

https://github.com/jackielii/structpages — The library is beta quality: the API has settled and it's been carrying medium to large production apps.

The one problem I haven't solved is the one Carson lists as a legitimate reason not to adopt hypermedia: team buy-in. We're starting a bigger project and the team wants React + Go JSON backend, mainly on the argument that AI agents generate React better (Gumroad's reason too). My experience is the opposite — agents thrive on this stack because of its simplicity. The generation will get better overtime with more components/layouts lands in the system. The architecture complexity is what really matters.
