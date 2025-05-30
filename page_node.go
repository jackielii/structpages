package srx

import (
	"path"
	"reflect"
	"strings"
)

type PageNode struct {
	Name     string
	Title    string
	Route    string
	Value    reflect.Value
	Partial  *reflect.Method
	Page     *reflect.Method
	Args     *reflect.Method
	Parent   *PageNode
	Children []*PageNode
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
	sb.WriteString(", page: " + formatMethod(p.Page))
	sb.WriteString(", partial: " + formatMethod(p.Partial))
	sb.WriteString(", args: " + formatMethod(p.Args))
	sb.WriteString("}")
	return sb.String()
}
