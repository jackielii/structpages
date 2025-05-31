package chirouter

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackielii/structpages"
)

type chiRouter struct {
	router chi.Router
}

func NewChiRouter(r chi.Router) *chiRouter {
	return &chiRouter{router: r}
}

func (r *chiRouter) Route(path string, fn func(structpages.Router)) {
	r.router.Route(path, func(r chi.Router) {
		fn(&chiRouter{router: r})
	})
}

func (r *chiRouter) HandleMethod(method, path string, handler http.Handler) {
	if method == "ALL" || method == "" {
		r.router.Handle(path, handler)
	} else {
		r.router.Method(method, path, handler)
	}
}

func (r *chiRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}
