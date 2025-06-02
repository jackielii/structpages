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

```go
sp := structpages.New()
r := structpages.NewRouter(http.NewServeMux())
sp.MountPages(r, index{}, "/", "index")
log.Println("Starting server on :8080")
http.ListenAndServe(":8080", r)
```

Check out the [examples](./examples) for more usages.
