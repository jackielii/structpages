// Package app is a gsx tree whose templates build instance ids by
// hand from generated ids, and with Key / KeyTarget.
package app

import (
	"context"
	"net/http"

	"github.com/jackielii/structpages"
)

// Root is the tree.
type Root struct {
	List list `route:"/ List"`
}

type list struct{}

type row struct {
	ID  string
	N   int
	Sel string
}

// roleTitle is a fixed role name: the same sub-element in every render.
const roleTitle = "title"

// rowID builds an instance id outside any template: no attribute to
// check, so it isn't reported.
func rowID(ctx context.Context, r row) (string, error) {
	id, err := structpages.ID(ctx, list.Row)
	if err != nil {
		return "", err
	}
	return id + "-" + r.ID, nil
}

// Mount mounts the tree.
func Mount() error {
	_, err := structpages.Mount(http.NewServeMux(), Root{}, "/", "App",
		structpages.WithTargetSelector(structpages.HTMXRenderTarget))
	return err
}
