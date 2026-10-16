// Package app is a gsx tree in which pages render regions carrying
// another page's component id.
package app

import (
	"net/http"

	"github.com/gsxhq/gsx"
	"github.com/jackielii/structpages"
)

// Root is the tree.
type Root struct {
	ACE     ace     `route:"/ace ACE"`
	Detail  detail  `route:"/detail Detail"`
	Handled handled `route:"/handled Handled"`
	Wrap    wrap    `route:"/wrap Wrap"`
}

// wrap embeds detail: detail's methods are promoted onto it.
type wrap struct{ detail }

// ace borrows detail's region; d is a plain field for the bound form.
type ace struct {
	d detail
}

type detail struct{}

// handled borrows detail's region and recognises it in Props.
type handled struct{}

func (h handled) Props(t structpages.RenderTarget) (gsx.Node, error) {
	if t.Is(detail.Content) {
		return h.Page(), nil
	}
	return h.Page(), nil
}

// widget is a shared component type, not a page.
type widget struct{}

// Mount mounts the tree.
func Mount() error {
	_, err := structpages.Mount(http.NewServeMux(), Root{}, "/", "App",
		structpages.WithTargetSelector(structpages.HTMXRenderTarget))
	return err
}
