package structpages

import (
	"context"
	"fmt"
	"iter"
	"net/http"
	"path"
	"reflect"
	"strings"
)

// CurrentPage returns the PageNode of the route currently being served, or
// nil when ctx did not originate from a structpages request — for example a
// bare context, or one wrapped only by [StructPages.PageContext] in a test.
//
// structpages sets this on the request context before rendering a matched
// Props/Component page, so a handler, a Props method, or any templ component
// they render can ask "which page am I?" without threading the node through
// every call. The node is the matched leaf; walk [PageNode.Parent] to reach
// its mount ancestors (e.g. shared layout chrome computing active-nav state
// by testing whether a nav target is an ancestor of the current page).
//
// Pages served by their own ServeHTTP do not set this — the value is for the
// component/Props render path.
func CurrentPage(ctx context.Context) *PageNode {
	return currentPageCtx.Value(ctx)
}

// PageNode represents a page in the routing tree.
// It contains metadata about the page including its route, title, and registered methods.
// PageNodes form a tree structure with parent-child relationships representing nested routes.
type PageNode struct {
	Name          string
	Title         string
	Method        string
	Route         string
	routeSegments []segment // Pre-parsed route segments for performance
	Value         reflect.Value
	Props         map[string]reflect.Method
	Components    map[string]reflect.Method
	Middlewares   *reflect.Method
	Parent        *PageNode
	Children      []*PageNode

	// idPath is the kebab-cased field-name path from the root (root
	// excluded) down to this node — the stable identity used to build
	// element ids. Populated by parseContext.assignIDPaths.
	idPath []string
	// idCompactSuffix is "" when this node's leaf name is unique across
	// the tree, otherwise "-<hash>" derived from idPath. It disambiguates
	// the compact (leaf-only) id form used when the full path is too long.
	idCompactSuffix string
}

// FullRoute returns the complete route path for this page node,
// including all parent routes. For example, if a parent has route "/admin"
// and this node has route "/users", FullRoute returns "/admin/users".
func (pn *PageNode) FullRoute() string {
	if pn.Parent == nil {
		return pn.Route
	}
	return path.Join(pn.Parent.FullRoute(), pn.Route)
}

// urlTarget returns the node whose route should represent this node in a
// generated URL.
//
// A directly-routable page (one ServeMux registers a handler for at its own
// FullRoute) represents itself. A pure subtree container — a struct that only
// groups child routes and has no render logic of its own — is never served at
// its bare path: ServeMux only matches its subtree, and the bare path
// 307-redirects to add the trailing slash. For such a container we resolve to
// its index child (the `/{$}` route), whose URL carries the canonical trailing
// slash. A container with no index child has no canonical page, so it is left
// as-is rather than fabricating a slash.
func (pn *PageNode) urlTarget() *PageNode {
	if pn.routable() || len(pn.Children) == 0 {
		return pn
	}
	if idx := pn.indexChild(); idx != nil {
		return idx
	}
	return pn
}

// routable reports whether ServeMux registers a handler at this node's own
// FullRoute. It mirrors buildHandler: a node is routable if it carries render
// methods (Components/Props) or implements an ServeHTTP handler. A node that is
// only a parent of other routes is not routable.
func (pn *PageNode) routable() bool {
	if len(pn.Components) > 0 || len(pn.Props) > 0 {
		return true
	}
	return pn.hasServeHTTP()
}

// hasServeHTTP reports whether the page value declares its own (non-promoted)
// ServeHTTP method, on either the value or pointer receiver. This is the same
// detection asHandler uses to decide a node is an http.Handler.
func (pn *PageNode) hasServeHTTP() bool {
	if !pn.Value.IsValid() {
		return false
	}
	st, pt := pn.Value.Type(), pn.Value.Type()
	if st.Kind() == reflect.Pointer {
		st = st.Elem()
	} else {
		pt = reflect.PointerTo(st)
	}
	if m, ok := st.MethodByName("ServeHTTP"); ok && !isPromotedMethod(&m) {
		return true
	}
	if m, ok := pt.MethodByName("ServeHTTP"); ok && !isPromotedMethod(&m) {
		return true
	}
	return false
}

// indexChild returns the child that owns this node's index route — the one
// registered at `/{$}`, which matches the parent path with a trailing slash.
// When the index is method-split across several children (e.g. GET landing +
// POST submit), the GET/ALL child wins since URLFor generates navigable URLs;
// any method-matched index is used as a fallback. Returns nil when no child is
// an index route.
func (pn *PageNode) indexChild() *PageNode {
	var fallback *PageNode
	for _, c := range pn.Children {
		if strings.Trim(c.Route, "/") != "{$}" {
			continue
		}
		if c.Method == http.MethodGet || c.Method == methodAll {
			return c
		}
		if fallback == nil {
			fallback = c
		}
	}
	return fallback
}

// getRouteSegments returns pre-parsed route segments, parsing on-demand if not cached
func (pn *PageNode) getRouteSegments() []segment {
	if pn.routeSegments != nil {
		// Return a copy to avoid mutations
		result := make([]segment, len(pn.routeSegments))
		copy(result, pn.routeSegments)
		return result
	}
	// Fallback: parse on demand (shouldn't normally happen)
	segments, _ := parseSegments(pn.FullRoute())
	return segments
}

// String returns a human-readable representation of the PageNode,
// useful for debugging. It includes all properties and recursively
// formats child nodes with proper indentation.
func (pn PageNode) String() string {
	var sb strings.Builder
	sb.WriteString("PageItem{")
	sb.WriteString("\n  name: " + pn.Name)
	sb.WriteString("\n  title: " + pn.Title)
	sb.WriteString("\n  route: " + pn.Route)
	sb.WriteString("\n  middlewares: " + formatMethod(pn.Middlewares))
	if pn.Value.IsValid() && pn.Value.Type().AssignableTo(handlerType) {
		sb.WriteString("\n  is http.Handler: true")
	}
	if len(pn.Components) == 0 {
		sb.WriteString("\n  components: []")
	}
	for name, comp := range pn.Components {
		sb.WriteString("\n  component: " + name + " -> " + formatMethod(&comp))
	}
	for name, props := range pn.Props {
		sb.WriteString("\n  prop: " + name + " -> " + formatMethod(&props))
	}
	for i, child := range pn.Children {
		fmt.Fprintf(&sb, "\n  child %d:", i+1)
		childStr := strings.TrimRight(child.String(), "\n")
		for _, line := range strings.SplitAfter(childStr, "\n") {
			sb.WriteString("  " + line)
		}
	}
	sb.WriteString("\n}")
	return sb.String()
}

func walk(p *PageNode, yield func(*PageNode) bool) bool {
	if !yield(p) {
		return false
	}
	for _, child := range p.Children {
		if !walk(child, yield) {
			return false
		}
	}
	return true
}

// All returns an iterator that walks through this PageNode and all its descendants
// in depth-first order. This is useful for traversing the entire page tree.
//
// Example:
//
//	for node := range pageNode.All() {
//	    fmt.Println(node.FullRoute())
//	}
func (pn *PageNode) All() iter.Seq[*PageNode] {
	return func(yield func(*PageNode) bool) {
		if !walk(pn, yield) {
			return
		}
	}
}
