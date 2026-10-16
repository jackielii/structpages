---
layout: home

hero:
  name: structpages
  text: Struct-based routing for Go web apps
  tagline: Built on http.ServeMux. gsx and HTMX first. Type-safe URLs.
  actions:
    - theme: brand
      text: Get Started
      link: /intro
    - theme: alt
      text: GitHub
      link: https://github.com/jackielii/structpages

features:
  - title: Routes are struct fields
    details: Nest structs to nest routes; route tags hold the path and title. Mount builds the tree onto any http.ServeMux, with no registration calls.
  - title: Partial rendering without dispatch code
    details: An HTMX request for one region renders just that component. Props loads only what that region needs, and the full page and the partial share one component.
  - title: Broken links fail CI
    details: URLs, ids and Refs resolve from page types, and structpages-lint checks every one against the route tree, so a renamed route or param fails the build instead of 404ing.
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
