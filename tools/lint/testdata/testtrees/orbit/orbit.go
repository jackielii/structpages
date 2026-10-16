// Package orbit is a module section mounted under /orbit by the app.
// Its call sites must resolve against the production tree even though
// tests mount stubbed or re-rooted trees with the same field names.
package orbit

import (
	"context"

	"ex/orbit/routes"

	"github.com/jackielii/structpages"
)

type Root struct {
	Home home    `route:"GET /{$} Home"`
	Data DataAPI `route:"/data Data"`
}

// DataAPI is an alias: the runtime sees straight through it, so the
// linter must too.
type DataAPI = data

type data struct {
	Detail detail `route:"GET /{id} Detail"`
}

type home struct{}

type detail struct{}

// Links exercises every way the section links to itself.
func Links(ctx context.Context) error {
	// Bare type: one route per tree, though the trees disagree on it.
	if _, err := structpages.URLFor(ctx, detail{}, "id", "1"); err != nil {
		return err
	}
	// Qualified Ref through the production anchor; a test tree stubs
	// Orbit out.
	if _, err := structpages.URLFor(ctx, structpages.Ref("Orbit.Home")); err != nil {
		return err
	}
	// Ref constant as a chain step, followed by a fragment.
	if _, err := structpages.URLFor(ctx, []any{routes.Detail, "?q={q}"}, "id", "1", "q", "x"); err != nil {
		return err
	}
	// The one real typo: wrong in every tree.
	_, err := structpages.URLFor(ctx, structpages.Ref("Orbit.Nope"))
	return err
}
