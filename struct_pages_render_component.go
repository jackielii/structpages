package structpages

import (
	"context"
	"errors"
	"io"
	"net/http"
)

// errRenderComponent is the error RenderComponent returns: it carries the
// components to render in place of the page's own.
type errRenderComponent struct {
	components []component
}

func (e *errRenderComponent) Error() string {
	return "should render component from target"
}

// RenderComponent returns an error that breaks Props' normal flow: instead
// of the component the request wants, the framework renders comp, followed
// by more in order, as one response. Return it from Props or an
// error-returning ServeHTTP:
//
//	func (p DashboardPage) Props(t RenderTarget, s *Store) (Stats, error) {
//		if t.Is(p.Notes) {
//			notes, err := s.Notes()
//			if err != nil {
//				return Stats{}, RenderComponent(p.NotesUnavailable(err))
//			}
//		}
//		…
//	}
//
// Several components make one response that updates several regions, for
// example the main swap plus htmx 4 <hx-partial> blocks:
//
//	return Stats{}, RenderComponent(p.Notes(notes), p.NotesCount(len(notes)))
//
// Each argument is any value with Render(ctx, w): a gsx.Node, a
// templ.Component, or a page's component called with its arguments, such
// as p.TodoList(todos). Building them at the call site keeps their
// arguments type-checked.
func RenderComponent(comp interface {
	Render(context.Context, io.Writer) error
}, more ...interface {
	Render(context.Context, io.Writer) error
},
) error {
	comps := make([]component, 0, 1+len(more))
	comps = append(comps, comp)
	for _, c := range more {
		comps = append(comps, c)
	}
	return &errRenderComponent{components: comps}
}

// handleRenderComponentError renders the components carried by a
// RenderComponent error and reports whether err was one.
func (sp *StructPages) handleRenderComponentError(w http.ResponseWriter, r *http.Request, err error) bool {
	var renderErr *errRenderComponent
	if !errors.As(err, &renderErr) {
		return false
	}
	sp.render(w, r, renderErr.components...)
	return true
}
