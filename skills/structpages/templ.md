# structpages with templ

structpages only needs a component with `Render(context.Context, io.Writer) error`, so `templ.Component`
works unchanged: every rule in SKILL.md applies. Only the markup differs. Read this file only for a templ
project; new work in this repository uses gsx.

## Syntax mapping

| gsx | templ |
|---|---|
| `component (p Index) Page(props indexProps) { … }` | `templ (p Index) Page(props indexProps) { … }` |
| `<Layout><p.Content props={props}/></Layout>` | `@Layout() { @p.Content(props) }` |
| `children gsx.Node` + `{ children }` | `{ children... }` |
| `href={Detail{} \|> url(params)}` | `href={ structpages.URLFor(ctx, Detail{}, params) }` |
| `id={Index.List \|> id}` | `id={ structpages.ID(ctx, Index.List) }` |
| `hx-target={Index.List \|> target}` | `hx-target={ structpages.IDTarget(ctx, Index.List) }` |
| `RenderComponent(<Widget n={n}/>)` | `RenderComponent(Widget(n))` |

templ has no pipeline filters, so call the functions directly. Attribute expressions accept
`(string, error)` and return the error from `Render`; this is the same hoisting gsx does, so no `must()`
helper and no `urlFor`/`idFor` wrapper is needed. `href` takes a plain string; `templ.SafeURL` is not needed
for URLs from `URLFor`.

## Places that need a plain string

`templ.Attributes` map values and script arguments cannot take `(string, error)`. Resolve the value where an
error can be returned instead of panicking:

- compute it in `Props` (or a Go helper returning `(T, error)`) and pass it in the props struct;
- or give the child component an explicit parameter for the URL and set the attribute on the element
  inside it, where `{ structpages.URLFor(...) }` is allowed.

## Lint

`structpages-lint`'s `url-attr` category scans `.templ` files for hard-coded internal URLs in `href`,
`action`, `formaction`, `hx-get`/`post`/`put`/`patch`/`delete` and `hx-push-url`/`hx-replace-url`. In
`.templ` files prefer the Go-style directive, which is stripped from the output:

```templ
// structpages:lint:ignore url-attr
<a href="/legacy">Legacy</a>
```
