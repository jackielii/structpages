---
title: templ and html/template
slug: /templ
sidebar_position: 6.5
---

# templ and html/template

structpages only needs a component with `Render(context.Context, io.Writer) error`, so any renderer works. Every pattern in [gsx Patterns](./gsx.md) — Props, partials, cross-page rendering, [testing with `PageContext`](./testing.md) — applies as is; only the markup differs. The `url`, `id` and `target` filters are gsx-only: other renderers call `URLFor`, `ID` and `IDTarget` themselves.

## templ

`templ.Component` works unchanged.

### Syntax mapping

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

### Places that need a plain string

`templ.Attributes` map values and script arguments can't take `(string, error)`. Resolve the value where an error can be returned instead of panicking:

- compute it in `Props` (or a Go helper returning `(T, error)`) and pass it in the props struct;
- or give the child component an explicit parameter for the URL and set the attribute on the element inside it, where `{ structpages.URLFor(...) }` is allowed.

## html/template

A page component returns any value with a `Render` method, so a small adapter type puts a template behind it. [`examples/html-template`](../examples/html-template/) parses one template set per page and returns the entry to execute:

```go
type tpl struct {
    page  string // which parsed set
    entry string // which template in it
    data  any
}

func (p tpl) Render(_ context.Context, w io.Writer) error {
    t, ok := pageTmpls[p.page]
    if !ok {
        return fmt.Errorf("unknown page %q", p.page)
    }
    return t.ExecuteTemplate(w, p.entry, p.data)
}

type post struct{}

func (post) Page(props postProps) tpl     { return tpl{page: "post", entry: "layout/public", data: props} }
func (post) Comments(props postProps) tpl { return tpl{page: "post", entry: "post/comments-list", data: props.Comments} }
```

Partials work as with gsx: each method returning a `tpl` is a component that `HX-Target` can select.

Templates have no render `ctx`, so give them URLs through a template function. The example binds one to the mounted `*StructPages` once, at parse time, resolving pages by `Ref`:

```go
sp, err := structpages.Mount(mux, index{}, "/", "Index")
if err != nil {
    log.Fatalf("mount: %v", err)
}
funcs := template.FuncMap{
    "urlFor": func(name string, args ...any) (string, error) {
        return sp.URLFor(structpages.Ref(name), args...)
    },
}
```

```html
<a hx-get="{{ urlFor "index" }}" hx-target="main" hx-push-url="true">Home</a>
```

A template function returning `(string, error)` stops execution with that error, so a bad name fails the render. `sp.URLFor` has no request context, so params aren't filled from the current URL: pass every path param explicitly. The `Ref` strings are data to the compiler, and the linter doesn't read `.html` templates, so a boot-time URL inventory ([Testing](./testing.md#every-url-resolves)) is what catches a renamed page here.

## Lint

`structpages-lint` checks `URLFor`/`ID`/`IDTarget`/`Ref` calls in templ's generated Go like any other Go, and its `url-attr` category scans `.templ` files for hard-coded internal URLs. It doesn't read `html/template` files. Suppress with the Go-style directive, which is stripped from the output (see [Lint](./lint.md#suppressing-a-diagnostic)).
