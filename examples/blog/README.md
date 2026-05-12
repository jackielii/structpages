# blog — comprehensive example with module-based component organization

A working blog (public reader + admin CMS) built with structpages, templ, and
HTMX. The goal is to demonstrate the framework's "newest patterns" alongside a
package layout that scales: each feature is its own Go package; shared UI
primitives live in `ui/components`; the layout shell lives in `ui/layout`.

This is the structure you'd reach for in a real React/Next-style app, applied
to Go + templ.

## Run

```sh
templ generate -include-version=false
go run .
# open http://localhost:8080  — admin login: admin / admin
```

The store is in-memory and seeded at boot, so restarting the server resets all
posts, comments, and sessions.

## Package layout

```
.
├── main.go                    bootstrap, Mount, custom error handler
├── store/                     in-memory data layer (sync.RWMutex)
├── auth/                      cookie-session Service + RequireAdmin middleware
├── ui/
│   ├── layout/                PublicShell, AdminShell (children-slot layout)
│   └── components/            Button, Input, Textarea, Alert, Card, Pagination,
│                              ErrorPage/ErrorBlock — standalone templ functions
├── blog/                      public reader feature
│   ├── routes.go
│   ├── home.templ
│   ├── post.templ
│   ├── category.templ
│   ├── search.templ
│   ├── comment.go             ServeHTTP for POST /posts/{slug}/comments
│   └── components.templ       PostCard, PostMeta, CommentsList (feature-local)
└── admin/                     authenticated CMS feature
    ├── routes.go              admin.Pages with Middlewares() returning RequireAdmin,
    │                          plus an Assets field that owns /admin/static/*
    ├── static/                module-scoped assets (admin-logo.svg)
    ├── login.go + login.templ LoginPage — sibling of admin.Pages, not a child
    ├── logout.go
    ├── dashboard.templ        Props + RenderTarget refreshing widgets independently
    ├── posts.go + posts.templ list / new / edit / create / update / delete
    ├── users.go + users.templ list / create / delete
    └── components.templ       StatsGrid, RecentPostsCard, PostsTable
```

The dependency graph is one-way: `main → {blog, admin} → ui/{layout,components}
→ store`, with `auth` as a peer of `store`. Cross-feature links (e.g. the public
header pointing at `/admin/login`) use `structpages.Ref("loginPage")` so the
`ui/layout` package never needs to import `admin`.

## Patterns demonstrated

| Pattern | Where to look |
|---|---|
| Nested route hierarchies (3 levels: `/admin/posts/{id}/edit`) | `admin/routes.go`, `admin/posts.go` |
| Dependency injection via `WithArgs(store, authSvc)` consumed by `Props`, `ServeHTTP`, `Middlewares` | `main.go` and every `Props` method |
| Page-level `Middlewares()` with DI returning `RequireAdmin` | `admin/routes.go` |
| `Props(r, target RenderTarget, *store.Store)` with conditional partial loads | `admin/dashboard.templ`, `blog/search.templ` |
| Standalone function components as HTMX targets — `target.Is(StatsGrid)` then `RenderComponent` | `admin/dashboard.templ`, `admin/components.templ` |
| `Page()` + `Content()` split (HTMX swaps `#content`, full doc on direct nav) | `blog/home.templ`, `admin/dashboard.templ` |
| `ServeHTTP` form handler with redirect or HTMX partial re-render | `blog/comment.go`, `admin/posts.go` |
| Custom `WithErrorHandler` rendering a styled error component (cross-package) | `main.go` + `ui/components` |
| `URLFor` with path params and query-string templates `[]any{p, "?page={page}"}` | `blog/category.templ` |
| `ID`/`IDTarget` wiring for `hx-target` | throughout |
| Cross-package component composition (`admin` imports `ui/layout` + `ui/components`) | every `.templ` |
| `Ref("loginPage")` for cross-feature links to avoid import cycles | `ui/layout/layout.templ` |
| Module-owned `/static/` subtree — `staticFiles` field on `admin.Pages` with `route:"/static/{path...}"` and `http.ServeFileFS(w, r, fs, r.PathValue("path"))` (no separate `pub.Handle("/admin/static/", …)` in `main`, no `StripPrefix`, handler is unaware of its mount path) | `admin/routes.go`, `admin/static/admin-logo.svg`, `ui/layout/layout.templ` |

## Quick verification (server running)

```sh
# Public
curl -sf http://localhost:8080/ | grep -q "Recent Posts"
curl -sf http://localhost:8080/posts/welcome | grep -q "Comments"
curl -sf "http://localhost:8080/categories/guides?page=1" | grep -q "Guides"
curl -sf "http://localhost:8080/search?q=props" | grep -q "result"

# Custom 404
curl -s http://localhost:8080/posts/nope | grep -q "couldn't find"

# Auth gating
curl -si http://localhost:8080/admin/ | head -1                # expect 303 → /admin/login
curl -sc cookies -d "username=admin&password=admin" http://localhost:8080/admin/login
curl -sb cookies http://localhost:8080/admin/ | grep -q "Dashboard"

# Module-owned static assets — admin.Pages.Assets serves /admin/static/{path...}
curl -sIb cookies http://localhost:8080/admin/static/admin-logo.svg | head -1   # expect HTTP/1.1 200
curl -si http://localhost:8080/admin/static/admin-logo.svg | head -1            # expect HTTP/1.1 303 (RequireAdmin → login)

# HTMX partials (each refreshes only its widget — check Network tab in DevTools)
curl -sb cookies -H "HX-Request: true" -H "HX-Target: stats-grid" http://localhost:8080/admin/
curl -sb cookies -H "HX-Request: true" -H "HX-Target: recent-posts-card" http://localhost:8080/admin/

# Form action + sibling re-render
curl -s -H "HX-Request: true" -H "HX-Target: comments-list" \
     -d "author=jane&body=Nice" http://localhost:8080/posts/welcome/comments | grep -q "Nice"
```

## Not for production

This example skips concerns that real apps need:

- Passwords are stored in plaintext (use bcrypt/argon2)
- Session ids live in process memory (use a signed/encrypted cookie or a session store)
- No CSRF tokens
- Tailwind is loaded from the Play CDN — fine for prototyping, ship a built CSS in production
- The store is a process-local map — replace with a real database
