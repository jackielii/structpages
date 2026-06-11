---
title: Routing
slug: /routing
sidebar_position: 4
---

# Routing Patterns and Struct Tags

Routes are struct fields with `route:` tags. Format: `route:"[METHOD] /path [Title]"`.

```go
type pages struct {
    home    `route:"/{$}   Home"`             // exact root match
    about   `route:"/about About"`            // all methods (default)
    create  `route:"POST /create Create"`     // POST only
    detail  `route:"/item/{itemId} Item"`     // path parameter
    files   `route:"/files/{path...} Files"`  // wildcard
}
```

## Route tag format

1. **Path only**: `route:"/path"` — all HTTP methods, no page title.
2. **Path with title**: `route:"/path Page Title"` — all methods, title "Page Title".
3. **Method and path**: `route:"POST /path"` — POST only, no title.
4. **Full format**: `route:"PUT /path Update Page"` — PUT only, title "Update Page".

Supported HTTP methods: `GET`, `HEAD`, `POST`, `PUT`, `PATCH`, `DELETE`, `CONNECT`, `OPTIONS`, `TRACE`. If no method is given, the route accepts all methods (internally stored as `ALL`).

Only the `route:` tag is read by the framework — any other tag on a route field is ignored.

## `/{$}` — exact match

Go's ServeMux treats a trailing `/` as a prefix match: `route:"/"` would swallow every unmatched path under the mount point. Use `/{$}` for "exactly this path" — most commonly the index page of a [page group](./concepts.md):

```go
type adminPages struct {
    dashboard `route:"/{$} Dashboard"`    // matches /admin/ exactly
    users     `route:"/users Users"`      // matches /admin/users
}
```

## Path parameters

Path parameters use Go 1.22+ `http.ServeMux` syntax. Extract them in the Props method via `r.PathValue` — they are not passed as function arguments:

```go
type pages struct {
    userProfile `route:"/users/{userId} User Profile"`
    blogPost    `route:"/blog/{year}/{month}/{slug}"`
}

func (p userProfile) Props(r *http.Request) (UserProfileProps, error) {
    userID := r.PathValue("userId") // "123" if URL is /users/123
    return UserProfileProps{UserID: userID}, nil
}

templ (p userProfile) Page(props UserProfileProps) {
    @layout() {
        <h1>User Profile for { props.UserID }</h1>
    }
}
```

**Name path params specifically — `{itemId}`, not `{id}`.** Nested routes compose into a single pattern, so two levels each declaring `{id}` collide: ServeMux rejects duplicate wildcard names in a pattern (`/order/{id}/item/{id}` panics at mount), and `URLFor`'s `map[string]any` params couldn't tell them apart anyway. Specific names compose cleanly: `/order/{orderId}/item/{itemId}`.

## Nested routes

Create hierarchical URL structures by nesting structs:

```go
type pages struct {
    admin adminPages `route:"/admin Admin Panel"`
}

type adminPages struct {
    dashboard `route:"/{$} Dashboard"`        // -> /admin/
    users     `route:"/users User List"`      // -> /admin/users
    settings  `route:"/settings Settings"`    // -> /admin/settings
}
```

A struct like `adminPages` that has no render of its own — no `Page` or `ServeHTTP`, only child pages — is a **page group**. It is never served at its bare path; `/admin` 307-redirects to `/admin/`, which its `/{$}` page serves. `URLFor` on a page group returns the index child's URL with the canonical trailing slash (see [URLFor](./urlfor.md)).

Children register before parents on the mux, so nested-route conflicts resolve correctly without you ordering anything by hand.

## Wildcard routes and static assets

Use the wildcard form for prefix subtrees — the framework joins nested paths with `path.Join`, which strips trailing slashes, so `route:"/static/"` would register as an exact match, not a prefix. `{path...}` is the right shape:

```go
type adminPages struct {
    dashboard `route:"/{$} Dashboard"`
    users     `route:"/users Users"`
    Assets    staticFiles `route:"GET /static/{path...} Assets"`
}

//go:embed all:static
var staticFS embed.FS

type staticFiles struct{}

func (staticFiles) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    root, err := fs.Sub(staticFS, "static")
    if err != nil {
        http.NotFound(w, r)
        return
    }
    http.ServeFileFS(w, r, root, r.PathValue("path"))
}
```

This keeps the module self-contained: `/admin` and `/admin/static/*` register together, with no separate `mux.Handle` call to keep in sync.

## Never write an in-app URL as a string literal

Resolve URLs by page type — `structpages.URLFor(ctx, somePage{})` — so a moved route breaks the build (or the boot) instead of silently dangling. The [`structpages-lint`](./lint.md) `route-literal` check flags `.go` string literals that exactly equal a mounted route, and `url-attr` flags hard-coded paths in `.templ` URL attributes.
