---
title: Route Conventions
slug: /route-conventions
sidebar_position: 5.5
---

# Route Conventions

A route struct's methods say what kind of route it is. Follow one rule and the framework, your middleware and your readers can all tell a document from a fragment from an action without extra metadata:

> **Props decides what to show; ServeHTTP decides what to do.**

| Route struct | Kind | What it does |
|---|---|---|
| Has `Page`, plus any region components (`Content`, `Orders`, `Invoices`, …); `Props` returns data and an error | **Page** | A document. A navigation gets `Page`; an htmx request for a region gets that component, on the page's own URL: `Content` for in-shell swaps, `Orders` to refresh the orders table. These are the page's **partials**. |
| No `Page`; `Props(...) error` returning `RenderComponent(...)` | **Fragment** | Markup with its own route, only ever swapped into a document: a dialog, a lazily loaded panel, a row. Props picks the component and renders it explicitly. |
| `ServeHTTP(w, r, ...) error` | **Action** | Does something. A POST, PATCH or DELETE may answer with a refreshed region; a GET only redirects or fails ("jump to message", an old alias). |
| `ServeHTTP(w, r, ...)` with no error | **Protocol** | Owns the response: downloads, JSON, server-sent events, OAuth callbacks. |
| Only child routes | **Group** | No handler of its own. Its `/{$}` child is its index page. |

None of this needs a library change. It is a way of using what structpages already does, and the [navigation guard](#guarding-fragment-urls) below builds on it.

## Pages

A page has a `Page` component, and that alone makes it a document: every request that doesn't ask for a region gets the whole thing. `Props` loads the data, and may load less when the request targets one region:

```go
type product struct{}

func (p product) Props(r *http.Request, store *Store) (Product, error) {
    return store.Product(r.PathValue("id"))
}
```

```gsx
component (p product) Page(prod Product) {
	<Layout>
		<p.Content prod={prod}/>
	</Layout>
}

component (p product) Content(prod Product) {
	<h1>{ prod.Name }</h1>
}
```

`Content` is optional. Add it when your layout swaps the main area (`hx-target="#content"`) and the page should answer with just that part.

### Partials: regions of a page

A page usually has more than one region that refreshes on its own: two tables, a filter panel, a stats widget. Each is a component of the page, served from the page's own URL. `Props` returns one value per region, asks `target.Is` which region the request wants, and loads only that region's data:

```go
type account struct{}

func (p account) Props(r *http.Request, target structpages.RenderTarget, store *Store) (Account, []Order, []Invoice, error) {
    id := r.PathValue("accountId")
    switch {
    case target.Is(p.Orders):
        orders, err := store.Orders(id, r.FormValue("status"))
        return Account{}, orders, nil, err
    case target.Is(p.Invoices):
        invoices, err := store.Invoices(id)
        return Account{}, nil, invoices, err
    }
    // Page and Content: everything.
    acct, err := store.Account(id)
    if err != nil {
        return Account{}, nil, nil, err
    }
    orders, err := store.Orders(id, "")
    if err != nil {
        return Account{}, nil, nil, err
    }
    invoices, err := store.Invoices(id)
    return acct, orders, invoices, err
}
```

```gsx
component (p account) Page(acct Account, orders []Order, invoices []Invoice) {
	<Layout>
		<p.Content acct={acct} orders={orders} invoices={invoices}/>
	</Layout>
}

component (p account) Content(acct Account, orders []Order, invoices []Invoice) {
	<h1>{ acct.Name }</h1>
	<p.Orders orders={orders}/>
	<p.Invoices invoices={invoices}/>
}

component (p account) Orders(orders []Order) {
	<section id={account.Orders |> id}>
		<select
			name="status"
			hx-get={account{} |> url}
			hx-target={account.Orders |> target}
			hx-swap="outerHTML"
		>…</select>
		<table>…</table>
	</section>
}

component (p account) Invoices(invoices []Invoice) {
	<section id={account.Invoices |> id}><table>…</table></section>
}
```

| Request | Renders | Data loaded |
|---|---|---|
| navigation to `/accounts/42` | `Page` | everything |
| htmx, `hx-target="#content"` | `Content` | everything |
| htmx, target `account.Orders` | `Orders` | orders only |
| htmx, target `account.Invoices` | `Invoices` | invoices only |

Each component takes only its own values, filled by type from Props' results, so give each region its own type. The framework renders the component the target `Is`, so Props only branches to load less; it doesn't need `RenderComponent`. The full matching rules are in [HTMX Integration](./htmx.md#rendertarget-in-props).

**Partial or fragment?** A partial has no URL of its own: a navigation to the page's URL gets the whole page, so there is nothing to guard. Make a region a partial when it is part of the page on first load and refreshes in place. Make it a [fragment](#fragments), with its own route, when it is only fetched on demand (a dialog, a panel opened by a click) or needs parameters the page doesn't have, such as the id of the row being edited.

## Fragments

A fragment is markup that only makes sense inside some page: a dialog, a list's rows, a details panel. It has **no `Page`**, and its `Props` returns only an error: `RenderComponent(...)` on success, anything else on failure.

```go
type editDialog struct{}

func (p editDialog) Props(r *http.Request, store *Store) error {
    item, err := store.Item(r.PathValue("id"))
    if err != nil {
        return err
    }
    return structpages.RenderComponent(p.Dialog(item))
}
```

```gsx
component (p editDialog) Dialog(item Item) {
	<dialog open>
		<form hx-post={updateItem{} |> url("id", item.ID)}>...</form>
	</dialog>
}
```

When a fragment has more than one component, Props chooses between them with `target.Is` and still renders the choice explicitly:

```go
func (p rows) Props(r *http.Request, target structpages.RenderTarget, store *Store) error {
    items, err := store.Items(r.Context())
    if err != nil {
        return err
    }
    if target.Is(p.Body) {
        return structpages.RenderComponent(p.Body(items))
    }
    return structpages.RenderComponent(p.Table(items))
}
```

Three rules keep fragments honest:

- **Never name a fragment's component `Page`.** `Page` is what a document navigation gets. A fragment named `Page` renders its bare markup to anyone who opens the URL, and middleware can no longer tell it from a real page.
- **Never rely on the fallback to `Page`.** When Props returns `(results…, nil)`, the framework renders the component the target `Is`, else `Page`. A fragment has no `Page` to fall back to, so a request that matches none of its components fails at request time. Returning `RenderComponent` makes Props the one place that decides.
- **Don't return data that nobody renders.** A fragment's Props returns `error`, not `(T, error)`. If it computes a value, it passes it to the component it renders.

## Actions

An action does something and says where to go next. It is a `ServeHTTP` method that returns an error, so it gets [buffered responses and the error handler](./error-handling.md):

```go
type updateItem struct{}

func (updateItem) ServeHTTP(w http.ResponseWriter, r *http.Request, store *Store) error {
    item, err := store.Update(r.Context(), r.PathValue("id"), r.FormValue("name"))
    if err != nil {
        return err
    }
    return structpages.RenderComponent(itemList{}.Row(item))
}
```

A POST, PATCH or DELETE may answer an htmx request with a refreshed region, as above. **A GET action never writes markup**: it redirects (an old URL, a "jump to message" link) or returns an error. If a GET shows something, it is a page or a fragment, and it belongs in `Props`.

Keep pages and actions apart, too. `ServeHTTP` takes precedence over `Page`, so a struct with both serves only its `ServeHTTP`. Give the form its own route instead: a `GET` page and a `POST` action at the same path, as the blog example's [login](https://github.com/jackielii/structpages/blob/main/examples/blog/admin/login.gsx) does.

## Protocols

Some routes aren't HTML at all: file downloads, JSON APIs, server-sent event streams, OAuth callbacks, static assets. Use the [no-error `ServeHTTP` form](./supported-flows.md#servehttp-signatures) so writes go straight to the wire and the HTML error handler stays out of it. If your app checks routes against these conventions (a linter, a test that visits every GET), give protocol routes an explicit marker, such as a method of your own, so they are an exception you declared rather than one you guessed.

## Guarding fragment URLs

Fragments have URLs, and browsers will open them: a link opened in a new tab, a notification click, a bookmark, Back or reload after a URL was pushed to history. Each of these gets bare markup with no layout, no scripts and no styles.

Under these conventions the fix is one generic middleware. A [`MiddlewareFunc`](./middleware.md) receives each route's `*PageNode` at mount time, and the node already says what kind of route it is: `pn.Components["Page"]` for a page, `pn.Props` without `Page` for a fragment. For a fragment, the guard looks at what the request wants:

| Request | Wants | Guard's response |
|---|---|---|
| `Sec-Fetch-Mode: navigate`, or `Sec-Fetch-Dest: document`, `iframe` or `frame` | a document, whatever htmx headers it carries | 303 to the landing page |
| `HX-History-Restore-Request: true` | a document | 303 (htmx 2 restores history with its own request, which follows redirects but ignores response headers) |
| `HX-Request: true` with `HX-Request-Type: full` (htmx 4) | a whole page | `HX-Location` to the landing page |
| `HX-Request: true`, otherwise | a part of a page | the fragment, as usual |
| anything else: no htmx headers, curl, tests | a document | 303 to the landing page |

Only an explicit htmx partial gets the fragment; everything else counts as a navigation. The guard redirects **without running Props**, so it loads no data and doesn't depend on the fragment's parameters being valid.

The landing page is, in order:

1. the fragment's own choice, when it implements a `Landing(*http.Request) (string, error)` method. Use this when the page that hosts the fragment is a sibling rather than an ancestor;
2. the nearest ancestor with `Page`, or a group's `/{$}` index page, whose path parameters the request can fill;
3. otherwise, your app's not-found page.

The guard also keeps caches from mixing representations: every rendering route gets `Vary` on the htmx and `Sec-Fetch` headers, and fragments and partial responses get `Cache-Control: private, no-store`.

```go
func NavigationGuard(next http.Handler, pn *structpages.PageNode) http.Handler {
    _, hasPage := pn.Components["Page"]
    fragment := !hasPage && pn.Props != nil
    if !hasPage && !fragment {
        return next // ServeHTTP routes: actions, redirects, protocols
    }
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Add("Vary", "HX-Request, HX-Target, HX-Request-Type, HX-History-Restore-Request, Sec-Fetch-Mode, Sec-Fetch-Dest")
        if r.Method != http.MethodGet && r.Method != http.MethodHead {
            next.ServeHTTP(w, r)
            return
        }
        intent := requestIntent(r)
        if intent == partial || fragment {
            w.Header().Set("Cache-Control", "private, no-store")
        }
        if !fragment || intent == partial {
            next.ServeHTTP(w, r)
            return
        }
        to, err := landingURL(r, pn)
        if err != nil {
            http.NotFound(w, r) // render your app's error page here
            return
        }
        if intent == htmxFull {
            w.Header().Set("HX-Location", to)
            return
        }
        http.Redirect(w, r, to, http.StatusSeeOther)
    })
}

type intent int

const (
    document intent = iota
    htmxFull
    partial
)

func requestIntent(r *http.Request) intent {
    switch r.Header.Get("Sec-Fetch-Dest") {
    case "document", "iframe", "frame":
        return document
    }
    if r.Header.Get("Sec-Fetch-Mode") == "navigate" ||
        r.Header.Get("HX-History-Restore-Request") == "true" ||
        r.Header.Get("HX-Request") != "true" {
        return document
    }
    if r.Header.Get("HX-Request-Type") == "full" {
        return htmxFull
    }
    return partial
}

// Landing is implemented by a fragment whose hosting page is not an ancestor.
type Landing interface {
    Landing(r *http.Request) (string, error)
}

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
    return "", errors.New("no page hosts this fragment")
}

func isGETPage(pn *structpages.PageNode) bool {
    _, hasPage := pn.Components["Page"]
    return hasPage && (pn.Method == http.MethodGet || pn.Method == "ALL")
}
```

Register it with the rest of your middleware:

```go
sp, err := structpages.Mount(mux, index{}, "/", "App",
    structpages.WithTargetSelector(structpages.HTMXRenderTarget),
    structpages.WithMiddlewares(NavigationGuard),
)
```

`URLFor` works inside middleware: structpages puts the route tree and the request's path parameters in the context before your middleware runs, so `/items/{id}/edit` lands on `/items/{id}` with the same `id`. A predicate that compares nodes (`c == n`) finds exactly the ancestor the guard chose.

The [htmx example](https://github.com/jackielii/structpages/tree/main/examples/htmx) runs this guard, and its `navguard_test.go` drives each request kind through it.

Adapt it to your app where it differs:

- **Authentication.** Put the guard before your sign-in middleware if a signed-out visitor should come back to the landing page rather than the fragment URL.
- **Error pages.** Replace `http.NotFound` with your app's full-document error page.
- **Query strings.** The landing URL carries path parameters only. A fragment that should keep, say, `?tab=`, builds its URL in its `Landing` method.

### Keep fragment URLs out of history

The guard repairs a fragment URL that a browser opens. It is better never to put one in the address bar:

- Only push or replace page URLs. Use `hx-push-url="true"` on requests to a page (the htmx example's nav links swap `#content` from page URLs), and on a fragment request set `hx-push-url="false"` or push the hosting page's URL explicitly.
- `Vary` keeps HTTP caches apart, but the browser's back-forward cache stores whole documents, not responses. A document whose address is a fragment URL can only be fixed by a reload, which the guard then redirects.

## Checklist

- A GET that shows a document has `Page`. Its regions are components of the same page, chosen with `target.Is`.
- A GET that shows part of a document has no `Page`, and its `Props` returns `RenderComponent(...)` or an error.
- No component of a fragment is named `Page`.
- A GET `ServeHTTP` only redirects or fails.
- Non-HTML routes use the no-error `ServeHTTP` form.
- No route has both `Page` and `ServeHTTP`.
- Only page URLs are pushed to history.

## See also

- [Page Response Patterns](./supported-flows.md): the shapes in code, with the todo example.
- [How a Request Is Rendered](./rendering.md): the target selector, Props, and which component renders.
- [Middleware](./middleware.md): `MiddlewareFunc` and the `*PageNode` it receives.
- [Error Handling](./error-handling.md): buffering, redirects from actions, JSON endpoints.
