package orbit

import (
	"net/http"
	"testing"

	"github.com/jackielii/structpages"
)

type stub struct{}

// stubRoot mounts a stub under the production name Orbit, as test
// fixtures do for sections they don't exercise.
type stubRoot struct {
	Orbit stub `route:"/stub Orbit"`
}

// rerooted mounts the data section at the root instead of /orbit, so
// detail lives at a different route than in production.
type rerooted struct {
	Data data `route:"/data Data"`
}

func TestMounts(t *testing.T) {
	if _, err := structpages.Mount(http.NewServeMux(), stubRoot{}, "/", "t"); err != nil {
		t.Fatal(err)
	}
	if _, err := structpages.Mount(http.NewServeMux(), rerooted{}, "/", "t"); err != nil {
		t.Fatal(err)
	}
}
