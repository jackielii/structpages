package structpages

import (
	"fmt"
	"net/http"
	"strings"
)

func PrintRoutes(route string, v any) string {
	r := &printRouter{}
	var sb strings.Builder
	sp := New(WithMiddlewares(func(h http.Handler, pn *PageNode) http.Handler {
		fmt.Fprintf(&sb, "%s\t%s\t%s\n", pn.Method, pn.FullRoute(), pn.Title)
		// for name := range pn.Components {
		// 	fmt.Fprintf(&r.sb, "- %sComponent %s\n", strings.Repeat(" ", r.indent), name)
		// }
		// fmt.Fprintf(&r.sb, "%sMiddleware for %s\n", strings.Repeat(" ", r.indent), pn.FullRoute())
		return h
	}))
	sp.MountPages(r, v, route, "")
	// return r.sb.String()
	return sb.String()
}

type printRouter struct {
	indent int
	sb     strings.Builder
}

func (p *printRouter) Route(path string, fn func(Router)) {
	fmt.Fprintf(&p.sb, "%s%s\n", strings.Repeat(" ", p.indent), path)
	sp := &printRouter{indent: p.indent + 2}
	fn(sp)
	p.sb.WriteString(sp.sb.String())
}

func (p *printRouter) HandleMethod(method, path string, handler http.Handler) {
	// println("HandleMethod called for method:", method, "and path:", path)
	fmt.Fprintf(&p.sb, "%s%s %s\n", strings.Repeat(" ", p.indent), path, method)
}

var _ Router = (*printRouter)(nil)
