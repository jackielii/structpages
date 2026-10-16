// Package orbittest mounts whatever tree a test hands it. Its page
// argument is an interface, which the linter can't follow.
package orbittest

import (
	"net/http"

	"github.com/jackielii/structpages"
)

func Mount(pages any) (*http.ServeMux, error) {
	mux := http.NewServeMux()
	_, err := structpages.Mount(mux, pages, "/", "Test")
	return mux, err
}
