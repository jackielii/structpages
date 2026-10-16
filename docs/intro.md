---
title: Introduction
slug: /intro
sidebar_position: 1
---

# Introduction

`structpages` is a Go web framework library that defines routes via struct tags and methods on top of `http.ServeMux`. It minimises boilerplate for HTML-first apps using [gsx](https://gsxhq.github.io/) and HTMX (templ and `html/template` work too), and gives you type-safe URL generation and dependency injection without giving up the standard library.

## Status

**Beta.** The API has settled and the library is battle-tested in production, carrying medium-to-large applications. Remaining breaking changes before v1 will be rare and called out in release notes.

## Why structpages

If you've built a Go web app with `http.ServeMux`, you've written code like:

```go
mux.HandleFunc("GET /products/{productId}", handleProductGet)
mux.HandleFunc("POST /products/{productId}/comments", handleProductCommentCreate)
mux.HandleFunc("DELETE /products/{productId}/comments/{commentId}", handleProductCommentDelete)
```

That works, but the route structure lives only in string literals. `structpages` lets you express the same hierarchy as Go structs, so the router and your handler types stay in lockstep:

```go
type product struct {
    comments `route:"/comments Comments"`
}

type productPages struct {
    product `route:"/products/{productId} Product"`
}
```

Each page renders through a **Props method** (loads data) and a **Page method** (a gsx component), or handles the request imperatively with a **handler method** (`ServeHTTP`) — whichever fits the request shape. URLs are generated from page types (`{product{} |> url}` in gsx, `structpages.URLFor(ctx, product{})` in Go) rather than string concatenation, and the [`structpages-lint`](./lint.md) analyzer catches dangling references at build time.

A request is matched to a page, its Props method loads data, and a page component renders: the whole page, or one region for an HTMX request. [Concepts](./concepts.md#request-lifecycle) walks through it.

## What's next

- [Quick Start](./quick-start.md) for a minimal working app, end to end.
- [Concepts](./concepts.md) for the vocabulary the rest of these docs use.
- [Routing](./routing.md) for the full tag syntax (HTTP methods, path params, titles).
- [gsx Patterns](./gsx.md) for shared layouts and the Props pattern.
- [HTMX Integration](./htmx.md) for partial rendering driven by `hx-target`.
- [Testing](./testing.md) for rendering pages and components in unit tests.
- [Examples](./examples/index.md) for full working apps in `examples/` you can clone.
- [Page Response Patterns](./supported-flows.md) for choosing between the Props/Page and `ServeHTTP` shapes.
