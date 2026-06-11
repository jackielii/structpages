---
title: URLFor & ID
slug: /urlfor
sidebar_position: 8
---

# URLFor & ID Generation

structpages provides type-safe helpers for generating URLs and DOM identifiers that stay in sync with the route tree:

- **`URLFor(ctx, page, params)`** — build a URL for a page from its type.
- **`Ref("Parent.Field")`** — string reference for cases the static type lookup can't handle.
- **`ID(ctx, Page.Method)`** — raw HTML `id` attribute for a page component.
- **`IDTarget(ctx, Page.Method)`** — `#`-prefixed CSS selector for HTMX `hx-target`.

All are validated by the [`structpages-lint`](./lint.md) analyzer. **The rule of thumb: never write an in-app URL as a string literal** — resolve it by page type so the literal can't drift when routes move.

## URLFor

`structpages.URLFor(ctx, page, args...)` returns `(string, error)`. Templ attribute values accept `(string, error)` directly:

```templ
<a href={ structpages.URLFor(ctx, MyPage{}) }>Link</a>
<a href={ structpages.URLFor(ctx, DetailPage{}, map[string]any{"itemId": item.ID}) }>Detail</a>
<form action={ structpages.URLFor(ctx, SavePage{}, map[string]any{"itemId": item.ID}) } method="POST">
```

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

## Ref

When the target page can't be referenced by static type — a cross-package import would cycle, or a Go type alias collapses two routes onto one `reflect.Type` — use `Ref` (a string type):

```go
url, err := structpages.URLFor(ctx, structpages.Ref("Admin.Settings"))
```

`Ref` resolves by field-name path in the page tree. The anchor segment matches a top-level node if one has that name, otherwise any uniquely-named node anywhere in the tree. An ambiguous anchor errors — qualify it with a parent segment. Ref strings are validated by `structpages-lint` (including refs stored in struct fields, e.g. a nav table), so a lint-passing Ref resolves at runtime.

## ID and IDTarget

For HTMX you need the server-rendered `id` attribute and the client-side `hx-target` selector to agree. Generate both from the same method reference:

```templ
<div id={ structpages.ID(ctx, index.TodoList) }>
    @p.TodoList(props.Todos)
</div>

<form hx-post={ structpages.URLFor(ctx, addTodo{}) }
      hx-target={ structpages.IDTarget(ctx, index.TodoList) }>
```

`ID(ctx, index.TodoList)` returns the page's full field-name path joined with the method — `"index-todo-list"` for a top-level page, `"admin-users-todo-list"` when nested; `IDTarget` prepends `#`. Plain strings pass through both functions unchanged — `IDTarget("body")` is `"body"`, not `"#body"`. The full id scheme, multi-mount disambiguation, and the swap loop are covered in [HTMX Integration](./htmx.md).

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

## Plain strings outside templ attributes

Templ attribute expressions take `(string, error)` directly — no wrapper needed. The exception, still inside templ, is a context that needs a plain string, like `templ.Attributes` map values; use a small `must` helper for those (and only those):

```go
func must[T any](v T, err error) T {
    if err != nil { panic(err) }
    return v
}
```

```templ
@PrimaryButton(templ.Attributes{
    "hx-get": must(structpages.URLFor(ctx, UserNewModal{})),
}) { + New User }
```
