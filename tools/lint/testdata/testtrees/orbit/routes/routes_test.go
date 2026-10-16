package routes_test

import (
	"context"
	"testing"

	"ex/orbit"
	"ex/orbit/routes"

	"github.com/jackielii/structpages"
)

// appRoot mirrors the names the app mounts Orbit under. The tree is
// built with Parse, not Mount, and is the only tree this test runs.
type appRoot struct {
	Orbit orbit.Root `route:"/orbit Orbit"`
}

func TestRefs(t *testing.T) {
	sp, err := structpages.Parse(appRoot{}, "/", "Test")
	if err != nil {
		t.Fatal(err)
	}
	ctx := sp.PageContext(context.Background())
	if _, err := structpages.URLFor(ctx, routes.Detail, "id", "1"); err != nil {
		t.Fatal(err)
	}
	if _, err := structpages.URLFor(ctx, structpages.Ref("Orbit.Data.Nope")); err == nil {
		t.Fatal("want an error for a missing page")
	}
	// An empty anonymous tree is scaffolding, not a misuse.
	if _, err := structpages.Parse(struct{}{}, "/", "Empty"); err != nil {
		t.Fatal(err)
	}
}
