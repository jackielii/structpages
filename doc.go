// structpages provides a way to define routing using struct and method tags.
// It integrates with the [http.ServeMux] or [chi.Router], allowing you to quickly
// build up pages and components without too much boilerplate.
//
// It supports templ components as built-in, but also allow other templating
// engines to be used.
//
// An example of using structpages can be:
//
//	sp := structpages.New()
//	r := structpages.NewRouter(nil) // nil for http.DefaultServeMux
//	sp.MountPages(r, index{}, "/", "index")
//	http.ListenAndServe(":8080", r)
//
// You can then define you pages in a `page.templ` file like this:
//
//	type index struct{}
//	templ (index) Page() {
//		<html>
//		...
//		</html>
//	}
//
// Checkout more examples in the examples/ folder
package structpages
