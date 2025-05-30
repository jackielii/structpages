package srx

import "net/http"

type printeRouter struct{}

// Delete implements Router.
func (p *printeRouter) Delete(path string, handler http.HandlerFunc) {
	println("Delete called for path:", path)
}

// Get implements Router.
func (p *printeRouter) Get(path string, handler http.HandlerFunc) {
	println("Get called for path:", path)
}

// Post implements Router.
func (p *printeRouter) Post(path string, handler http.HandlerFunc) {
	println("Post called for path:", path)
}

// Put implements Router.
func (p *printeRouter) Put(path string, handler http.HandlerFunc) {
	println("Put called for path:", path)
}

// Route implements Router.
func (p *printeRouter) Route(path string, fn func(Router)) {
	println("Route called for path:", path)
}

var _ Router = (*printeRouter)(nil)

// func (pr *StructPages) registerPageItem(router Router, pc *parseContext, page *PageNode, parentRoute string) {
