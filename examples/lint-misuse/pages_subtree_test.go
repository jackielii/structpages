package main

import (
	"net/http"
	"testing"

	"github.com/jackielii/structpages"
)

// TestSubTreeMountsCleanly is a structural test that re-mounts a
// sub-tree of the production root standalone (the same pattern used
// in real-world admin/handler_test.go style suites). It mirrors a
// case the lint analyzer used to mishandle: the analyzer would see
// itemsRoot mounted twice (once as a child of root in pages.go's
// main, once at the same FullRoute in this test) and incorrectly
// report ambiguity for any URLFor call to itemsIndex / itemDetail.
//
// Keeping this in the lint-misuse fixture pins the regression: the
// snapshot in lint_test.go's want literal does NOT include an
// "ambiguous" diagnostic for itemsIndex even though pages.go calls
// URLFor(ctx, itemsIndex{}) at the bare-type lookup path.
func TestSubTreeMountsCleanly(t *testing.T) {
	mux := http.NewServeMux()
	if _, err := structpages.Mount(mux, &itemsRoot{}, "/items", "items only"); err != nil {
		t.Fatal(err)
	}
}
