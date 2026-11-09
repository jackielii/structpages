---
title: Guarding Fragment URLs
slug: /fragment-guard
sidebar_position: 5.6
---

# Guarding Fragment URLs

## The problem

> **How do I stop an unstyled fragment from rendering as a full page?**

A fragment is markup that only makes sense inside a page: a dialog, a list's rows, a details panel. htmx fetches it from its own URL and swaps it into the page that is already on screen, with that page's layout, styles and scripts around it.

But the URL is a real URL, and browsers open URLs as pages. That happens when someone:

- opens a link or button target in a new tab;
- bookmarks it, or copies it into a chat;
- clicks a notification or an email that links to it;
- presses Back or reload after a request pushed the fragment's URL to the address bar (`hx-push-url`);
- reaches it from a crawler or link preview.

Each time, the server answers the way it answers htmx: with the fragment alone. The browser shows a dialog's form or a list's rows on a blank page, with no layout, no styles, no navigation and no scripts. The page looks broken, and the reader can't get back into the app from it.

Fixing this route by route doesn't last. Each fragment has to check the request, choose a page to send the reader to, and keep its URL out of history, and every new fragment is one more place to forget. The checks are also easy to get subtly wrong: an `HX-Request` check, for instance, mistakes htmx's own history restore for a partial.

So fix it once, in one middleware. It knows from each route's `*PageNode` what kind of route it wraps, and from each request what kind of response the request wants. When a page load reaches a fragment, it sends the reader to the page that shows that fragment instead of rendering it. This page describes that pattern, the decisions behind it, and how to keep it enforced.

## The pattern

The pattern has five parts:

1. **[Route kinds](#route-kinds)** come from method shapes, so the guard needs no registry.
2. **[Request intent](#request-intent)** separates a page load from a partial.
3. **[Landing](#landing)** decides where a fragment's page load goes, without loading data.
4. **[Headers belong to the platform](#headers-belong-to-the-platform)**: history, refusals and caching are set in one place, not in each Props.
5. **[Enforcement](#enforcement)** keeps routes in shape as the app grows.

A complete guard follows [the parts](#the-guard), then [a route before and after](#before-and-after), and the [pitfalls](#pitfalls) are the mistakes that are easy to make on the way.

The [fragment-guard example](https://github.com/jackielii/structpages/tree/main/examples/fragment-guard) runs all of it: a project page whose task list, rename dialog, comments and mentions are fragments, one for each landing case. Run it with `go run .` in that directory and open the fragment links on its home page, in a new tab or in place. The snippets below come from it.

## Route kinds

[Route Conventions](./route-conventions.md) sets the rule: **Props decides what to show; ServeHTTP decides what to do.** The guard reads it from the `*PageNode` a [`MiddlewareFunc`](./middleware.md) receives at mount time:

| Route struct | Kind | The guard |
|---|---|---|
| Has `Page` (`pn.Components["Page"]`) | page | passes it through; sets `Vary` and caches partials as private |
| No `Page`, has `Props` (`pn.Props != nil`) returning `RenderComponent` | fragment | lands page loads elsewhere; serves partials |
| `ServeHTTP(w, r, ...) error` | action | passes it through. A GET action only redirects or fails |
| `ServeHTTP` plus a marker of your own | protocol | passes it through: downloads, JSON, streams, callbacks |

Two conventions make the shapes reliable. Never name a fragment's component `Page`, and give each HTTP method its own route type ([one route type per method](./route-conventions.md#one-route-type-per-method)). A fragment's GET is then always a `Props`, and nothing that writes ever hides behind it.

## Request intent

A request is a **page load** (it wants a document) when it is any of:

- **native**: `Sec-Fetch-Mode: navigate`, or `Sec-Fetch-Dest` of `document`, `iframe` or `frame`. This wins over any htmx headers it also carries, which a browser can resend on reload;
- **an htmx history restore**: `HX-History-Restore-Request: true`. htmx fetches the page itself when its history cache misses;
- **an htmx request that swaps the whole body**: `HX-Target` of `body` or `body#id` (htmx 4), or no `HX-Target` with `HX-Boosted: true` (htmx 2, whose body has no id);
- **not an htmx request**: no `HX-Request: true`. That covers curl, tests and crawlers.

Every other htmx request is a **partial**: it wants part of a page, and the fragment answers it.

| Request (GET or HEAD) | Intent | Fragment's response |
|---|---|---|
| native | page load | 303 to the landing |
| htmx history restore | page load | 303 to the landing. htmx 2's restore request follows redirects but reads no response headers |
| htmx, swaps the body | page load | `HX-Location` to the landing |
| htmx, anything else | partial | the fragment |
| no htmx headers | page load | 303 to the landing |

Two rules keep the classification honest:

- **`HX-Request-Type: full` alone is not a page load.** htmx 4 sends `full` for body targets, but also for any `hx-select` swap into an element. A guard that lands every `full` request navigates away when a dialog loads with `hx-select`. The target says what is being replaced, so ask the target.
- **Classify reads only.** For a POST, PATCH or DELETE, any htmx request is an htmx request, and your error handler should answer it as one (a swap, a retarget). Writes pass through the guard.

## Landing

When a page load reaches a fragment, the guard sends the browser to the page that shows the fragment in context: its **landing**. The guard never loads data and never runs the fragment's Props, so it can only choose from what the request already says. Three cases cover every fragment.

### Default: the nearest ancestor page

Nothing to declare. The guard walks up the route tree to the nearest ancestor with `Page`, or to a group's exact `/{$}` index page, and builds its URL with `URLFor` from the request's path parameters. Group a page with its fragments and the landing follows:

```go
type pages struct {
    home     homePage     `route:"GET /{$} Home"`
    projects projectPages `route:"/projects/{project}"`
    comments comments     `route:"GET /comments Comments"`
    mention  mention      `route:"GET /mentions/{id} Mention"`
}

type projectPages struct {
    project    projectPage  `route:"GET /{$} Project"`
    tasks      taskList     `route:"GET /tasks Tasks"`
    rename     renameDialog `route:"GET /tasks/{task}/rename Rename task"`
    saveRename saveRename   `route:"POST /tasks/{task}/rename Rename task"`
}
```

A page load of `/projects/42/tasks` or `/projects/42/tasks/7/rename` lands on `/projects/42/`. The fragment needs no page-load branch:

```go
func (p taskList) Props(r *http.Request, store *Store) error {
    // Only a partial reaches this: the guard lands page loads.
    projectID := r.PathValue("project")
    if _, err := store.Project(projectID); err != nil {
        return err
    }
    return structpages.RenderComponent(p.List(projectID, store.Tasks(projectID)))
}
```

An ancestor whose path parameters the request can't fill is skipped, and the walk goes on up.

### `Landing`: built from the request

When the ancestor is the wrong page but the request still says where to go, the fragment implements a `Landing` method. It builds the destination from the path and query alone, with no lookup: a page, its parameters, a query and an anchor.

```go
type comments struct{} // GET /comments, off the root

// Landing opens the project page on the task's comments. It reads only the
// query: no lookup, so it can't reveal anything the URL didn't say.
func (comments) Landing(r *http.Request) (string, error) {
    q := r.URL.Query()
    return structpages.URLFor(r.Context(), []any{projectPage{}, "?task={task}#comments"},
        map[string]any{"project": q.Get("project"), "task": q.Get("task")})
}
```

`/comments?project=42&task=7` lands on `/projects/42/?task=7#comments`, where the page loads the task's comments. The guard calls `Landing` before any handler runs, so Props never sees a page load. A `Landing` that can't build its URL returns the error; it should fail visibly, not pick some other page.

### `SelfLanding`: the landing needs data

Sometimes only a lookup knows where a fragment belongs. A mention's URL names the mention, but the task it is on, and whether this reader may see it, are in the database. The guard never makes that lookup, so the fragment declares that it lands itself. The guard then lets its page loads through, and Props handles both kinds of request:

```go
type mention struct{} // GET /mentions/{id}

// LandsItself tells the guard to let page loads through.
func (mention) LandsItself() {}

func (p mention) Props(w http.ResponseWriter, r *http.Request, store *Store) error {
    m, err := store.Mention(r.PathValue("id")) // the access check: not theirs → not found
    if err != nil {
        return err
    }
    if !Partial(r) {
        to, err := structpages.URLFor(r.Context(), []any{projectPage{}, "?task={task}#comments"},
            map[string]any{"project": m.ProjectID, "task": m.TaskID})
        if err != nil {
            return err
        }
        http.Redirect(w, r, to, http.StatusSeeOther)
        return structpages.ErrSkipPageRender
    }
    c, err := store.Comment(m.CommentID)
    if err != nil {
        return err
    }
    return structpages.RenderComponent(p.Card(m, c))
}
```

`Partial` is the guard's own classification. Don't test `HX-Request` by hand: it is true for a history restore, which wants a document.

### Choosing

| The page that shows the fragment | Use |
|---|---|
| is an ancestor, with parameters from the path | nothing: the default |
| is another page, built from the path or query | `Landing` |
| depends on a lookup or an access check | `SelfLanding` |

**Fragments never branch on page load otherwise.** Only a `SelfLanding` fragment may test intent; in any other fragment the branch is dead code, because the guard has already answered. Prefer the default or `Landing` whenever they give the same page as a lookup would, and keep `SelfLanding` rare.

### Fallback and response

When no case applies (no override, and no ancestor page whose parameters the request can fill), answer with your app's full-page 404, with a link to the section the URL belongs to. Don't render the fragment.

The landing reaches the browser in the form the request can follow:

- a native request or an htmx history restore gets **303 See Other**, so the address bar names the page;
- an htmx request that swaps the body gets **`HX-Location`**, which htmx follows as a boosted navigation. If your htmx version lets you answer history restores this way too, make that navigation replace the history entry rather than push one, so the stale fragment URL doesn't stay behind the page.

Path parameters carry over through `URLFor`. Query strings don't, unless a `Landing` maps them, and an anchor never reaches the server, so only a `Landing` can add one.

## Access comes after landing

The guard lands a page load before the fragment's Props runs, so before the fragment's own access check. A visitor who may not see the record gets the same 303 as one who may. The landing page then applies its own authorization.

This is deliberate: the guard can land without loading anything. It has two consequences:

- **The landing must not reveal the record.** It only repeats what the fragment's URL already said (the ids in its path), and the destination checks access as usual. An override must not name anything a stranger couldn't already see.
- **Test a fragment's access with the partial request** (`HX-Request: true` and an `HX-Target`), not a plain GET. A plain GET only ever sees the redirect, so a test asserting 404 on it fails for the wrong reason.

Signing in should return to the landing, not the fragment URL. Put the guard before your sign-in middleware, or resolve the landing first and hand it to sign-in as the return address. Otherwise a signed-out visitor signs in and is sent back to bare markup.

## Headers belong to the platform

A fragment's Props decides what to show. The response's history, status and cache headers are decided once, in the guard and the error handler, so no route has to remember them.

### History

The guard sets `HX-Push-Url: false` on every fragment response. A fragment URL never reaches the address bar, whatever `hx-push-url` the requesting element says, so Back and reload never land on one.

A fragment that really does change the address (a dialog whose state deserves a URL) should push a page URL, never its own. Give it a typed helper, such as `Push(ctx, url)` or `Replace(ctx, url)`, that records the intent in the request context. The writer then applies it when the response commits, after checking that the URL names a GET page. A header set by hand can name anything, and nothing checks it.

On the client side, use `hx-push-url="true"` only for requests to page URLs, such as a nav link that swaps `#content` from a page's URL.

### Outcomes

Props reports what happened, and one error handler turns it into status and swap headers. Make the outcomes types:

- a **refusal**, such as an invalid form, carries its status (422), a message and the markup to show, and the handler sets the status and retargets the swap;
- **nothing to render** becomes 204;
- a quiet **401** for an expired session.

A Props that calls `w.WriteHeader` or sets `HX-Reswap` itself writes a contract that every module then implements differently. See [Error Handling](./error-handling.md) for the error handler.

### Caching

Each URL now has several representations: the document, the partial, the redirect. The guard keeps caches from mixing them.

- **`Vary`** on `HX-Request`, `HX-Target`, `HX-Request-Type`, `HX-Boosted`, `HX-History-Restore-Request`, `Sec-Fetch-Mode` and `Sec-Fetch-Dest`, on every rendering route. Merge the tokens into whatever `Vary` other middleware set, so each appears once.
- **`Cache-Control: private, no-store`** on fragment responses, page partials and the guard's own redirects, by default. A fragment whose successful partial is safe to keep, such as a notification count, opts in with a private max-age. It sets the header just before returning `RenderComponent`, on the success path only: Props' writer isn't buffered, so a header set earlier stays on an error response.
- **No validators on partials.** Remove `If-None-Match` and `If-Modified-Since` from partial requests before the handler sees them. A browser keeps one cache entry per URL, so its ETag may belong to the document. A 304 would then swap the document into the page, or the page's last partial into a dialog.

`Vary` protects HTTP caches, not the back-forward cache, which stores whole documents. A document whose address is a fragment URL can only be fixed by a reload, which the guard then lands. That is why keeping fragment URLs out of history matters as much as the guard.

## Enforcement

The guard trusts method shapes, so keep the shapes right.

- **Lint.** Static checks, such as a `go/analysis` pass, over every registered GET:
  - a fragment's Props returns only `error`, and no fragment has a component named `Page`;
  - a GET `ServeHTTP` writes no markup;
  - only `SelfLanding` fragments test intent;
  - Props doesn't set history or status headers;
  - nothing switches on `r.Method`;
  - every pushed URL names a page.

  Adopt it with a baseline that may only shrink, and a reviewed list of exceptions. The [structpages linter](./lint.md) covers the library's own rules; these are app rules on top.
- **Route walk.** A test that mounts the app and loads every GET as a document (`Sec-Fetch-Mode: navigate`, no htmx headers). It follows same-origin redirects to a limit, and fails only on what is certainly wrong:
  - bare markup with no document;
  - a redirect loop, or a redirect off the origin;
  - a history control (`hx-push-url`, `hx-replace-url`) that names a non-page.

  A route without a fixture gets placeholder parameters, and any full document it ends on passes, even a 404.

Lint is the main railing, because it checks each route as it is written. The walk is the backstop that catches what lint can't see.

## The guard

This is the guard from the [fragment-guard example](https://github.com/jackielii/structpages/tree/main/examples/fragment-guard) (`guard.go`). The [htmx example](https://github.com/jackielii/structpages/tree/main/examples/htmx) runs the same file as `navguard.go`, and its `navguard_test.go` drives each request kind through it. It implements intent, landing (default, `Landing`, `SelfLanding`), `HX-Push-Url: false`, `Vary`, no-store and validator stripping. Typed history helpers and outcome types live in your app's response writer and error handler, so they aren't shown.

```go
func NavigationGuard(next http.Handler, pn *structpages.PageNode) http.Handler {
    _, hasPage := pn.Components["Page"]
    fragment := !hasPage && pn.Props != nil
    if !hasPage && !fragment {
        return next // ServeHTTP pages: actions, redirects, downloads, JSON
    }
    _, landsItself := pn.Value.Interface().(SelfLanding)
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        addVary(w.Header(), varyHeaders...)
        if r.Method != http.MethodGet && r.Method != http.MethodHead {
            next.ServeHTTP(w, r) // writes: any htmx request stays an htmx request
            return
        }
        intent := requestIntent(r)
        if intent == partial || fragment {
            // The default. A fragment whose successful response may be
            // cached privately sets its own Cache-Control in Props, just
            // before returning RenderComponent.
            w.Header().Set("Cache-Control", "private, no-store")
        }
        if intent == partial {
            // A partial is never answered 304: the browser's copy may be
            // another representation of the same URL.
            r = r.Clone(r.Context())
            r.Header.Del("If-None-Match")
            r.Header.Del("If-Modified-Since")
            if fragment {
                // A fragment URL never reaches the address bar, whatever
                // hx-push-url the element says.
                w.Header().Set("HX-Push-Url", "false")
            }
        }
        if !fragment || intent == partial || landsItself {
            next.ServeHTTP(w, r)
            return
        }
        // Redirect before Props runs, so before the fragment's own access
        // checks: the landing page applies its own.
        to, err := landingURL(r, pn)
        if err != nil {
            http.NotFound(w, r) // render your app's error page here
            return
        }
        if intent == wholePage {
            w.Header().Set("HX-Location", to)
            return
        }
        http.Redirect(w, r, to, http.StatusSeeOther)
    })
}

var varyHeaders = []string{
    "HX-Request", "HX-Target", "HX-Request-Type", "HX-Boosted",
    "HX-History-Restore-Request", "Sec-Fetch-Mode", "Sec-Fetch-Dest",
}

// addVary adds tokens to the Vary header once each, keeping what other
// middleware already put there.
func addVary(h http.Header, tokens ...string) {
    var all []string
    for _, v := range h.Values("Vary") {
        for _, t := range strings.Split(v, ",") {
            if t = strings.TrimSpace(t); t != "" {
                all = append(all, t)
            }
        }
    }
    for _, t := range tokens {
        if !slices.ContainsFunc(all, func(s string) bool { return strings.EqualFold(s, t) }) {
            all = append(all, t)
        }
    }
    h.Set("Vary", strings.Join(all, ", "))
}

type intent int

const (
    document  intent = iota // a browser navigation, curl, a test: wants a document
    wholePage               // an htmx request that swaps the whole body (boosted links)
    partial                 // an htmx request for part of a page
)

// requestIntent reads what kind of response a read is for. Anything that is
// not an htmx request counts as a document.
func requestIntent(r *http.Request) intent {
    switch r.Header.Get("Sec-Fetch-Dest") {
    case "document", "iframe", "frame":
        return document
    }
    // htmx 2 restores history on a cache miss with its own request, which
    // follows redirects but reads no response headers: send it a 303.
    if r.Header.Get("Sec-Fetch-Mode") == "navigate" ||
        r.Header.Get("HX-History-Restore-Request") == "true" ||
        r.Header.Get("HX-Request") != "true" {
        return document
    }
    if targetsBody(r) {
        return wholePage
    }
    // Everything else is a partial, including htmx 4's HX-Request-Type: full,
    // which it also sends for any hx-select swap into an element.
    return partial
}

// Partial reports whether a read asks for part of a page. Only a
// SelfLanding fragment needs it: the guard has already answered every other
// fragment's page loads.
func Partial(r *http.Request) bool { return requestIntent(r) == partial }

// targetsBody reports whether an htmx request swaps the whole body: htmx 4
// sends HX-Target "body" or "body#id"; htmx 2 sends the target's id, so a
// boosted request into an id-less body has no HX-Target.
func targetsBody(r *http.Request) bool {
    target := r.Header.Get("HX-Target")
    if tag, _, _ := strings.Cut(target, "#"); tag == "body" {
        return true
    }
    return target == "" && r.Header.Get("HX-Boosted") == "true"
}

// Landing is implemented by a fragment whose hosting page is not an
// ancestor, to name where a document navigation should go instead. It
// builds the URL from the request alone: no lookups, no access checks.
type Landing interface {
    Landing(r *http.Request) (string, error)
}

// SelfLanding marks a fragment whose landing page needs a lookup or an
// access check, which the guard never makes. The guard lets its page loads
// through, and its Props redirects when !Partial(r).
type SelfLanding interface {
    LandsItself()
}

var errNoLanding = errors.New("no page hosts this fragment")

// landingURL returns where a document navigation to fragment pn goes: its
// Landing, else the nearest ancestor that is a page (or a group's index
// page) whose URL the request's path parameters can fill.
func landingURL(r *http.Request, pn *structpages.PageNode) (string, error) {
    if l, ok := pn.Value.Interface().(Landing); ok {
        return l.Landing(r)
    }
    for a := pn.Parent; a != nil; a = a.Parent {
        for _, n := range append([]*structpages.PageNode{a}, a.Children...) {
            if n != a && strings.Trim(n.Route, "/") != "{$}" {
                continue // only a itself, or its index child
            }
            if !isGETPage(n) {
                continue
            }
            to, err := structpages.URLFor(r.Context(), func(c *structpages.PageNode) bool { return c == n })
            if err == nil {
                return to, nil
            }
        }
    }
    return "", errNoLanding
}

func isGETPage(pn *structpages.PageNode) bool {
    _, hasPage := pn.Components["Page"]
    return hasPage && (pn.Method == http.MethodGet || pn.Method == "ALL")
}
```

Register it with your other middleware. The first middleware in `WithMiddlewares` is the outermost:

```go
sp, err := structpages.Mount(mux, index{}, "/", "App",
    structpages.WithTargetSelector(structpages.HTMXRenderTarget),
    structpages.WithMiddlewares(NavigationGuard, RequireSignIn),
)
```

`URLFor` works inside middleware: structpages puts the route tree and the request's path parameters in the context before any middleware runs. So `/items/{id}/edit` lands on `/items/{id}` with the same `id`, and a predicate that compares nodes (`c == n`) finds exactly the ancestor the guard chose.

Adapt it where your app differs:

- **Errors.** Replace `http.NotFound` with your full-page 404 that links to the section.
- **Protocols.** If a protocol route has `Props`, or you want them named explicitly, check your protocol marker first and return `next`.
- **Authentication.** With the guard outside sign-in, as above, sign-in returns to the landing. If sign-in must run first, resolve the landing there.

## Before and after

Here is the rename dialog from the example, first as it tends to be written without the pattern, then with it. Before, one route type serves both the dialog and the save, and handles page loads, history and status itself:

```go
type rename struct{} // route:"/projects/{project}/tasks/{task}/rename": any method

func (p rename) ServeHTTP(w http.ResponseWriter, r *http.Request, store *Store) error {
    if r.Header.Get("HX-Request") != "true" {
        // Someone opened the dialog's URL: send them to the project.
        http.Redirect(w, r, "/projects/"+r.PathValue("project"), http.StatusSeeOther)
        return nil
    }
    w.Header().Set("HX-Push-Url", "false")
    task, err := store.Task(r.PathValue("project"), r.PathValue("task"))
    if err != nil {
        return err
    }
    if r.Method != http.MethodPost {
        return structpages.RenderComponent(p.Dialog(task, task.Title, ""))
    }
    title := strings.TrimSpace(r.PostFormValue("title"))
    if title == "" {
        w.WriteHeader(http.StatusUnprocessableEntity)
        return structpages.RenderComponent(p.Dialog(task, title, "A task needs a title."))
    }
    if err := store.RenameTask(task.ProjectID, task.ID, title); err != nil {
        return err
    }
    return structpages.RenderComponent(taskList{}.Refreshed(task.ProjectID, store.Tasks(task.ProjectID)))
}
```

It works, mostly. But each line in it is a decision this route made alone:

- **The page-load check is wrong for history.** An htmx 2 history restore sends `HX-Request: true`, so on Back after a pushed dialog URL, it renders the bare dialog. A boosted link gets the dialog too.
- **The landing is a string**, built by hand. It breaks silently when the project route moves.
- **Access order is per route.** Here the redirect happens to run before the lookup. A fragment written the other way round answers a stranger's page load with 404 and a member's with 303, which tells the stranger which records exist.
- **`HX-Push-Url: false` is set here**, and has to be remembered in every other fragment.
- **The GET carries the POST's parsing and writes.** To a middleware it looks like an action, so nothing can tell the dialog is a fragment, and a GET that "only shows a dialog" is one refactor away from writing.
- **The 422 is a bare status.** Every module picks its own way to tell htmx to swap it.

After, the same behaviour is two route types and no navigation code:

```go
type projectPages struct {
    project    projectPage  `route:"GET /{$} Project"`
    // ...
    rename     renameDialog `route:"GET /tasks/{task}/rename Rename task"`
    saveRename saveRename   `route:"POST /tasks/{task}/rename Rename task"`
}

// loadTask is the shared part. Both route types call it; neither calls the other.
func loadTask(r *http.Request, store *Store) (Task, error) {
    return store.Task(r.PathValue("project"), r.PathValue("task"))
}

// Props shows the dialog: no form parsing, no writes.
func (p renameDialog) Props(r *http.Request, store *Store) error {
    task, err := loadTask(r, store)
    if err != nil {
        return err
    }
    return structpages.RenderComponent(p.Dialog(task, task.Title, ""))
}

// ServeHTTP saves the new title: the only place that parses the form and writes.
func (saveRename) ServeHTTP(w http.ResponseWriter, r *http.Request, store *Store) error {
    task, err := loadTask(r, store)
    if err != nil {
        return err
    }
    title := strings.TrimSpace(r.PostFormValue("title"))
    if title == "" {
        return structpages.RenderComponent(renameDialog{}.Dialog(task, title, "A task needs a title."))
    }
    if err := store.RenameTask(task.ProjectID, task.ID, title); err != nil {
        return err
    }
    return structpages.RenderComponent(taskList{}.Refreshed(task.ProjectID, store.Tasks(task.ProjectID)))
}
```

| Concern | Before: in the route | After: decided once |
|---|---|---|
| Is this a page load? | `HX-Request` check, wrong for history restores and boosted links | the guard's intent rules |
| Where does it land? | a hand-built string | the ancestor page, found from the route tree |
| Access on a page load | depends on branch order | always after landing; the page checks its own |
| History | `HX-Push-Url: false` in each fragment | the guard, on every fragment response |
| Show vs save | one handler switching on `r.Method` | a fragment and an action sharing `loadTask` |
| Refusals | `w.WriteHeader(422)` | an outcome type the error handler maps. The example keeps a 200 re-render, which htmx 2 swaps without configuration |
| Caching | whatever each route remembered | `Vary`, no-store and no validators, from the guard |

The example's `main_test.go` checks the result. It covers each landing case as a new tab, a boosted link and a partial, the save, and a route walk that opens every GET as a document.

## Pitfalls

Each of these is easy to get wrong, and each one breaks something real:

- **Treating `HX-Request-Type: full` as a page load.** Every `hx-select` swap then navigates away. Ask the target.
- **Stripping `HX-Target` before other middleware reads it.** Deleting htmx headers to stop a stale target from selecting a fragment also hides them from every middleware and render target after it, including the ones that need them on real htmx requests. Classify from the headers and leave them alone. If you do drop stale htmx headers, drop them only from native requests, and before any middleware reads them.
- **A fragment's own page-load branch.** A fragment that used to redirect page loads itself (`if r.Header.Get("HX-Request") != "true" { … }`) never sees one under the guard, so the branch is dead. Delete it if the guard lands on the same page. Otherwise, make the fragment `SelfLanding`.
- **Testing a dialog with a plain GET.** It gets a 303, not the dialog, and not the dialog's 404. Send the partial request.
- **Rendering the fragment in a page on a page load.** It's tempting to wrap the fragment in the layout instead of redirecting. Then its URL becomes a second, poorer address for the page, and it can't know the data or parameters the page needs.
- **Trusting `Referer` or a `return_to` parameter to choose the landing.** Shared fragments choose one canonical landing.

## Checklist

- Every GET that shows something is a page or a fragment. A fragment has no `Page`, and its Props returns `RenderComponent` or an error.
- Each HTTP method has its own route type. Nothing switches on `r.Method`.
- Fragments land by default. Use `Landing` only when the ancestor is wrong, and `SelfLanding` only when the landing needs a lookup.
- Only `SelfLanding` fragments test intent, and they use the guard's `Partial`, not raw headers.
- Access tests send the partial request.
- Props sets no history, status or swap headers. Typed helpers and outcomes do.
- Only page URLs are pushed to history.
- Lint checks the shapes, and a route walk loads every GET as a document.

## See also

- [Route Conventions](./route-conventions.md): pages, fragments, actions and protocols.
- [Middleware](./middleware.md): `MiddlewareFunc` and the `*PageNode` it receives.
- [HTMX Integration](./htmx.md#history-and-fragment-urls): history and fragment URLs from the htmx side.
- [Error Handling](./error-handling.md): buffering, the error handler, redirects from actions.
