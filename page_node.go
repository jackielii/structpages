package srx

import (
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
	sb.WriteString("name: " + p.Name)
	sb.WriteString(", title: " + p.Title)
	sb.WriteString(", route: " + p.Route)
	sb.WriteString(", middlewares: " + formatMethod(p.Middlewares))
	for name, comp := range p.Components {
		sb.WriteString(", component: " + name + " -> " + formatMethod(comp))
	}
	sb.WriteString(", args: " + formatMethod(p.Args))
	sb.WriteString("}")
	return sb.String()
}
