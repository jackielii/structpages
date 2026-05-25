package structpages

import (
	"context"
	"testing"
)

// Topology C from the design discussion: same struct type mounted
// under different parents with different field names. The framework
// derives the kebab id from the *field name* of the matched page
// node, so each mount produces a distinct id. Self-render must use
// the *current request's* page node — not whichever match comes
// first in tree iteration.

type dashboardC struct{}

func (dashboardC) Header() component { return testComponent{"Header"} }

type topologyCRoot struct {
	AdminDash dashboardC `route:"/admin Admin"`
	UserDash  dashboardC `route:"/user User"`
}

func TestID_SelfRenderUsesCurrentPage(t *testing.T) {
	pc, err := parsePageTree("/", &topologyCRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}

	// Find the two mount nodes.
	var admin, user *PageNode
	for n := range pc.root.All() {
		switch n.Name {
		case "AdminDash":
			admin = n
		case "UserDash":
			user = n
		}
	}
	if admin == nil || user == nil {
		t.Fatalf("could not find both mounts; admin=%v user=%v", admin, user)
	}

	base := pcCtx.WithValue(context.Background(), pc)

	cases := []struct {
		name        string
		currentPage *PageNode
		want        string
	}{
		{"admin render", admin, "#admin-dash-header"},
		{"user render", user, "#user-dash-header"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := currentPageCtx.WithValue(base, tc.currentPage)
			got, err := IDTarget(ctx, dashboardC.Header)
			if err != nil {
				t.Fatalf("IDTarget: %v", err)
			}
			if got != tc.want {
				t.Errorf("IDTarget = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestID_NoCurrentPage_FallsBackToGlobalLookup pins the behavior
// when current page isn't set (e.g., sp.ID(v) at boot time or
// cross-page rendering where current page is a different type).
// Should still produce a valid id, even if it's only one of the
// possible answers.
func TestID_NoCurrentPage_FallsBackToGlobalLookup(t *testing.T) {
	pc, err := parsePageTree("/", &topologyCRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	// No currentPageCtx — global lookup. Pre-fix behavior is
	// first-match (deterministic by tree order, but ambiguous).
	// Fix #2 (separate commit) will turn this into an error when
	// matches yield different ids. For now we just assert ID does
	// not panic and returns something matching one of the mounts.
	got, err := IDTarget(ctx, dashboardC.Header)
	if err != nil {
		t.Logf("IDTarget cross-mount with no current page returned error (expected once Fix #2 lands): %v", err)
		return
	}
	if got != "#admin-dash-header" && got != "#user-dash-header" {
		t.Errorf("IDTarget = %q, want one of admin-dash-header / user-dash-header", got)
	}
}
