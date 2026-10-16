---
title: Lint
slug: /lint
sidebar_position: 13
---

# structpages-lint

`structpages-lint` is a static analyzer for structpages projects. It is the primary guard behind the rule of thumb: **never write an in-app URL as a string literal** — resolve it by page type so renames break CI instead of drifting.

## Why it matters more as the app grows

The page tree lives in `route` tags, but its references are scattered across every handler, `Props`
method and template, as `URLFor` targets, `Ref` strings, param keys, partial method references and
`hx-*` attributes. The compiler checks none of them: `Ref` strings, param keys and hard-coded `href`s are
all just strings to it. In a small app a broken link shows up the next time you click around. In a large one,
renaming a route, moving a page into a sub-tree or renaming a path param touches dozens of files across
packages. Whatever you miss turns into a 404 or a runtime `URLFor` error in a corner nobody tested.
`structpages-lint` rebuilds the tree the same way `Mount` does and resolves every one of those references
against it, so the whole rename shows up as a list of `file:line` findings before merge.

## Install and run

Pin it as a tool dependency so CI and every developer run the same version:

```bash
go get -tool github.com/jackielii/structpages/tools/lint/cmd/structpages-lint@latest
go tool structpages-lint ./...
```

The linter needs Go 1.26, and `go get -tool` raises your module's `go` line to match. If that isn't possible yet, install it globally instead: `go install github.com/jackielii/structpages/tools/lint/cmd/structpages-lint@latest`.

- Run it from the module root, or pass the packages that hold your `structpages.Mount` calls; it needs them
  to rebuild the tree.
- It exits 0 when clean, 1 when it reports findings, and 2 when packages fail to load or type-check (nothing
  was checked). Wire it into CI alongside `go test`.
- If pages are mounted behind build tags, pass them through with `-tags`.
- In gsx projects, generate first (`go tool gsx generate .`): template filters are checked through the
  generated `.x.go`.

### In CI

```yaml
- run: go tool gsx generate .   # gsx projects: the linter reads the generated .x.go (recurses)
- run: go tool structpages-lint ./...
- run: go tool structpages-lint -tags devtools ./...   # once per build-tag set that mounts pages
```

Findings exit 1 and fail the step. Packages that don't load or type-check exit 2, so a broken build can't pass as clean.

### Several trees

Every `structpages.Mount` and `structpages.Parse` call builds its own tree: the app's, a standalone preview
binary's, and every test fixture's. The runtime resolves `URLFor`, `ID` and `Ref` against the one tree
serving the request, and the linter does the same. A call is checked against the trees mounted by a binary
or test binary that compiles the call in, and it passes if any of them resolves it. A test tree that stubs
a section out, or mounts it at `/` instead of `/orbit`, doesn't produce findings in production code.
A type is ambiguous only when a single tree mounts it at two routes. A finding is worded from a production
tree when one applies.

A helper that takes the page as an interface and passes it on, such as
`func serve(t *testing.T, pages any)` calling `structpages.Mount(mux, pages, "/", "t")`, builds a tree at
each call that hands it a concrete page: `serve(t, appRoot{})` mounts `appRoot`. This works through chains of
helpers, generic helpers, helpers in other packages, and a route passed in as a constant argument. A helper
only ever called with a variable builds nothing the linter can see.

## What it checks

| Category | What it flags |
|---|---|
| `urlfor` | `structpages.URLFor` (and `Within`) chain/composition errors — unmounted type, unknown child type, fragment-before-step, ambiguous bare lookups. |
| `ref` | `structpages.Ref(...)` strings that don't resolve to a page-tree node — including refs stored in struct fields and vars (e.g. a nav table). |
| `idfor` | `structpages.ID` / `IDTarget` method expressions whose receiver is not mounted as a page, or whose method is missing on a chain leaf. The same for the argument of `RenderTarget.Is`, which matches the target an `ID(ctx, X)` element produces: a method expression or method value (`p.Content`) whose receiver isn't mounted, or an argument that isn't a function at all, such as `t.Is(p.Content())`, which calls the component instead of naming it, or a `Ref`/string, which `Is` never matches. |
| `params` | `URLFor` param keys that don't appear in the route pattern (including placeholders in a `?q={q}` fragment) — literal `map[string]any` keys and the constant keys of the key/value form, `URLFor(ctx, p, "id", x)` / `url("id", x)`. |
| `url-attr` | URL-bearing HTML attributes in `.gsx` and `.templ` files (`href`, `action`, `formaction`, `hx-{get,post,put,patch,delete}`, `hx-{push,replace}-url`) whose values are hard-coded internal paths, string concats, `fmt.Sprint*` calls, or gsx `` f`/path/@{x}` `` templates. Allows `https://`, `mailto:`, `#`, and protocol-relative `//…` externals. |
| `route-literal` | `.go` string literals whose value exactly equals a mounted route — e.g. `return "/admin/queues"` — where you should resolve by page type via `URLFor`. Deliberately narrow: exact concrete-route matches only; comparisons (`==`/`switch`) and `Ref(...)` args are skipped; `_test.go` and generated files are skipped. |
| `selector` | A `structpages.Mount` call with no `structpages.WithTargetSelector(...)` option whose tree serves htmx templates. There is no default target selector since v0.9.0, so every htmx partial request to that tree renders the full `Page`. Pass `WithTargetSelector(structpages.HTMXRenderTarget)` for htmx 1/2 or `WithTargetSelector(structpages.HTMXv4RenderTarget)` for htmx 4. See [How `selector` decides](#how-selector-decides). |
| `region` | A page that renders a region carrying another page's component id: in a component method of mounted page `P`, an element whose `id` is `ID(ctx, Q.M)` or whose `hx-target` is `IDTarget(ctx, Q.M)`, for another mounted page `Q`. Requests from that region reach `P` targeting `Q.M`'s id, which `P` has no component for, so `P` swaps its whole Page into the region. See [How `region` decides](#how-region-decides). |
| `mount` | A `Mount`/`Parse` page argument, or a `route`-tagged field, whose type isn't a named struct, so the linter can't rebuild that part of the tree and skips it. Reported while the tree is built, before suppression directives are read, so it can't be suppressed: fix the type. |

The `*StructPages` methods `sp.URLFor(page, ...)`, `sp.ID(v)` and `sp.IDTarget(v)` are checked the same way as
the context functions. `Is` is checked when it is called through the `structpages.RenderTarget` interface, including
through an interface or struct that embeds it; a type of your own that happens to have an `Is` method isn't
checked. A standalone function component (`t.Is(ui.Toast)`) has no place in the tree and always passes, and an
`any`-typed argument is a runtime value the linter doesn't check. A `structpages.Within(ctx, page)` target is resolved like a `URLFor` page; it takes no
params, and a string fragment in its `[]any` chain is reported.

### How `selector` decides

A tree serves htmx when a `.gsx` or `.templ` file in the directory of a package that declares one of its page types
uses one of `hx-get`, `hx-post`, `hx-put`, `hx-patch`, `hx-delete`, `hx-target`, `hx-swap`, `hx-boost` or
`hx-select` as an attribute name, or contains an htmx 4 `<hx-partial>` element. Names match case-insensitively,
with htmx's `data-` prefix (`data-hx-get`) and with an htmx 4 modifier suffix (`hx-target:inherited`). Only names
written in the template count: attributes that reach an element through a spread (`{ attrs... }`), a templ
expression key, or `templ.Attributes` built in Go aren't seen, and neither are templates in a package that declares
no page of the tree, such as a shared `ui` package. The finding names the first htmx use it found.

The Mount is flagged only when its options are known not to include a selector: every option is a direct call to a
`structpages` function, passed inline or through a literal slice (`[]structpages.Option{...}...`). An option the
linter can't see through, such as a variable, an `opts...` spread of a variable, or a call to your own function
that returns an option, could set the selector, so that Mount isn't flagged. A Mount inside a helper that takes the
page as an interface (see [Several trees](#several-trees)) is flagged when any tree built through the helper serves
htmx. Mounts in `_test.go` files are skipped: tests often mount a tree only to resolve URLs and ids, and a test
that sends htmx requests without a selector fails on its own. Suppress with `//structpages:lint:ignore selector` on the Mount line or the
line above.

### How `region` decides

The check looks at every `structpages.ID` / `IDTarget` call, including the gsx `id` and `target` filters and the
`*StructPages` methods, that sits in a component method of a mounted page `P`. A component method is a method
returning a value with `Render(context.Context, io.Writer) error`, which is what gsx's `component (p P) M()`
lowers to. The call is flagged when all of these hold:

- Its component is a method `Q.M`: a method expression (`detail.Content`), a bound method value (`p.d.Content`),
  or the trailing element of the `[]any` chain form (`[]any{section, detail.Content}`), whose leading steps place
  the id but don't change whose component it is. `Q` isn't `P`, and some tree mounts both of them. Unlike the
  resolution checks, a tree that mounts only one of the two, such as a test tree stubbing a section out, doesn't
  clear the finding: the bug happens in every tree that mounts both.
- It is the value of an HTML element's attribute in the `.gsx` source, either as the attribute's last pipeline
  stage (`id={X |> id}`) or as the whole value (`id={structpages.ID(ctx, X)}`). For `ID` the attribute must be
  `id`. For `IDTarget` it must be `hx-target` in any htmx spelling (`hx-target:inherited`, `data-hx-target`), on an
  element other than `<hx-partial>`. An `<hx-partial>` is an out-of-band swap carried in a response: it places
  content and no request targets it, so updating another page's region from one is fine.
- `P`'s own `Props` doesn't call `t.Is(Q.M)` on its `RenderTarget`. That call means `P` handles the borrowed
  region deliberately (typically with `RenderComponent`).

Standalone function components are never flagged: they belong to no page. Neither are components on a type that
isn't a mounted page, such as a shared component type, because the enclosing method has to be on a page.
Suppress a finding with `// structpages:lint:ignore region` on the attribute's line or the line above.

The attribute is matched exactly. gsx writes a `/*line pages.gsx:L:C*/` directive in front of every call it
generates, naming the pipeline stage or seed the call came from, and the linter looks that position up among the
`.gsx` file's attributes. That has some limits:

- Only `.gsx` templates are checked. templ's generated code carries no position mapping back to the `.templ`
  source, so there is no exact way to tell which attribute a call feeds.
- An id built elsewhere and passed in as a value (a prop, a variable, a helper's return) isn't seen. Neither is a
  call wrapped in more Go code inside the attribute, such as `id={"x-" + structpages.ID(ctx, X)}`.
- Attributes on a component tag (`<ui.Box id={…}/>`) are props. What the component does with them isn't known,
  so they aren't checked.
- The `.x.go` files must be regenerated with gsx v0.4.0 or later, as for the other template checks.

### gsx templates

gsx lowers the `url`, `id` and `target` filters to real `structpages.URLFor` / `ID` / `IDTarget` calls in
the generated `.x.go`, so `urlfor`, `ref`, `params` and `idfor` check every `{ page{} |> url("id", x) }`
exactly like hand-written Go. Findings are reported at the `.gsx` line through gsx's `//line` directives.
`url-attr` parses the `.gsx` sources directly with gsx's own parser.

The key/value check follows `URLFor`'s own argument rules. With as many args as placeholders the call is
positional, and so is an odd count; a key that isn't a constant can't be checked. None of those are
reported. An unknown key is reported even when no key matches, because `URLFor` then quietly falls back
to positional fill: `url("nope", 1)` against `/toggle/{id}` renders `/toggle/nope`.

## Suppressing a diagnostic

Place the directive on the same line or the line above. Prefer `//`-style everywhere — Go-style comments never reach the HTML, while `<!-- … -->` HTML comments render into every response:

```go
//structpages:lint:ignore route-literal
return "/legacy-path"
```

```gsx
component Footer() {
    // structpages:lint:ignore url-attr
    <a href="/legacy">…</a>
}
```

The same `//` form works in `.templ` files:

```templ
// structpages:lint:ignore url-attr
<a href="/legacy">…</a>
```

In `.gsx` the directive can be a bare `//` comment, a braced `{/* … */}` comment, or a comment inside the
element's attribute list, and it covers every category. gsx doesn't carry comments into the generated Go,
so for findings on a `|> url` / `|> id` / `|> target` call the analyzer reads the directive from the
`.gsx` line that gsx's `//line` directive points at. That needs gsx v0.4.0 or later, which puts attribute
holes on their real line.

Multiple categories are comma-separated; a bare `structpages:lint:ignore` suppresses every category on the targeted line. Suppress only for a URL that genuinely isn't a structpages route (a path served by another handler or a reverse proxy) — anything else is a bug waiting for a rename.

## What lint can't see

Static analysis can't follow URLs assembled from runtime data or refs behind dynamic dispatch. For those, add a boot-time validation inventory — see [URLFor & ID → Validation](./urlfor.md#validation-no-dangling-urls-in-production).
