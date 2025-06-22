# structpages

[![CI](https://github.com/jackielii/structpages/actions/workflows/ci.yml/badge.svg)](https://github.com/jackielii/structpages/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/jackielii/structpages.svg)](https://pkg.go.dev/github.com/jackielii/structpages)
[![codecov](https://codecov.io/gh/jackielii/structpages/branch/main/graph/badge.svg)](https://codecov.io/gh/jackielii/structpages)
[![Go Report Card](https://goreportcard.com/badge/github.com/jackielii/structpages)](https://goreportcard.com/report/github.com/jackielii/structpages)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Struct Pages provides a way to define routing using struct tags and methods. It
integrates with the [http.ServeMux], allowing you to quickly build up pages and
components without too much boilerplate.

**Status**: **Alpha** - This package is in early development and may have breaking changes in the future. Currently used in a medium-sized project, but not yet battle-tested in production.

## Features

- Struct based routing
- Templ support built-in
- Built on top of http.ServeMux
- Middleware support
- HTMX partial rendering

## Installation

```shell
go get github.com/jackielii/structpages
```

## Usage

```templ
type index struct {
	product `route:"/product Product"`
	team    `route:"/team Team"`
	contact `route:"/contact Contact"`
}

templ (index) Page() {
	@html() {
		<h1>Welcome to the Index Page</h1>
		<p>Navigate to the product, team, or contact pages using the links below:</p>
	}
}
...
```

Route definitions are done using struct tags in for form of `[method] path [Title]`. Valid patterns:

- `/path` - For all methods that match `/path` without a title
- `POST /path` - For POST requests matching `/path`
- `/path Awesome Product` - For ALL requests matching `/path` with a title "Awesome Product"

```go
sp := structpages.New()
r := structpages.NewRouter(http.NewServeMux())
sp.MountPages(r, index{}, "/", "index")
log.Println("Starting server on :8080")
http.ListenAndServe(":8080", r)
```

Check out the [examples](./examples) for more usages.

**TODO**:

- [ ] Document routing patterns and struct tags
- [ ] Document middleware usage
- [ ] Document HTMX integration
- [ ] Document `UrlFor` functionality
- [ ] Document more templ patterns
