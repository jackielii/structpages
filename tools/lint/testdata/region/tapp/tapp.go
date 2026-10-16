// Package tapp is the templ counterpart of app: pages rendering
// regions carrying another page's component id.
package tapp

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/jackielii/structpages"
)

// Root is the tree.
type Root struct {
	ACE     ace     `route:"/ace ACE"`
	Detail  detail  `route:"/detail Detail"`
	Handled handled `route:"/handled Handled"`
	Stale   stale   `route:"/stale Stale"`
}

// ace borrows detail's region; d is a plain field for the bound form.
type ace struct {
	d detail
}

type detail struct{}

// handled borrows detail's region and recognises it in Props.
type handled struct{}

func (h handled) Props(t structpages.RenderTarget) (templ.Component, error) {
	if t.Is(detail.Content) {
		return h.Page(), nil
	}
	return h.Page(), nil
}

// stale borrows detail's region, but its _templ.go isn't what this
// templ version generates, so its attributes can't be known.
type stale struct{}

// widget is a shared component type, not a page.
type widget struct{}

// Mount mounts the tree.
func Mount() error {
	_, err := structpages.Mount(http.NewServeMux(), Root{}, "/", "App",
		structpages.WithTargetSelector(structpages.HTMXRenderTarget))
	return err
}
