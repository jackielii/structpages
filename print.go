package srx

import "net/http"

type printRouter struct{}

func (p *printRouter) Route(path string, fn func(Router)) {
	println("Route called for path:", path)
}

func (p printRouter) HandleMethod(method, path string, handler http.Handler) {
	println("HandleMethod called for method:", method, "and path:", path)
}

var _ Router = (*printRouter)(nil)
