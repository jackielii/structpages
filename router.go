package srx

import (
	"net/http"
	"path"

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
	if router == nil {
		router = http.DefaultServeMux
	}
	return &stdRouter{router: router}
}

func (r *stdRouter) Route(pattern string, fn func(Router)) {
	subRouter := &stdRouter{
		prefix: path.Join(r.prefix, pattern),
		router: r.router,
	}
	fn(subRouter)
}

func (r *stdRouter) HandleMethod(method, pattern string, handler http.Handler) {
	if method == "" {
		method = http.MethodGet
	}
	pattern = path.Join(r.prefix, pattern)
	pattern = method + " " + pattern
	r.router.Handle(pattern, handler)
}

func (r *stdRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}
