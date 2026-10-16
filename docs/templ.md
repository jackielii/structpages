---
title: Using templ
slug: /templ
sidebar_position: 6.5
---

# Using templ

structpages only needs a component with `Render(context.Context, io.Writer) error`, so `templ.Component` works unchanged. Every pattern in [gsx Patterns](./gsx.md) — Props, partials, cross-page rendering, testing with `PageContext` — applies as is; only the markup differs.

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

templ has no pipeline filters, so call `URLFor`, `ID` and `IDTarget` directly. Attribute expressions accept `(string, error)` and return the error from `Render` — the same hoisting gsx does — so no `must()` helper and no `urlFor`/`idFor` wrapper is needed. `href` takes a plain string; `templ.SafeURL` is not needed for URLs from `URLFor`.

```templ
templ (p index) Content(props IndexProps) {
    <form hx-post={ structpages.URLFor(ctx, addTodo{}) }
          hx-target={ structpages.IDTarget(ctx, index.TodoList) }>
        <input name="text"/>
    </form>
    <div id={ structpages.ID(ctx, index.TodoList) }>
        @p.TodoList(props.Todos)
    </div>
}
```

## Places that need a plain string

`templ.Attributes` map values and script arguments can't take `(string, error)`. Resolve the value where an error can be returned instead of panicking:

- compute it in `Props` (or a Go helper returning `(T, error)`) and pass it in the props struct;
- or give the child component an explicit parameter for the URL and set the attribute on the element inside it, where `{ structpages.URLFor(...) }` is allowed.

## Lint

`structpages-lint` checks `URLFor`/`ID`/`IDTarget`/`Ref` calls in templ's generated Go like any other Go, and its `url-attr` category scans `.templ` files for hard-coded internal URLs. Suppress with the Go-style directive, which is stripped from the output (see [Lint](./lint.md#suppressing-a-diagnostic)).
