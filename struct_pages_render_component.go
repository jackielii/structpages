package structpages

import (
	"context"
	"errors"
	"io"
	"net/http"
)

// errRenderComponent is the error RenderComponent returns: it carries the
// component to render in place of the page's own.
type errRenderComponent struct {
	component component
}

func (e *errRenderComponent) Error() string {
	return "should render component from target"
}

// RenderComponent returns an error that tells the framework to render comp
// instead of the page's default component. Return it from Props (or an
// error-returning ServeHTTP), typically after the RenderTarget picked a
// partial:
//
//	func (p DashboardPage) Props(r *http.Request, sel RenderTarget) (DashboardProps, error) {
//		if sel.Is(UserStatsWidget) {
//			return DashboardProps{}, RenderComponent(UserStatsWidget(loadUserStats()))
//		}
//		...
//	}
//
// comp is any value with Render(ctx, w): a gsx.Node, a templ.Component, or
// a page's partial called with its arguments, such as p.TodoList(todos).
// Building the component at the call site keeps its arguments type-checked.
func RenderComponent(comp interface {
	Render(context.Context, io.Writer) error
},
) error {
	return &errRenderComponent{component: comp}
}

// handleRenderComponentError renders the component carried by a
// RenderComponent error and reports whether err was one.
func (sp *StructPages) handleRenderComponentError(w http.ResponseWriter, r *http.Request, err error) bool {
	var renderErr *errRenderComponent
	if !errors.As(err, &renderErr) {
		return false
	}
	sp.render(w, r, renderErr.component)
	return true
}
