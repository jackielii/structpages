package srx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Router interface {
	Route(path string, fn func(Router))
	Get(path string, handler http.HandlerFunc)
	Post(path string, handler http.HandlerFunc)
	Put(path string, handler http.HandlerFunc)
	Delete(path string, handler http.HandlerFunc)
}

type chiRouter struct {
	router chi.Router
}

func NewChiRouter(r chi.Router) Router {
	return &chiRouter{router: r}
}

func (r *chiRouter) Route(path string, fn func(Router)) {
	r.router.Route(path, func(r chi.Router) {
		fn(&chiRouter{router: r})
	})
}

func (r *chiRouter) Get(path string, handler http.HandlerFunc) {
	r.router.Get(path, handler)
}

func (r *chiRouter) Post(path string, handler http.HandlerFunc) {
	r.router.Post(path, handler)
}

func (r *chiRouter) Put(path string, handler http.HandlerFunc) {
	r.router.Put(path, handler)
}

func (r *chiRouter) Delete(path string, handler http.HandlerFunc) {
	r.router.Delete(path, handler)
}
