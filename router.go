package srx

import (
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
)

type Router interface {
	Route(path string, fn func(Router))
	HandleMethod(method, path string, handler http.Handler)
}

type chiRouter struct {
	router chi.Router
}

func NewChiRouter(r chi.Router) *chiRouter {
	return &chiRouter{router: r}
}

func (r *chiRouter) Route(path string, fn func(Router)) {
	r.router.Route(path, func(r chi.Router) {
		fn(&chiRouter{router: r})
	})
}

func (r *chiRouter) HandleMethod(method, path string, handler http.Handler) {
	r.router.Method(method, path, handler)
}

func (r *chiRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}

type stdRouter struct {
	prefix string
	router *http.ServeMux
}

func NewStdRouter(router *http.ServeMux) *stdRouter {
	return &stdRouter{router: router}
}

func (r *stdRouter) Route(pattern string, fn func(Router)) {
	subRouter := &stdRouter{
		prefix: path.Join(r.prefix, pattern),
		router: http.NewServeMux(),
	}
	fn(subRouter)
}

func (r *stdRouter) HandleMethod(method, pattern string, handler http.Handler) {
	if method == "" {
		method = http.MethodGet
	}
	pattern = path.Join(r.prefix, pattern)
	pattern = method + " " + pattern
	println("Registering route:", pattern)
	r.router.Handle(pattern, handler)
}

func (r *stdRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}

func stripPrefix(pattern string, h http.Handler) http.Handler {
	if pattern == "" {
		return h
	}
	if !strings.Contains(pattern, "{") && !strings.Contains(pattern, "}") {
		return http.StripPrefix(pattern, h)
	}
	panic("not implemented yet: stripPrefix with path parameters")
	// return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 	parts := strings.Split(pattern, "/")
	// 	for _, part := range parts {
	// 		if
	// 	h.ServeHTTP(w, r)
	// })
}
