---
layout: home

hero:
  name: structpages
  text: Struct-based routing for Go web apps
  tagline: Built on http.ServeMux. gsx and HTMX first. Type-safe URLs. Beta — battle-tested in production.
  actions:
    - theme: brand
      text: Get Started
      link: /intro
    - theme: alt
      text: GitHub
      link: https://github.com/jackielii/structpages

features:
  - title: Struct-based routing
    details: Define routes as Go struct fields with tags. Nesting structs creates nested route hierarchies. No registration calls.
  - title: gsx + HTMX
    details: Pages render with gsx (templ and html/template work too). HTMX-aware partial rendering via the default HTMXRenderTarget returns the right component for hx-target without manual dispatch.
  - title: Type-safe URLs
    details: URLFor builds URLs from page types; ID and IDTarget build matching HTML ids and selectors. The structpages-lint analyzer catches mismatches before they ship.
---

## Quick look

```gsx
type index struct {
	product `route:"/product Product"`
	team    `route:"/team Team"`
}
type product struct{}
type team struct{}

component (p index) Page() {
	<html>
		<body>
			<h1>Welcome</h1>
			<a href={product{} |> url}>Product</a>
			<a href={team{} |> url}>Team</a>
		</body>
	</html>
}

component (p product) Page() {
	<h1>Product</h1>
}

component (p team) Page() {
	<h1>Team</h1>
}
```

```go
func main() {
	mux := http.NewServeMux()
	if _, err := structpages.Mount(mux, index{}, "/", "Home"); err != nil {
		log.Fatal(err)
	}
	log.Fatal(http.ListenAndServe(":8080", mux))
}
```

[See the full walkthrough →](/quick-start)

## Resources

- [pkg.go.dev reference](https://pkg.go.dev/github.com/jackielii/structpages)
- Claude Code plugin: `/plugin marketplace add jackielii/structpages`
- [llms.txt](https://raw.githubusercontent.com/jackielii/structpages/main/llms.txt)
