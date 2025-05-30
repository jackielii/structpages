package srx

import (
	"net/http"
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

type stdRouter struct {
	router *http.ServeMux
}

func NewStdRouter(router *http.ServeMux) *stdRouter {
	return &stdRouter{router: router}
}

func (r *stdRouter) Route(path string, fn func(Router)) {
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}
	subRouter := NewStdRouter(http.NewServeMux())
	fn(subRouter)
	handler := http.StripPrefix(path[:len(path)-1], subRouter.router)
	r.router.Handle(path, handler)
}

func (r *stdRouter) HandleMethod(method, path string, handler http.Handler) {
	if method == "" {
		method = http.MethodGet
	}
	path = method + " " + path
	r.router.Handle(path, handler)
}
