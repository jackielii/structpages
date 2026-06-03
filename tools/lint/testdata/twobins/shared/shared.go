// Package shared holds a page tree mounted by two separate binaries:
// a standalone preview binary and the main app. A bare URLFor against
// a page in this package must not be flagged ambiguous (issue #22).
package shared

import (
	"context"

	"github.com/jackielii/structpages"
)

// Root is the gallery root, mounted at "/" standalone and nested under
// "/design-system" inside the app.
type Root struct {
	Home homePage `route:"/{$} Home"`
}

type homePage struct{}

// LinkHome builds a link to the home page. The bare URLFor(homePage{})
// resolves to exactly one route in each binary, so the linter must
// stay silent here.
func LinkHome(ctx context.Context) (string, error) {
	return structpages.URLFor(ctx, homePage{})
}
