package structpages

import (
	"net/http"
)

// Router is an interface for registering HTTP routes.
// It provides a method for registering handlers for specific HTTP methods.
// This simplified interface allows structpages to work with different routing implementations.
type Router interface {
	HandleMethod(method, path string, handler http.Handler)
}

type stdRouter struct {
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

func (r *stdRouter) HandleMethod(method, pattern string, handler http.Handler) {
	if method != methodAll && method != "" {
		pattern = method + " " + pattern
	}
	r.router.Handle(pattern, handler)
}

func (r *stdRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}
