# Fragment guard

A small project tracker whose fragments can't be opened as bare markup. It
goes with [Guarding Fragment URLs](../../docs/fragment-guard.md).

```bash
go run .
```

Open http://localhost:8080 and try the fragment links on the home page, as
boosted links or in a new tab. Each one lands on the page that shows it:

| Fragment | URL | Lands on | How |
|---|---|---|---|
| task list | `/projects/42/tasks` | `/projects/42/` | default: the ancestor page |
| rename dialog | `/projects/42/tasks/7/rename` | `/projects/42/` | default |
| comments | `/comments?project=42&task=7` | `/projects/42/?task=7#comments` | `Landing`, built from the query |
| mention | `/mentions/1` | `/projects/42/?task=7#comments` | `SelfLanding`: Props looks it up |
| someone else's mention | `/mentions/3` | a 404 page | `SelfLanding`, with an access check |

Files:

- `pages.gsx`: the routes. Pages have `Page`; fragments have `Props` returning
  `RenderComponent`; the rename dialog (GET) and its save (POST) are separate
  route types.
- `guard.go`: the navigation guard, the same file as `../htmx/navguard.go`.
- `main_test.go`: each landing case as a new tab, a boosted link and a partial,
  plus a route walk that opens every GET as a document.

After editing `pages.gsx`:

```bash
go tool gsx generate . && go tool gsx fmt -w .
```
