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

func (p PageNode) String() string {
	var sb strings.Builder
	sb.WriteString("PageItem{")
	sb.WriteString("\n  name: " + p.Name)
	sb.WriteString("\n  title: " + p.Title)
	sb.WriteString("\n  route: " + p.Route)
	sb.WriteString("\n  middlewares: " + formatMethod(p.Middlewares))
	if p.Value.Type().AssignableTo(handlerType) {
		sb.WriteString("\n  is http.Handler: true")
	}
	sb.WriteString("\n  config: " + formatMethod(p.Config))
	if len(p.Components) == 0 {
		sb.WriteString("\n  components: []")
	}
	for name, comp := range p.Components {
		sb.WriteString("\n  component: " + name + " -> " + formatMethod(&comp))
	}
	for name, props := range p.Props {
		sb.WriteString("\n  prop: " + name + " -> " + formatMethod(&props))
	}
	for i, child := range p.Children {
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

func (p *PageNode) All() iter.Seq[*PageNode] {
	return func(yield func(*PageNode) bool) {
		if !walk(p, yield) {
			return
		}
	}
}
