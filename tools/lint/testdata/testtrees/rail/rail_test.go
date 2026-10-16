package rail_test

import (
	"context"
	"net/http"
	"testing"

	"ex/orbit/orbittest"
	"ex/rail"

	"github.com/jackielii/structpages"
)

type page struct{}

type groupPages struct {
	Index page `route:"GET /{$} Group"`
	Item  page `route:"GET /items/{id} Item"`
}

// railTree is mounted only through serveTree → serveShell → Mount.
type railTree struct {
	Group groupPages `route:"/g Group"`
	Other page       `route:"GET /other Other"`
}

// refuseTree is mounted directly, so it is no longer the only tree the
// package's data refs are checked against.
type refuseTree struct {
	Refuse page `route:"/refuse Refuse"`
}

type genericTree struct {
	Gen page `route:"/gen Gen"`
}

type routedTree struct {
	Routed page `route:"/r Routed"`
}

type crossTree struct {
	Cross page `route:"/cross Cross"`
}

func serveShell(t *testing.T, pages any) {
	if _, err := structpages.Mount(http.NewServeMux(), pages, "/", "t"); err != nil {
		t.Fatal(err)
	}
}

func serveTree(t *testing.T, pages any) { serveShell(t, pages) }

func parseGeneric[T any](t *testing.T, pages T) {
	if _, err := structpages.Parse(pages, "/", "t"); err != nil {
		t.Fatal(err)
	}
}

func mountAt(t *testing.T, pages any, route string) {
	if _, err := structpages.Mount(http.NewServeMux(), pages, route, "t"); err != nil {
		t.Fatal(err)
	}
}

var items = []rail.Item{
	{Ref: "Other"},
	{Ref: "Group.Index"},
	{Ref: "Gen"},
	{Ref: "/at/r"},
	{Ref: "Cross"},
	{Ref: "Plain.Home"},  // no tree has Plain
	{Ref: "Group.Itemm"}, // a typo in railTree, reported from railTree
}

func TestRail(t *testing.T) {
	if _, err := structpages.Mount(http.NewServeMux(), refuseTree{}, "/", "t"); err != nil {
		t.Fatal(err)
	}
	serveTree(t, railTree{})
	parseGeneric(t, genericTree{})
	mountAt(t, routedTree{}, "/at")
	if _, err := orbittest.Mount(crossTree{}); err != nil {
		t.Fatal(err)
	}
	// A direct call is checked against the helper-mounted tree too.
	if _, err := structpages.URLFor(context.Background(), []any{railTree{}, groupPages{}}); err == nil {
		t.Log(items)
	}
}
