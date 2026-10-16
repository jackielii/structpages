---
title: URLFor & ID
slug: /urlfor
sidebar_position: 8
---

# URLFor & ID

structpages provides type-safe helpers for generating URLs and DOM identifiers that stay in sync with the route tree:

- **`URLFor(ctx, page, params)`** — build a URL for a page from its type.
- **`Ref("Parent.Field")`** — string reference for cases the static type lookup can't handle.
- **`ID(ctx, Page.Method)`** — raw HTML `id` attribute for a page component.
- **`IDTarget(ctx, Page.Method)`** — `#`-prefixed CSS selector for HTMX `hx-target`.
- **`Within(ctx, page)`** — whether the page being rendered is that page or below it, for active navigation.

All are validated by the [`structpages-lint`](./lint.md) analyzer. **The rule of thumb: never write an in-app URL as a string literal** — resolve it by page type so the literal can't drift when routes move.

## URLFor

`structpages.URLFor(ctx, page, args...)` returns `(string, error)`. In gsx it is the `url` filter: `{ page |> url(params) }` is `structpages.URLFor(ctx, page, params)`, and the error is returned from `Render`:

```gsx
component ItemLinks(item Item) {
    <a href={MyPage{} |> url}>Link</a>
    <a href={DetailPage{} |> url(map[string]any{"itemId": item.ID})}>Detail</a>
    <form action={SavePage{} |> url(map[string]any{"itemId": item.ID})} method="POST">…</form>
}
```

In Go code (Props, handlers, middleware) call `structpages.URLFor(r.Context(), ...)` and handle the error. templ users call it directly in attribute expressions (see [Using templ](./templ.md)).

**The recommended shape is two arguments**: `URLFor(ctx, page, params)` with `params` as a `map[string]any`. It's explicit at the call site, survives route changes, and fills both path and query placeholders by name. Positional and key/value-pair forms also work but are easier to misalign during refactors.

| form | shape | use when |
|---|---|---|
| bare typed page | `URLFor(ctx, MyPage{}, params)` | the type is mounted exactly once |
| typed chain | `URLFor(ctx, []any{Parent{}, Leaf{}}, params)` | same leaf type mounted under multiple parents — parent disambiguates |
| chain + URL fragment | `URLFor(ctx, []any{Parent{}, Leaf{}, "?tab={t}"}, params)` | need to append a query template or path suffix |
| string (auto-Ref) | `URLFor(ctx, "Parent.Field", params)` | can't import the page type (cross-package cycle); top-level strings only — strings inside `[]any` are URL fragments |
| Ref by qualified name | `URLFor(ctx, Ref("Parent.Field"), params)` | explicit form of the string sugar above |

### Always strict

A bare type that matches multiple mounted nodes **errors** instead of silently picking one. The error lists every match and recommends the chain form. There is no opt-out — silent first-match is always wrong, so disambiguating at the call site is mandatory:

```go
type root struct {
    Components componentsRoot `route:"/components Components"`
    Patterns   patternsRoot   `route:"/patterns Patterns"`
}
type componentsRoot struct { Detail entryPage `route:"/{slug} Component"` }
type patternsRoot   struct { Detail entryPage `route:"/{slug} Pattern"` }

// Bare URLFor errors — entryPage matches two nodes.
url, err := structpages.URLFor(ctx, entryPage{}, map[string]any{"slug": "button"})

// Chain anchors at the parent struct; descends into the entryPage child.
url, err := structpages.URLFor(ctx,
    []any{componentsRoot{}, entryPage{}},
    map[string]any{"slug": "button"})
// → "/components/button"
```

**Chain semantics:** inside `[]any{...}`, leading typed values form a chain through the page tree; each subsequent typed value descends into a child of that type (must be unique among siblings). Once a string appears, no more typed values are allowed; remaining strings concat literally to the pattern. A typed value after a string fragment errors at runtime.

### Query strings

Pass a `[]any` whose trailing strings form the URL template; the `map[string]any` fills both path and query placeholders:

```go
url, err := structpages.URLFor(ctx,
    []any{MyList{}, "?page={page}&q={q}"},
    map[string]any{"page": pageNum, "q": query},
)
```

In markup the same form builds pagination links, one per page:

```gsx
component Pager(current int, total int, status string) {
    <nav class="pager">
        { for n := 1; n <= total; n++ {
            <a
                href={[]any{ListPage{}, "?page={page}&status={status}"} |> url(map[string]any{"page": n, "status": status})}
                { if n == current { aria-current="page" } }
            >
                { n }
            </a>
        } }
    </nav>
}
```

### Page groups resolve to their index

A [page group](./concepts.md) is never served at its bare path — ServeMux matches only its subtree, and the bare path 307-redirects to add the trailing slash. So `URLFor` on a page group returns its index child's URL (the `/{$}` route) with the canonical trailing slash:

```go
href, err := structpages.URLFor(ctx, sectionRoot{})
// → "/section/"   (not "/section", which would 307)
```

Link a page group by its type and the URL serves a 200 directly. Don't hand-append a trailing slash, and don't link to the slashless form. Leaf pages return their own bare path unchanged.

### Params auto-fill from the current request

Unfilled placeholders that match path params from the *current request's route* are filled automatically — you only pass what differs:

```go
// Inside a handler for /orgs/{org}/products/{productId}:
href, err := structpages.URLFor(ctx, siblingPage{})
// → "/orgs/acme/products/p-123/sibling"  (params inherited)
```

Only the current route's params auto-fill; sibling routes with different param names do not.

## Dynamic references with Ref

When the target page can't be referenced by static type — a cross-package import would cycle, a configuration-driven menu stores its targets as data, or a Go type alias collapses two routes onto one `reflect.Type` — use `Ref` (a string type):

```go
// By page field name
url, err := structpages.URLFor(ctx, structpages.Ref("homePage"))

// Qualified path, with params
url, err := structpages.URLFor(ctx, structpages.Ref("Admin.ProductPage"),
    map[string]any{"productId": "123"})

// Composed with a URL fragment
url, err := structpages.URLFor(ctx, []any{structpages.Ref("userPage"), "?tab=profile"})
```

`Ref` resolves by field-name path in the page tree. The anchor segment matches a top-level node if one has that name, otherwise any uniquely-named node anywhere in the tree. An ambiguous anchor errors — qualify it with a parent segment. Ref strings are validated by `structpages-lint`, including refs stored in struct fields, so a lint-passing Ref resolves at runtime.

For page components, `Ref("PageName.MethodName")` works with `ID`/`IDTarget`:

```go
id, err := structpages.ID(ctx, structpages.Ref("userPage.UserList"))
// → "user-page-user-list"
```

An unqualified method name (`Ref("UserList")`) resolves only if exactly one page has that method; ambiguity errors with the candidates listed.

### Example: configuration-driven menu

```go
type MenuItem struct {
    PageRef structpages.Ref
    Label   string
}

var menu = []MenuItem{
    {structpages.Ref("home"), "Home"},
    {structpages.Ref("users"), "Users"},
    {structpages.Ref("settings"), "Settings"},
}
```

```gsx
component Navigation() {
    <nav>
        { for _, item := range menu {
            <a href={item.PageRef |> url}>{ item.Label }</a>
        } }
    </nav>
}
```

The `url` filter returns its error from `Render`, so the call site needs no error handling. `structpages-lint` checks the Refs stored in `menu`, so a renamed page fails CI; the [boot-time inventory](#validation-no-dangling-urls-in-production) covers deploys. To mark the item for the current page, see [Active navigation](#active-navigation).

### Type aliases

Go type aliases (`type X = Y`) are identical at runtime — `reflect.TypeOf` can't tell them apart. When a type and its alias are both mounted, a bare type lookup is ambiguous:

```go
type productPage struct{}
type featuredProduct = productPage // alias — same reflect.Type

type pages struct {
    products productPage     `route:"/products Products"`
    featured featuredProduct `route:"/featured Featured"`
}

// Errors: productPage matches two mounted nodes, and the error lists both.
url, err := structpages.URLFor(ctx, featuredProduct{})
```

Disambiguate by field name:

```go
url, err := structpages.URLFor(ctx, structpages.Ref("featured"))     // → "/featured"
id, err := structpages.IDTarget(ctx, structpages.Ref("featured.Page")) // → "#featured-page"
```

The `[]any{Parent{}, Leaf{}}` chain doesn't help here, since both mounts share one type. Aliases are fine for routing and rendering; only type-based lookup is affected.

## ID and IDTarget

For HTMX you need the server-rendered `id` attribute and the client-side `hx-target` selector to agree. Generate both from the same method reference with the `id` and `target` filters:

```gsx
component (p index) Content(props IndexProps) {
    <form hx-post={addTodo{} |> url} hx-target={index.TodoList |> target}>…</form>
    <div id={index.TodoList |> id}>
        <p.TodoList todos={props.Todos}/>
    </div>
}
```

`ID(ctx, index.TodoList)` returns the page's full field-name path joined with the method — `"index-todo-list"` for a top-level page, `"admin-users-todo-list"` when nested; `IDTarget` prepends `#`. Plain strings pass through both functions unchanged — `"body" |> target` is `"body"`, not `"#body"`. The full id scheme, multi-mount disambiguation, and the swap loop are covered in [HTMX Integration](./htmx.md).

## Active navigation

A nav bar marks the item the current page sits under. Ask `structpages.Within(ctx, page)` rather than matching URL prefixes: it reports whether the page being rendered is `page` or one of its descendants. `page` takes the forms `URLFor` takes (a page value, `Ref`, string, a `[]any` chain without string fragments, or a predicate).

```go
type managePages struct {
    Inbox   inboxPage   `route:"/{$} Inbox"`
    Tickets ticketPages `route:"/tickets Tickets"`
}

type ticketPages struct {
    List   ticketList   `route:"/{$} Tickets"`
    Detail ticketDetail `route:"/{number} Ticket"`
}

type navItem struct {
    Label string
    Page  any // any form URLFor takes as its page
}

type navLink struct {
    Label  string
    Href   string
    Active bool
}

// navRail resolves its items against the page being served and renders them.
type navRail []navItem

var manageNav = navRail{
    {Label: "Inbox", Page: structpages.Ref("Manage.Inbox")},
    {Label: "Tickets", Page: structpages.Ref("Manage.Tickets")},
}

func (r navRail) Render(ctx context.Context, w io.Writer) error {
    links := make([]navLink, 0, len(r))
    for _, item := range r {
        href, err := structpages.URLFor(ctx, item.Page)
        if err != nil {
            return err
        }
        active, err := structpages.Within(ctx, item.Page)
        if err != nil {
            return err
        }
        links = append(links, navLink{Label: item.Label, Href: href, Active: active})
    }
    return RailLinks(links).Render(ctx, w)
}
```

```gsx
component RailLinks(links []navLink) {
    <nav class="rail">
        { for _, l := range links {
            <a href={l.Href} { if l.Active { aria-current="page" } }>{ l.Label }</a>
        } }
    </nav>
}
```

- **A group covers its subtree.** "Tickets" is a group, so it is active on the list and on every `/tickets/{number}` page, with no args for the path parameter.
- **Pages compare by node, not by path.** "Inbox" is the `/{$}` index leaf, so it is active only on the inbox, even though its URL is a prefix of the ticket URLs.
- **Outside a page render** — no `CurrentPage` in ctx, such as a `ServeHTTP` page or a bare `PageContext` in a test — `Within` returns `false, nil`. A target that doesn't resolve is an error, so a renamed page fails the render instead of quietly never matching.

`structpages.CurrentPage(ctx)` returns the matched leaf `*PageNode` itself if you need more than a yes/no; walk `Parent` for its ancestors.

## Validation: no dangling URLs in production

[`structpages-lint`](./lint.md) is the primary guard — it statically validates `URLFor`/`Ref` calls, params, and hard-coded routes in CI. For what static analysis can't see (URLs assembled from runtime data, refs behind dynamic dispatch), add a boot-time inventory that kills the startup with the list of what's dangling:

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

Call it from `main` after `Mount` (fail the boot) and from a one-line test (coverage in CI). See [`examples/url-validation/`](https://github.com/jackielii/structpages/tree/main/examples/url-validation) for the runnable pattern.

## No `must()` helpers

gsx holes and filters take `(T, error)` wherever the error can be returned from `Render`: element and component attributes, text holes, and the `@{}` holes of `js`/`f` literals on an attribute. So don't write a `must()` helper or app-level `urlFor`/`idFor` wrappers.

The exception is a literal assigned inside a `{{ }}` block, which is a Go statement with no error channel; gsx rejects error-returning filters there. Resolve the value in the attribute where it's used instead, or have a wrapper component take the plain string (`slot={Index.Canvas |> target}`) and build the literal on its own element. templ has its own plain-string places (`templ.Attributes` values); see [Using templ](./templ.md).
