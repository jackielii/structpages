package structpages

import (
	"net/http"
	"reflect"
)

// Ref represents a dynamic reference to a page or method by name.
// Use it when static type references aren't available (e.g., configuration-driven
// menus, generic components, or code generation scenarios).
//
// For URLFor, the string can be:
//   - Page name: Ref("UserManagement")
//   - Route path: Ref("/user/management") - must start with /
//
// For IDFor, the string can be:
//   - Qualified method: Ref("PageName.MethodName")
//   - Simple method: Ref("MethodName") - must be unambiguous across all pages
//
// Both URLFor and IDFor return descriptive errors if the reference is invalid,
// providing runtime safety for dynamic references.
//
// Example usage:
//
//	// Dynamic menu from configuration
//	menuItems := []struct{ Page Ref; Label string }{
//	    {Ref("HomePage"), "Home"},
//	    {Ref("UserManagement"), "Users"},
//	}
//	for _, item := range menuItems {
//	    url, err := URLFor(ctx, item.Page)
//	    // Handle error if page doesn't exist
//	}
//
//	// Dynamic component reference
//	targetID, err := IDFor(ctx, Ref("UserManagement.UserList"))
type Ref string

// RenderTarget is what a request wants rendered, as read by the mount's
// TargetSelector. Props can take it as a parameter to load only what the
// request wants, or to decide to render something else.
//
// When Props returns no error, the framework asks the target Is(m) for each
// of the page's components m and renders the one it is, or Page when it is
// none of them. A target that is more than one component is an error.
//
//	func (p dashboard) Props(t RenderTarget, s *Store) (Stats, []Note, error) {
//	    if t.Is(p.Notes) {
//	        notes, err := s.Notes()
//	        return Stats{}, notes, err // renders Notes
//	    }
//	    …                              // renders Page
//	}
//
// ID/Is symmetry: an element rendered with id ID(ctx, X) produces a request
// whose target Is(X), where X is the served page's component, another page's
// component, or a standalone function. For a standalone function this is the
// package-qualified id ("overlay-mount" for overlay.Mount), so
// Is(overlay.Mount) doesn't match a same-named Mount from another package.
// The same holds for one instance of a component: an element rendered with
// id Key(ctx, X, k) produces a request whose target Is(X), and [KeyOf] of
// it returns k.
type RenderTarget interface {
	// Is reports whether the request wants component: a method expression
	// (Page.Content), a bound method value (p.Content) or a standalone
	// function component. It has no side effects.
	Is(component any) bool
}

// TargetSelector reads what a request wants. It returns the RenderTarget
// passed to Props and used to select the component to render.
//
// Without WithTargetSelector, every request wants Page. For htmx, use
// HTMXRenderTarget (htmx 1.x and 2.x) or HTMXv4RenderTarget (htmx 4).
type TargetSelector func(r *http.Request, pn *PageNode) (RenderTarget, error)

// methodRenderTarget is the concrete implementation for method components.
type methodRenderTarget struct {
	name   string
	method reflect.Method
	key    string // a keyed target id's key, for KeyOf
}

func (mrt *methodRenderTarget) Is(method any) bool {
	// Check if this methodRenderTarget has a valid method
	// (it might be a zero method for Props-only pages)
	if mrt.method.Type == nil {
		return false
	}

	info, err := extractMethodInfo(method)
	if err != nil {
		return false
	}

	// For standalone functions, don't match against method targets
	if info.isFunction {
		return false
	}

	// For methods, compare both name and receiver type
	if mrt.method.Type.NumIn() == 0 {
		return false
	}
	return mrt.method.Name == info.methodName && info.receiverIs(mrt.method.Type.In(0))
}

// newMethodRenderTarget creates a RenderTarget for a method component.
func newMethodRenderTarget(name string, method *reflect.Method) RenderTarget {
	return &methodRenderTarget{
		name:   name,
		method: *method,
	}
}
