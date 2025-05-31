package structpages

import (
	"fmt"
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
	Args        *reflect.Method
	Components  map[string]*reflect.Method
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
	if len(p.Components) == 0 {
		sb.WriteString("\n  components: []")
	}
	if p.Value.Type().AssignableTo(handlerType) {
		sb.WriteString("\n  is http.Handler: true")
	}
	for name, comp := range p.Components {
		sb.WriteString("\n  component: " + name + " -> " + formatMethod(comp))
	}
	sb.WriteString("\n  args: " + formatMethod(p.Args))
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
