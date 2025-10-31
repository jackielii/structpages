package structpages

import (
	"fmt"
	"iter"
	"path"
	"reflect"
	"strings"
)

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
