// Package pages exercises RenderTarget.Is arguments; its gsx template
// uses htmx, so the trees it is part of need a target selector.
package pages

import (
	"ex/admin"
	"ex/ui"

	"github.com/jackielii/structpages"
)

// Root is the app tree.
type Root struct {
	Home  index       `route:"/{$} Home"`
	Admin admin.Panel `route:"/admin Admin"`
}

type index struct{}

func (index) Page() ui.Comp    { return ui.Comp{} }
func (index) Content() ui.Comp { return ui.Comp{} }

// concreteTarget has an Is method of its own; calls on it aren't calls
// through structpages.RenderTarget.
type concreteTarget struct{}

func (concreteTarget) Is(any) bool { return false }

// embedsTarget promotes RenderTarget.Is.
type embedsTarget struct{ structpages.RenderTarget }

// narrowTarget embeds RenderTarget in another interface.
type narrowTarget interface {
	structpages.RenderTarget
	Name() string
}

func (p index) Props(t structpages.RenderTarget, any any, e embedsTarget, n narrowTarget) (ui.Comp, error) {
	var c concreteTarget
	switch {
	case t.Is(p.Content): // bound method value of a mounted page
	case t.Is(index.Content): // method expression
	case t.Is((*index).Page): // pointer-receiver method expression
	case t.Is(admin.Panel.Stats): // another mounted page, another package
	case t.Is(ui.Toast): // standalone function
	case t.Is(any): // runtime value: not checked
	case c.Is(p.Content()): // concrete Is: not RenderTarget's
	case t.Is(p.Content()): // want: calls the component
	case e.Is(index{}.Content()): // want: through an embedded RenderTarget
	case n.Is(p.Page()): // want: through an embedding interface
	case t.Is(ui.Widget.Body): // want: unmounted method expression
	case t.Is(ui.Widget{}.Body): // want: unmounted method value
	case t.Is(structpages.Ref("index.Content")): // want: not a function
	case t.Is("content"): // want: not a function
	//structpages:lint:ignore idfor
	case t.Is(ui.Widget.Body): // suppressed
	}
	return ui.Comp{}, nil
}
