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

A Mount that passes its options as a spread, `Mount(mux, root{}, "/", "App", opts...)`, builds its tree like any
other, so its pages get every check. A Mount (or helper call) whose route isn't a constant builds no tree: the
linter can't tell where the tree is mounted, so its pages get no tree-based check from that call (`urlfor`,
`ref`, `idfor`, `params`, `route-literal`, `selector`, `region`).

## What it checks

| Category | What it flags |
|---|---|
| `urlfor` | `structpages.URLFor` (and `Within`) chain/composition errors — unmounted type, unknown child type, fragment-before-step, ambiguous bare lookups. |
| `ref` | `structpages.Ref(...)` strings that don't resolve to a page-tree node — including refs stored in struct fields and vars (e.g. a nav table). |
| `idfor` | `structpages.ID` / `IDTarget` / `Key` / `KeyTarget` method expressions whose receiver is not mounted as a page, or whose method is missing on a chain leaf. The same for the argument of `RenderTarget.Is`, which matches the target an `ID(ctx, X)` element produces: a method expression or method value (`p.Content`) whose receiver isn't mounted, or an argument that isn't a function at all, such as `t.Is(p.Content())`, which calls the component instead of naming it, or a `Ref`/string, which structpages' own target selectors never match (a custom `RenderTarget` might). |
| `params` | `URLFor` param keys that don't appear in the route pattern (including placeholders in a `?q={q}` fragment) — literal `map[string]any` keys and the constant keys of the key/value form, `URLFor(ctx, p, "id", x)` / `url("id", x)`. |
| `escape` | A `URLFor` value already escaped with `net/url` — `url.QueryEscape(x)`, `url.PathEscape(x)` or `v.Encode()` passed positionally, as a key/value pair or in a literal map, `url("q", url.QueryEscape(q))` in gsx. `URLFor` escapes every value for its place in the URL, so it would be escaped twice. Pass the value raw; put an encoded `url.Values` in the template instead, `[]any{page, "?" + v.Encode()}`. Needs no page tree. |
| `url-attr` | URL-bearing HTML attributes in `.gsx` and `.templ` files (`href`, `action`, `formaction`, `hx-{get,post,put,patch,delete}`, `hx-{push,replace}-url`) whose values are hard-coded internal paths, string concats, `fmt.Sprint*` calls, or gsx `` f`/path/@{x}` `` templates. Allows `https://`, `mailto:`, `#`, and protocol-relative `//…` externals. |
| `route-literal` | `.go` string literals whose value exactly equals a mounted route — e.g. `return "/admin/queues"` — where you should resolve by page type via `URLFor`. Deliberately narrow: exact concrete-route matches only; comparisons (`==`/`switch`), `Ref(...)` args and the route argument of a `Mount`/`Parse` call (or of a helper forwarding to one), which defines the route, are skipped; `_test.go` and generated files are skipped. |
| `selector` | A `structpages.Mount` call with no `structpages.WithTargetSelector(...)` option whose tree serves htmx templates. There is no default target selector since v0.9.0, so every htmx partial request to that tree renders the full `Page`. Pass `WithTargetSelector(structpages.HTMXRenderTarget)` for htmx 1/2 or `WithTargetSelector(structpages.HTMXv4RenderTarget)` for htmx 4. See [How `selector` decides](#how-selector-decides). |
| `region` | A page that renders a region carrying another page's component id: in a component method of mounted page `P`, an element whose `id` is `ID(ctx, Q.M)` or `Key(ctx, Q.M, k)`, or whose `hx-target` is `IDTarget(ctx, Q.M)` or `KeyTarget(ctx, Q.M, k)`, for another page `Q` mounted in the same tree. A request that targets the region and comes back to `P` asks for a component `P` doesn't have, so `P` swaps its whole Page into the region. An element whose own request goes to another page (a lazy load from `Q`, a form posting to `Q`) is fine. See [How `region` decides](#how-region-decides). |
| `keyed` | An element `id` or `hx-target` that appends a runtime value to a generated id: the hand-built id of one instance of a component, such as `` id=f`@{Page.Row \|> id}-@{row.ID}` ``, `` hx-target=f`@{Page.Row \|> target}-@{row.ID}` ``, or `id={rowID + "-" + row.ID}` with `rowID` from `ID`. No component has that id, so the htmx target selectors can't match a request targeting it to `Page.Row`, and `Props` can't read the key back. Generate it with `Key` / `KeyTarget`: `` @{Page.Row \|> key(row.ID)} `` / `keytarget`, whose key `Props` reads with `KeyOf`. A constant suffix, such as the `-document` of a sub-element's id, isn't flagged. Needs no page tree. See [How `keyed` decides](#how-keyed-decides). |
| `mount` | A `Mount`/`Parse` page argument, or a `route`-tagged field, whose type isn't a named struct, so the linter can't rebuild that part of the tree and skips it. Reported while the tree is built, before suppression directives are read, so it can't be suppressed: fix the type. |

The `*StructPages` methods `sp.URLFor(page, ...)`, `sp.ID(v)`, `sp.IDTarget(v)`, `sp.Key(v, k)` and `sp.KeyTarget(v, k)` are checked the same way as
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
`structpages` function, passed inline or through a literal slice (`[]structpages.Option{...}...`).
`WithTargetSelector(nil)` counts as no selector, since the runtime ignores a nil one. An option the
linter can't see through, such as a variable, an `opts...` spread of a variable, or a call to your own function
that returns an option, could set the selector, so that Mount isn't flagged. A Mount inside a helper that takes the
page as an interface (see [Several trees](#several-trees)) is flagged when any tree built through the helper serves
htmx. Mounts in `_test.go` files are skipped: tests often mount a tree only to resolve URLs and ids, and a test
that sends htmx requests without a selector fails on its own. Suppress with `//structpages:lint:ignore selector` on the Mount line or the
line above.

### How `region` decides

The check looks at every `structpages.ID` / `IDTarget` / `Key` / `KeyTarget` call, including the gsx `id`, `target`, `key` and `keytarget` filters and the
`*StructPages` methods, that sits in a component method of a mounted page `P`. A component method is a method
returning a value with `Render(context.Context, io.Writer) error`, which is what gsx's `component (p P) M()` and
templ's `templ (p P) M()` lower to. The call is flagged when all of these hold:

- Its component is a method `Q.M`: a method expression (`detail.Content`), a bound method value (`p.d.Content`),
  or the trailing element of the `[]any` chain form (`[]any{section, detail.Content}`), whose leading steps place
  the id but don't change whose component it is. `Q` is the type `structpages.ID` resolves the method to at
  runtime: for a method expression, the type it is written on, even when the method is promoted from an embedded
  field (`wrap.Content` is `wrap`'s); for a method value, the type that declares the method (`p.Content` on a `wrap`
  embedding `detail` is `detail`'s). `Q` isn't `P`, and some tree mounts both of them. Unlike the
  resolution checks, a tree that mounts only one of the two, such as a test tree stubbing a section out, doesn't
  clear the finding: the bug happens in every tree that mounts both.
- It is the value of an HTML element's attribute in the `.gsx` or `.templ` source: in gsx, either the attribute's
  last pipeline stage (`id={X |> id}`) or the whole value (`id={structpages.ID(ctx, X)}`); in templ, the whole
  value (`id={ structpages.ID(ctx, X) }`). For `ID` and `Key` the attribute must be
  `id`. For `IDTarget` and `KeyTarget` it must be `hx-target` in any htmx spelling (`hx-target:inherited`, `data-hx-target`), on an
  element other than `<hx-partial>`. An `<hx-partial>` is an out-of-band swap carried in a response: it places
  content and no request targets it, so updating another page's region from one is fine.
- The requests that carry that target come back to `P`:
  - If the element has request attributes of its own (`hx-get`, `hx-post`, `hx-put`, `hx-patch`, `hx-delete`, in
    any htmx spelling, including in an in-tag `if`), each must be a `URLFor` call (the gsx `url` filter, or a
    hand-written `structpages.URLFor` as the whole value) that resolves to `P`. A lazy load from `Q`
    (`hx-get={Q{} |> url}`), a form posting to another page, or any request whose URL is a literal, a variable
    or some other call isn't flagged: the linter can't tell it comes back to `P`, and `url-attr` already
    reports literal URLs.
  - If it has none, it is a region whose descendants send the requests (a reload button inside it, or children
    inheriting `hx-target:inherited`), and the finding stands.
  - An element that also takes a spread of attributes (`{ attrs... }`) isn't flagged: its request attributes
    are only known at runtime.
- `P`'s own `Props` doesn't call `t.Is(Q.M)` on its `RenderTarget`. That call means `P` handles the borrowed
  region deliberately (typically with `RenderComponent`). Nothing else counts: a `t.Is(Q.M)` in a `ServeHTTP` that
  takes the `RenderTarget` by injection doesn't, and neither does a hand-written comparison of the `HX-Target`
  header, such as a `partialTarget` helper reading `r.Header`, even when it handles the region. Move such code to `t.Is(Q.M)`, which is what `Is` is for, or suppress the finding
  with `structpages:lint:ignore region`:

  ```go
  // Not seen as handling the region: the linter can't follow header comparisons.
  func (p acePage) Props(r *http.Request) (aceProps, error) {
      if partialTarget(r) == detailContentID {
          return aceProps{}, structpages.RenderComponent(detail.DetailPage{}.Content(…))
      }
      …
  }

  // Handled: Props asks the RenderTarget about DetailPage.Content.
  func (p acePage) Props(t structpages.RenderTarget) (aceProps, error) {
      if t.Is(detail.DetailPage.Content) {
          return aceProps{}, structpages.RenderComponent(detail.DetailPage{}.Content(…))
      }
      …
  }
  ```

Standalone function components are never flagged: they belong to no page. Neither are components on a type that
isn't a mounted page, such as a shared component type, because the enclosing method has to be on a page.
Suppress a finding with `// structpages:lint:ignore region` on the attribute's line or the line above, in the
`.gsx` or `.templ` file, where the finding is reported.

The attribute is matched exactly:

- **gsx:** gsx writes a `/*line pages.gsx:L:C*/` directive in front of every call it generates, naming the
  pipeline stage or seed the call came from. The linter looks that position up among the `.gsx` file's
  attributes.
- **templ:** templ writes no position mapping into `_templ.go`. The linter instead runs the templ generator it is
  built with on the `.templ` file, formats the result as `templ generate` does, and uses the generator's source map
  only if the result is byte-for-byte the `_templ.go` on disk. The one exception is the
  `// templ: version:` comment line, which may differ or be missing (`templ generate -include-version=false`): it
  is a comment, so it changes no code. If anything else differs (another
  templ version that generates different code, `-include-timestamp`, a hand edit, or a formatter run over the
  generated file: `goimports` regroups its imports), the file's positions can't be trusted, and its attributes are
  skipped by `region` and `keyed` without a finding. To have them checked, regenerate with the templ version the
  linter is built with (the `github.com/a-h/templ` version in the linter's `go.mod`), and leave `*_templ.go` out of
  any `goimports` or `gofmt -s` run, as you would gsx's `*.x.go`.

Limits:

- An id built elsewhere and passed in as a value (a prop, a variable, a helper's return) isn't seen. Neither is a
  call wrapped in more Go code inside the attribute, such as `id={"x-" + structpages.ID(ctx, X)}`.
- Attributes on a component tag (`<ui.Box id={…}/>`) are props. What the component does with them isn't known,
  so they aren't checked.
- The `.x.go` files must be regenerated with gsx v0.4.0 or later, as for the other template checks.
- The attribute's code is compared with the generated call token by token, so layout, line breaks and comments
  inside the value don't matter. In gsx, a filter is joined only when it is the attribute's sole pipeline stage
  (`{X |> id}`): after other stages its argument is no longer the component.

### How `keyed` decides

The check reads the value of every `id` and `hx-target` attribute (in any htmx spelling) on an HTML element in
the `.gsx` and `.templ` sources: a gsx `` f`…` `` template part by part, and an expression value (`id={…}`)
split at string concatenation. Each expression is joined exactly to the code generated for it, as for
[`region`](#how-region-decides). A part is a generated id when it is a call of `ID`, `IDTarget`, `Key` or
`KeyTarget` (a gsx filter, a hand-written call, or a `*StructPages` method), or a local variable assigned
once, from such a call, and never again: `{{ rowID, err := structpages.ID(ctx, Page.Row) }}`. A local
variable assigned once from another expression stands for that expression, so
`{{ id := rowID + "-" + row.ID }}` followed by `id={id}` is seen too.

What follows the generated id decides it:

- A compile-time constant, literal text or a named constant, is the same in every render. The id is a
  sub-element's (`-document`, `-title`), not an instance's, and isn't flagged.
- Anything else (a variable, a field, a call) is a runtime value. The id differs per render, which makes it
  an instance's id, and it is flagged.

Where it counts:

- In an `id`, the whole value is the id: the generated id must start it, and a runtime value anywhere after
  it counts. `` id=f`@{Page.Row |> id}-@{row.ID}-title` `` is flagged; the fix keeps the role:
  `` id=f`@{Page.Row |> key(row.ID)}-title` ``.
- In an `hx-target`, the value is a selector and the generated `#id` one part of it. Only a runtime value
  inside the id's token counts: after literal identifier characters that extend the id (letters, digits,
  `-`, `_`, CSS escapes), before any character that ends it (a space, `[`, `:`, `.`, `>`, …).
  `` hx-target=f`@{Page.Row |> target}-@{row.ID}` `` is flagged,
  `` hx-target=f`@{Page.Row |> target} [data-id="@{row.ID}"]` `` isn't. A runtime value right after the
  target, `` f`@{Page.Row |> target}@{sel}` ``, may start another part of the selector, so it isn't flagged.
  `ID` and `Key` count in an `hx-target` only after a literal `#`.
- A `Key` or `KeyTarget` id followed by a runtime value is flagged too: `KeyOf` reads the appended text back
  as part of the key. Pass the whole key to `key(…)`.

Limits:

- An id built in Go outside the template and passed in as a value (a prop, a `Props` result, a helper's
  return) isn't seen, and neither is one built with `fmt.Sprintf`.
- In an `id`, text before the generated id isn't looked at.
- Attributes on a component tag (`<ui.Box id={…}/>`) are props, so they aren't checked.
- A `.templ` file is checked only when its `_templ.go` is what the templ version the linter is built with
  generates, as for `region`.

Suppress a finding with `// structpages:lint:ignore keyed` on the attribute's line or the line above, in the
`.gsx` or `.templ` file.

### gsx templates

gsx lowers the `url`, `id`, `target`, `key` and `keytarget` filters to real `structpages.URLFor` / `ID` / `IDTarget` / `Key` / `KeyTarget` calls in
the generated `.x.go`, so `urlfor`, `ref`, `params`, `escape` and `idfor` check every `{ page{} |> url("id", x) }`
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
