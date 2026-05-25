package lint

import (
	"testing"
)

func TestDedupByFullRoute(t *testing.T) {
	mk := func(route string) *PageNode {
		return &PageNode{FullRoute: route}
	}

	cases := []struct {
		name string
		in   []*PageNode
		want []string // expected FullRoutes in order
	}{
		{
			name: "empty",
			in:   nil,
			want: nil,
		},
		{
			name: "single",
			in:   []*PageNode{mk("/items")},
			want: []string{"/items"},
		},
		{
			name: "two same — collapses",
			in:   []*PageNode{mk("/items/{$}"), mk("/items/{$}")},
			want: []string{"/items/{$}"},
		},
		{
			name: "two different — keep both",
			in:   []*PageNode{mk("/items/{$}"), mk("/v2/items/{$}")},
			want: []string{"/items/{$}", "/v2/items/{$}"},
		},
		{
			name: "three with mixed dupes — first occurrence wins",
			in: []*PageNode{
				mk("/a"), mk("/b"), mk("/a"), mk("/c"), mk("/b"),
			},
			want: []string{"/a", "/b", "/c"},
		},
		{
			name: "all duplicates — collapses to one",
			in:   []*PageNode{mk("/x"), mk("/x"), mk("/x"), mk("/x")},
			want: []string{"/x"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dedupByFullRoute(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d (got: %v)", len(got), len(tc.want), routes(got))
			}
			for i, r := range tc.want {
				if got[i].FullRoute != r {
					t.Errorf("got[%d] = %q, want %q", i, got[i].FullRoute, r)
				}
			}
		})
	}
}

// TestDedupByFullRoute_PreservesAmbiguityAcrossRoutes is the
// regression guard for the fix: distinct FullRoutes must NOT
// collapse, otherwise real production ambiguity (same page type
// mounted at two different paths) silently disappears.
func TestDedupByFullRoute_PreservesAmbiguityAcrossRoutes(t *testing.T) {
	in := []*PageNode{
		{FullRoute: "/admin/practitioners/{$}"},
		{FullRoute: "/staff/practitioners/{$}"},
	}
	got := dedupByFullRoute(in)
	if len(got) != 2 {
		t.Fatalf("want 2 matches preserved, got %d: %v", len(got), routes(got))
	}
}

func routes(ns []*PageNode) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = n.FullRoute
	}
	return out
}
