package structpages

import (
	"context"
	"strings"
	"testing"

	"github.com/jackielii/structpages/internal/idconflicta"
)

// A singly-mounted tree for exercising bound-method-value resolution and
// chain edge cases that the cross-package and length tests don't reach.
type covLeaf struct{}

func (covLeaf) Widget() component { return testComponent{"Widget"} }

func covStandalone() component { return testComponent{"standalone"} }

type covSection struct {
	Leaf covLeaf `route:"/leaf L"`
}

type covRoot struct {
	Section covSection `route:"/section S"`
}

// TestID_BoundMethodValue covers the isBound resolution path (name-based
// page lookup) for both the global lookup and the current-page match.
func TestID_BoundMethodValue(t *testing.T) {
	pc, err := parsePageTree("/", &covRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	base := pcCtx.WithValue(context.Background(), pc)

	var inst covLeaf // a value → inst.Widget is a *bound* method value

	// Global lookup (no current page): resolves by type name "covLeaf".
	got, err := ID(base, inst.Widget)
	if err != nil {
		t.Fatalf("ID(bound, global): %v", err)
	}
	if got != "section-leaf-widget" {
		t.Errorf("bound global id = %q, want %q", got, "section-leaf-widget")
	}

	// With the current page set to the Leaf mount, the current-page match
	// path (pageNodeMatchesMethod, isBound branch) is taken.
	var leaf *PageNode
	for n := range pc.root.All() {
		if n.Name == "Leaf" {
			leaf = n
		}
	}
	if leaf == nil {
		t.Fatal("Leaf node not found")
	}
	ctx := currentPageCtx.WithValue(base, leaf)
	got, err = ID(ctx, inst.Widget)
	if err != nil {
		t.Fatalf("ID(bound, current page): %v", err)
	}
	if got != "section-leaf-widget" {
		t.Errorf("bound current-page id = %q, want %q", got, "section-leaf-widget")
	}
}

// TestID_ChainEdgeErrors covers the trailing-element validation branches in
// idForChain not reached by TestID_ChainFormErrors.
func TestID_ChainEdgeErrors(t *testing.T) {
	pc, err := parsePageTree("/", &covRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	cases := []struct {
		name      string
		input     []any
		wantInErr string
	}{
		{"empty method name string", []any{covSection{}, covLeaf{}, ""}, "empty method name"},
		{"nil trailing element", []any{covSection{}, covLeaf{}, nil}, "nil trailing"},
		{"standalone function trailing", []any{covSection{}, covStandalone}, "standalone function"},
		{"method name without any chain step", []any{"Widget"}, "no page context"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ID(ctx, tc.input)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantInErr)
			}
			if !strings.Contains(err.Error(), tc.wantInErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantInErr)
			}
		})
	}
}

// TestID_BoundMethodAmbiguous covers the isBound name-matching path when the
// same type name is mounted twice (the two cross-package Widget types from
// idConflictRoot in id_cross_package_test.go): a bound method value resolves
// by type name, matches both, and errors with both distinct ids listed.
func TestID_BoundMethodAmbiguous(t *testing.T) {
	pc, err := parsePageTree("/", &idConflictRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	var inst idconflicta.Widget // bound value; List is a bound method value
	_, err = ID(ctx, inst.List)
	if err == nil {
		t.Fatal("expected ambiguity error for bound method on a doubly-mounted type name")
	}
	for _, want := range []string{"a-widget-list", "b-widget-list"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}
}

// TestID_ChainSelfRenderOverride covers the override in idForChain: when the
// chain resolves to a leaf whose type matches the current render's page, the
// current mount wins — mirroring bare-method-expression self-render. degRoot
// (degComp mounted under Alpha and Beta) is defined in id_length_test.go.
func TestID_ChainSelfRenderOverride(t *testing.T) {
	pc, err := parsePageTree("/", &degRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	base := pcCtx.WithValue(context.Background(), pc)

	var betaItem *PageNode
	for n := range pc.root.All() {
		if n.Parent != nil && n.Parent.Name == "Beta" && n.Name == "Item" {
			betaItem = n
		}
	}
	if betaItem == nil {
		t.Fatal("Beta/Item node not found")
	}

	// Chain explicitly descends Alpha, but the current page is Beta/Item;
	// since the resolved leaf type matches, the current mount overrides.
	ctx := currentPageCtx.WithValue(base, betaItem)
	got, err := IDTarget(ctx, []any{degAlpha{}, degComp.Box})
	if err != nil {
		t.Fatalf("IDTarget: %v", err)
	}
	if got != "#beta-item-box" {
		t.Errorf("chain self-render id = %q, want %q (current page overrides chain)", got, "#beta-item-box")
	}
}
