---
title: Quick Start
slug: /quick-start
sidebar_position: 2
---

# Quick Start

A minimal `structpages` app with two pages and Templ-rendered HTML. Run it in under five minutes.

## 1. Create the project

```bash
mkdir hello-structpages && cd hello-structpages
go mod init hello-structpages
go get github.com/jackielii/structpages
go get github.com/a-h/templ
```

Pin the templ CLI as a `tool` dep so collaborators get the right version:

```bash
go get -tool github.com/a-h/templ/cmd/templ@latest
```

## 2. Define your pages

Create `pages.templ`:

```templ
package main

import "github.com/jackielii/structpages"

type home struct{}

templ (home) Page() {
    <html>
        <body>
            <h1>Hello, structpages!</h1>
            <p><a href={ structpages.URLFor(ctx, about{}) }>About</a></p>
        </body>
    </html>
}

type about struct{}

templ (about) Page() {
    <html>
        <body>
            <h1>About</h1>
            <p><a href={ structpages.URLFor(ctx, home{}) }>Home</a></p>
        </body>
    </html>
}

type pages struct {
    home  `route:"/{$}   Home"`
    about `route:"/about About"`
}
```

Two things to note:

- **`/{$}` means "exactly `/`".** A bare `route:"/"` would be a ServeMux catch-all prefix that swallows every unmatched path; `/{$}` matches only the root.
- **Links use `URLFor`, not string literals.** Templ attributes accept `(string, error)` directly, so `href={ structpages.URLFor(ctx, about{}) }` works as-is. When a route moves, every link follows — and [`structpages-lint`](./lint.md) flags any hard-coded internal path you write by accident.

## 3. Wire up `main.go`

```go
package main

import (
    "log"
    "net/http"

    "github.com/jackielii/structpages"
)

func main() {
    mux := http.NewServeMux()
    if _, err := structpages.Mount(mux, pages{}, "/", "Site"); err != nil {
        log.Fatal(err)
    }
    log.Println("Listening on :8080")
    if err := http.ListenAndServe(":8080", mux); err != nil {
        log.Fatal(err)
    }
}
```

## 4. Generate Templ code and run

```bash
go tool templ generate -include-version=false
go run .
```

Open [http://localhost:8080](http://localhost:8080). You should see "Hello, structpages!" and a link to `/about`.

## What just happened

- `pages` is a **page group** — a struct whose fields each carry a route tag, with no rendering of its own. Embedding means promoted methods, but the dispatcher *skips* promoted methods, so the `Page()` defined on each inner type is the one that runs.
- `Mount(mux, pages{}, "/", "Site")` walks the struct and registers each route on `mux`.
- Each request is dispatched to the matching **page**, which renders its **Page method** (a Templ component) into the response.

## Next steps

- [Concepts](./concepts.md) for the vocabulary the rest of these docs use.
- [Routing](./routing.md) for the full tag syntax (HTTP methods, path params, titles).
- [Templ Patterns](./templ.md) for shared layouts and the Props pattern.
- [HTMX Integration](./htmx.md) for partial rendering driven by `hx-target`.
- [Examples](./examples/index.md) for full working apps in `examples/` you can clone.
