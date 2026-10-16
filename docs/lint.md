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

### Several trees

Every `structpages.Mount` and `structpages.Parse` call builds its own tree: the app's, a standalone preview
binary's, and every test fixture's. The runtime resolves `URLFor`, `ID` and `Ref` against the one tree
serving the request, and the linter does the same. A call is checked against the trees mounted by a binary
or test binary that compiles the call in, and it passes if any of them resolves it. A test tree that stubs
a section out, or mounts it at `/` instead of `/orbit`, doesn't produce findings in production code.
A type is ambiguous only when a single tree mounts it at two routes. A finding is worded from a production
tree when one applies.

## What it checks

| Category | What it flags |
|---|---|
| `urlfor` | `structpages.URLFor` (and `Within`) chain/composition errors — unmounted type, unknown child type, fragment-before-step, ambiguous bare lookups. |
| `ref` | `structpages.Ref(...)` strings that don't resolve to a page-tree node — including refs stored in struct fields and vars (e.g. a nav table). |
| `idfor` | `structpages.ID` / `IDTarget` method expressions whose receiver is not mounted as a page, or whose method is missing on a chain leaf. |
| `params` | `URLFor` param keys that don't appear in the route pattern (including placeholders in a `?q={q}` fragment) — literal `map[string]any` keys and the constant keys of the key/value form, `URLFor(ctx, p, "id", x)` / `url("id", x)`. |
| `url-attr` | URL-bearing HTML attributes in `.gsx` and `.templ` files (`href`, `action`, `formaction`, `hx-{get,post,put,patch,delete}`, `hx-{push,replace}-url`) whose values are hard-coded internal paths, string concats, `fmt.Sprint*` calls, or gsx `` f`/path/@{x}` `` templates. Allows `https://`, `mailto:`, `#`, and protocol-relative `//…` externals. |
| `route-literal` | `.go` string literals whose value exactly equals a mounted route — e.g. `return "/admin/queues"` — where you should resolve by page type via `URLFor`. Deliberately narrow: exact concrete-route matches only; comparisons (`==`/`switch`) and `Ref(...)` args are skipped; `_test.go` and generated files are skipped. |

The `*StructPages` methods `sp.URLFor(page, ...)`, `sp.ID(v)` and `sp.IDTarget(v)` are checked the same way as
the context functions. A `structpages.Within(ctx, page)` target is resolved like a `URLFor` page; it takes no
params, and a string fragment in its `[]any` chain is reported.

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
// structpages:lint:ignore url-attr
<a href="/legacy">…</a>
```

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
