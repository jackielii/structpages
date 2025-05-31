package structpages

import (
	"net/http"
	"path"
)

type Router interface {
	Route(path string, fn func(Router))
	HandleMethod(method, path string, handler http.Handler)
}

type stdRouter struct {
	prefix string
	router *http.ServeMux
}

func NewRouter(router *http.ServeMux) *stdRouter {
	if router == nil {
		router = http.DefaultServeMux
	}
	return &stdRouter{router: router}
}

func (r *stdRouter) Route(pattern string, fn func(Router)) {
	// println("Registering route group", "pattern", path.Join(r.prefix, pattern))
	fn(&stdRouter{
		prefix: path.Join(r.prefix, pattern),
		router: r.router,
	})
}

func (r *stdRouter) HandleMethod(method, pattern string, handler http.Handler) {
	if method == "" {
		method = http.MethodGet
	}
	pattern = path.Join(r.prefix, pattern)
	pattern = method + " " + pattern
	// println("Registering route", "method", method, "pattern", pattern)
	r.router.Handle(pattern, handler)
}

func (r *stdRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}
