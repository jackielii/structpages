// Package tapp is the templ counterpart of app: templates building
// instance ids by hand from generated ids.
package tapp

import (
	"net/http"

	"github.com/jackielii/structpages"
)

// Root is the tree.
type Root struct {
	List list `route:"/ List"`
}

type list struct{}

type row struct{ ID string }

// roleTitle is a fixed role name: the same sub-element in every render.
const roleTitle = "title"

// Mount mounts the tree.
func Mount() error {
	_, err := structpages.Mount(http.NewServeMux(), Root{}, "/", "App",
		structpages.WithTargetSelector(structpages.HTMXRenderTarget))
	return err
}
