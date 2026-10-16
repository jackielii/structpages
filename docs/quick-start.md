---
title: Quick Start
slug: /quick-start
sidebar_position: 2
---

# Quick Start

A minimal `structpages` app with two pages and [gsx](https://gsxhq.github.io/)-rendered HTML. Run it in under five minutes. (templ and `html/template` work too; see [Using templ](./templ.md).)

## 1. Create the project

You need Go 1.26 or later.

```bash
mkdir hello-structpages && cd hello-structpages
go mod init hello-structpages
go get github.com/jackielii/structpages
go get github.com/gsxhq/gsx
```

Pin the gsx CLI as a `tool` dep so collaborators get the right version:

```bash
go get -tool github.com/gsxhq/gsx/cmd/gsx@latest
```

Then register structpages' URL and id helpers as gsx filters. Create `gsx.toml` at the module root (gsx uses
the nearest one walking up from the package):

```toml
[filters]
url    = "github.com/jackielii/structpages.URLFor"
id     = "github.com/jackielii/structpages.ID"
target = "github.com/jackielii/structpages.IDTarget"
```

## 2. Define your pages

Create `pages.gsx`:

```gsx
package main

type pages struct {
	home  `route:"/{$} Home"`
	about `route:"/about About"`
}

type home struct{}
type about struct{}

component (p home) Page() {
	<html>
		<body>
			<h1>Hello, structpages!</h1>
			<p><a href={about{} |> url}>About</a></p>
		</body>
	</html>
}

component (p about) Page() {
	<html>
		<body>
			<h1>About</h1>
			<p><a href={home{} |> url}>Home</a></p>
		</body>
	</html>
}
```

Two things to note:

- **`/{$}` means "exactly `/`".** A bare `route:"/"` would be a ServeMux catch-all prefix that swallows every unmatched path; `/{$}` matches only the root.
- **Links use the `url` filter, not string literals.** `{about{} |> url}` is `structpages.URLFor(ctx, about{})`; gsx passes the render `ctx` and returns any error from `Render`. When a route moves, every link follows — and [`structpages-lint`](./lint.md) flags any hard-coded internal path you write by accident.

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
    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

## 4. Generate and run

```bash
go tool gsx generate .
go mod tidy
go run .
```

`gsx generate` writes `pages.x.go` next to `pages.gsx`; rerun it after editing `.gsx` files.

Open [http://localhost:8080](http://localhost:8080). You should see "Hello, structpages!" and a link to `/about`.

## What just happened

- `pages` is a **page group** — a struct whose fields each carry a route tag, with no rendering of its own. Embedding means promoted methods, but the dispatcher *skips* promoted methods, so the `Page()` defined on each inner type is the one that runs.
- `component (p home) Page()` is a gsx method component: `gsx generate` turns it into a Go method `func (p home) Page() gsx.Node`, and a `gsx.Node` is a component (it has `Render(ctx, w)`).
- `Mount(mux, pages{}, "/", "Site")` walks the struct and registers each route on `mux`.
- Each request is dispatched to the matching **page**, which renders its **Page method** (a gsx component) into the response.

## Next steps

- [Concepts](./concepts.md) for the vocabulary the rest of these docs use.
- [Routing](./routing.md) for the full tag syntax (HTTP methods, path params, titles).
- [gsx Patterns](./gsx.md) for shared layouts and the Props pattern.
- [HTMX Integration](./htmx.md) for partial rendering driven by `hx-target`.
- [Testing](./testing.md) for rendering pages and components in unit tests.
- [Examples](./examples/index.md) for full working apps in `examples/` you can clone.
