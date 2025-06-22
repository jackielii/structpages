package structpages

import (
	"fmt"
	"iter"
	"path"
	"reflect"
	"strings"
)

type PageNode struct {
	Name        string
	Title       string
	Method      string
	Route       string
	Value       reflect.Value
	Props       map[string]reflect.Method
	Components  map[string]reflect.Method
	Config      *reflect.Method
	Middlewares *reflect.Method
	Parent      *PageNode
	Children    []*PageNode
}

func (pn *PageNode) FullRoute() string {
	if pn.Parent == nil {
		return pn.Route
	}
	return path.Join(pn.Parent.FullRoute(), pn.Route)
}

func (pn PageNode) String() string {
	var sb strings.Builder
	sb.WriteString("PageItem{")
	sb.WriteString("\n  name: " + pn.Name)
	sb.WriteString("\n  title: " + pn.Title)
	sb.WriteString("\n  route: " + pn.Route)
	sb.WriteString("\n  middlewares: " + formatMethod(pn.Middlewares))
	if pn.Value.Type().AssignableTo(handlerType) {
		sb.WriteString("\n  is http.Handler: true")
	}
	sb.WriteString("\n  config: " + formatMethod(pn.Config))
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

func (pn *PageNode) All() iter.Seq[*PageNode] {
	return func(yield func(*PageNode) bool) {
		if !walk(pn, yield) {
			return
		}
	}
}
