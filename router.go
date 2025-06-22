package structpages

import (
	"net/http"
	"path"
)

// Router is an interface for registering HTTP routes.
// It provides methods for creating route groups and registering handlers for specific HTTP methods.
// This interface allows structpages to work with different routing implementations.
type Router interface {
	Route(path string, fn func(Router))
	HandleMethod(method, path string, handler http.Handler)
}

type stdRouter struct {
	prefix string
	router *http.ServeMux
}

// NewRouter creates a new router that wraps http.ServeMux.
// If router is nil, it uses http.DefaultServeMux.
// The returned router implements the Router interface and can be used with StructPages.MountPages.
//
// Example:
//
//	mux := http.NewServeMux()
//	router := structpages.NewRouter(mux)
//	sp.MountPages(router, pages{}, "/", "My App")
func NewRouter(router *http.ServeMux) *stdRouter {
	if router == nil {
		router = http.DefaultServeMux
	}
	return &stdRouter{router: router}
}

func (r *stdRouter) Route(pattern string, fn func(Router)) {
	fn(&stdRouter{
		prefix: path.Join(r.prefix, pattern),
		router: r.router,
	})
}

func (r *stdRouter) HandleMethod(method, pattern string, handler http.Handler) {
	pattern = path.Join(r.prefix, pattern)
	if method != methodAll && method != "" {
		pattern = method + " " + pattern
	}
	r.router.Handle(pattern, handler)
}

func (r *stdRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}
