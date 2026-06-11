---
title: Lint
slug: /lint
sidebar_position: 13
---

# structpages-lint

`structpages-lint` is a static analyzer for structpages projects. It is the primary guard behind the rule of thumb: **never write an in-app URL as a string literal** — resolve it by page type so renames break CI instead of drifting.

## Install and run

```bash
go install github.com/jackielii/structpages/tools/lint/cmd/structpages-lint@latest
structpages-lint ./...
```

Wire it into CI alongside `go test`. If your project uses build tags, pass them through with `-tags`.

## What it checks

| Category | What it flags |
|---|---|
| `urlfor` | `structpages.URLFor` chain/composition errors — unknown child type, fragment-before-step, ambiguous bare lookups. |
| `ref` | `structpages.Ref(...)` strings that don't resolve to a page-tree node — including refs stored in struct fields and vars (e.g. a nav table). |
| `id`, `idtarget` | `structpages.ID` / `IDTarget` method expressions whose receiver is not mounted as a page. |
| `params` | `URLFor` params that don't appear in the route pattern. |
| `url-attr` | URL-bearing HTML attributes in `.templ` files (`href`, `action`, `formaction`, `hx-{get,post,put,patch,delete}`, `hx-{push,replace}-url`) whose values are hard-coded internal paths, string concats, or `fmt.Sprint*` calls. Allows `https://`, `mailto:`, `#`, and protocol-relative `//…` externals. |
| `route-literal` | `.go` string literals whose value exactly equals a mounted route — e.g. `return "/admin/queues"` — where you should resolve by page type via `URLFor`. Deliberately narrow: exact concrete-route matches only; comparisons (`==`/`switch`) and `Ref(...)` args are skipped; `_test.go` and generated files are skipped. |

## Suppressing a diagnostic

Place the directive on the same line or the line above. Prefer `//`-style in both `.go` and `.templ` — Go-style comments are stripped from generated HTML, while `<!-- … -->` HTML comments render into every response:

```go
//structpages:lint:ignore route-literal
return "/legacy-path"
```

```templ
// structpages:lint:ignore url-attr
<a href="/legacy">…</a>
```

Multiple categories are comma-separated; a bare `structpages:lint:ignore` suppresses every category on the targeted line.

## What lint can't see

Static analysis can't follow URLs assembled from runtime data or refs behind dynamic dispatch. For those, add a boot-time validation inventory — see [URLFor & ID → Validation](./urlfor.md#validation-no-dangling-urls-in-production).
