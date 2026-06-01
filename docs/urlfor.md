---
title: URLFor & ID
slug: /urlfor
sidebar_position: 5
---

# URLFor & ID Generation

`structpages` provides four related helpers for generating URLs and DOM identifiers that stay in sync with the route tree:

- **`URLFor(target)`** — build a URL for a page from its struct (pointer to a leaf, or `Ref`).
- **`Ref{...}`** — dynamic reference for cases the static type lookup can't handle.
- **`ID(target)`** — raw HTML `id` attribute string for a page or component.
- **`IDTarget(target)`** — `#`-prefixed CSS selector for HTMX `hx-target`.

All four are checked at build time by the [`structpages-lint`](../tools/lint) analyzer.

## URLFor

Generate a URL by passing a pointer to the target page struct:

```go
href := structpages.URLFor(ctx, &productPage{})
// → "/products"
```

If the route has path parameters, pass them as additional arguments in tag order:

```go
href := structpages.URLFor(ctx, &productPage{}, "p-123")
// → "/products/p-123"
```

### Container pages resolve to their index

A subtree container — a page struct that only groups child routes and has no
render logic of its own — is never served at its bare path: `http.ServeMux`
matches only its subtree, and the bare path 307-redirects to add the trailing
slash. `URLFor` on a container therefore returns its index child's URL (the
`/{$}` route), i.e. the canonical trailing-slash form, so the link serves
directly with no redirect hop:

```go
// type sectionRoot struct { Index sectionIndex `route:"/{$}"`; … }
href := structpages.URLFor(ctx, &sectionRoot{})
// → "/section/"   (not "/section", which would 307)
```

Leaf pages — those that render or handle their own route — return their own
path unchanged.

### `[]any` chain for nested params

When the target is deeply nested with parameters at multiple levels, pass the chain as a single `[]any`:

```go
// route: /orgs/{org}/products/{id}/edit
href := structpages.URLFor(ctx, &editPage{}, []any{"acme", "p-123"})
// → "/orgs/acme/products/p-123/edit"
```

This disambiguates from the variadic form when a single argument is itself a slice.

### Inheriting params from the current request

If the current request already has matching path params in its context, `URLFor` reuses them automatically. You only need to pass params that differ from the current route:

```go
// Inside a handler for /orgs/{org}/products/{id}, where org="acme", id="p-123":
href := structpages.URLFor(ctx, &siblingPage{})
// → "/orgs/acme/products/p-123/sibling"  (params inherited)
```

### Strict mode (default)

`URLFor` is strict by default: if a required param isn't provided and can't be inherited from the request, it returns an error rather than silently emitting a broken URL like `/orgs//products//edit`. Boot-time validation via `URLForValidate` lets you fail fast in tests.

## Ref

When the target page can't be referenced by static type (e.g. you have multiple identical leaf types at different routes), use `Ref`:

```go
href := structpages.URLFor(ctx, structpages.Ref{Name: "admin.productPage"})
```

`Ref` resolves by the fully-qualified node name in the page tree. The lint analyzer also verifies `Ref` strings.

## ID and IDTarget

For HTMX, you want consistent DOM ids that match across server-rendered HTML and client-side `hx-target` selectors:

```go
// In the template:
<div id={ structpages.ID(&commentList{}) }>...</div>

// In a form posting to a partial endpoint:
<form hx-post="/comments" hx-target={ structpages.IDTarget(&commentList{}) }>
```

`ID` returns `"commentList"`; `IDTarget` returns `"#commentList"`. Use `IDTarget` for `hx-target` (it expects a CSS selector) and `ID` for the actual `id` attribute on the element.

## Build-time checking

Install the analyzer:

```bash
go install github.com/jackielii/structpages/tools/lint/cmd/structpages-lint@latest
structpages-lint ./...
```

It catches:
- `URLFor` calls with wrong param count for the target route.
- `Ref` strings that don't resolve to any page.
- `ID` / `IDTarget` calls against types that aren't mounted.
- Mismatches between `URLFor` target and the surrounding `Mount` tree.

Wire it into CI alongside `go vet` for fast feedback.

## See also

- Hand-written API signatures: [`api.md`](./api.md#urlfor).
- End-to-end demonstration: [`examples/url-validation/`](../examples/url-validation/).
- Authoritative reference for library consumers: [`skills/structpages/SKILL.md`](../skills/structpages/SKILL.md) §3 "URL Generation".
