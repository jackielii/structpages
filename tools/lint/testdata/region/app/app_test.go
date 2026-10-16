package app

import (
	"net/http"
	"testing"

	"github.com/jackielii/structpages"
)

// stubRoot mounts ace without detail, the way a section's test tree
// stubs its neighbours out. ace still borrows detail's region in the
// production tree, so this tree must not clear the region findings.
type stubRoot struct {
	ACE ace `route:"/ace ACE"`
}

func TestStub(t *testing.T) {
	if _, err := structpages.Mount(http.NewServeMux(), stubRoot{}, "/", "t",
		structpages.WithTargetSelector(structpages.HTMXRenderTarget)); err != nil {
		t.Fatal(err)
	}
}
