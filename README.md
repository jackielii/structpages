# structpages

Struct Pages

## Features

- Struct based routing
- Templ support built-in
- Built on top of http.ServeMux or chi.Router
- Middleware support
- HTMX partial rendering

## Installation

```shell
go get github.com/jackielii/structpages
```

## Usage

```go
sp := structpages.New()
r := structpages.NewRouter(http.DefaultServeMux)
sp.MountPages(r, "/", index{})
http.ListenAndServe(":8080", r)
```

Check out the [examples](./examples) for more usages.
