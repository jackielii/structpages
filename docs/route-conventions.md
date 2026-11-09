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

None of this needs a library change. It is a way of using what structpages already does, and [Guarding Fragment URLs](./fragment-guard.md) builds a navigation guard on it.

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

## One route type per method

A form usually has a GET that shows it and a POST that saves it, at the same path. Give each method its own route type: the GET is a fragment (or a page) with `Props`, and the POST, PATCH or DELETE is an action with `ServeHTTP`. They share plain functions for what they have in common, such as loading the record:

```go
type pages struct {
    rename     renameDialog `route:"GET /widgets/{widget}/rename Rename widget"`
    saveRename saveRename   `route:"POST /widgets/{widget}/rename Rename widget"`
}

// loadRename is the shared part. Both route types call it; neither calls the other.
func loadRename(r *http.Request, store *Store) (renameForm, error) {
    w, err := store.Widget(r.Context(), r.PathValue("widget"))
    if err != nil {
        return renameForm{}, err
    }
    return renameForm{Widget: w, Name: w.Name}, nil
}

// Props shows the dialog: no form parsing, no writes.
func (p renameDialog) Props(r *http.Request, store *Store) error {
    f, err := loadRename(r, store)
    if err != nil {
        return err
    }
    return structpages.RenderComponent(p.Dialog(f))
}

// ServeHTTP saves the new name, the only place that parses the form and writes.
func (saveRename) ServeHTTP(w http.ResponseWriter, r *http.Request, store *Store) error {
    f, err := loadRename(r, store)
    if err != nil {
        return err
    }
    f.Name = r.PostFormValue("name")
    // validate, save, and answer with the refreshed widget
}
```

Never switch on `r.Method` inside one handler, and never call another route's handler. A shared handler hides the POST's parsing and writes behind the GET, and middleware that reads the route's shape sees a fragment where there is also an action. The one exception is HEAD on a protocol resource, which the same no-error `ServeHTTP` answers.

## Fragment URLs in the browser

Fragments have URLs, and browsers will open them: a link in a new tab, a bookmark, a notification click, Back or reload after a URL was pushed. Each of these gets bare markup with no layout. Because the route kinds above are visible on every `*PageNode`, one middleware can fix this for every fragment at once. [Guarding Fragment URLs](./fragment-guard.md) is the full pattern: telling a page load from a partial, choosing where a fragment's page load lands, keeping fragment URLs out of history and caches, and enforcing the shapes.

## Checklist

- A GET that shows a document has `Page`. Its regions are components of the same page, chosen with `target.Is`.
- A GET that shows part of a document has no `Page`, and its `Props` returns `RenderComponent(...)` or an error.
- No component of a fragment is named `Page`.
- A GET `ServeHTTP` only redirects or fails.
- Non-HTML routes use the no-error `ServeHTTP` form.
- No route has both `Page` and `ServeHTTP`.
- Each HTTP method has its own route type; nothing switches on `r.Method`.
- Only page URLs are pushed to history.

## See also

- [Guarding Fragment URLs](./fragment-guard.md): the navigation guard, landing pages, history and caching.
- [Page Response Patterns](./supported-flows.md): the shapes in code, with the todo example.
- [How a Request Is Rendered](./rendering.md): the target selector, Props, and which component renders.
- [Middleware](./middleware.md): `MiddlewareFunc` and the `*PageNode` it receives.
- [Error Handling](./error-handling.md): buffering, redirects from actions, JSON endpoints.
