package structpages

import (
	"context"
	"strings"
	"testing"

	"github.com/jackielii/structpages/internal/idconflicta"
	"github.com/jackielii/structpages/internal/idconflictb"
)

// Two different page types, in two different packages, share the same
// unqualified type name ("Widget") and the same method name ("List").
// Because the generated ID is derived only from the mount field name
// (which, for an embedded field, is the unqualified type name) plus the
// method name, both pages collapse to the SAME id — even though they are
// distinct components on distinct routes.
type idConflictSectionA struct {
	idconflicta.Widget `route:"/a A"`
}

type idConflictSectionB struct {
	idconflictb.Widget `route:"/b B"`
}

type idConflictRoot struct {
	a idConflictSectionA `route:"/sa SectionA"`
	b idConflictSectionB `route:"/sb SectionB"`
}

// TestCrossPackageIDCollision verifies the path-based id scheme gives
// two distinct same-named types on distinct routes distinct ids.
func TestCrossPackageIDCollision(t *testing.T) {
	pc, err := parsePageTree("/", &idConflictRoot{})
	if err != nil {
		t.Fatalf("parsePageTree failed: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	idA, err := ID(ctx, idconflicta.Widget.List)
	if err != nil {
		t.Fatalf("ID for package-a Widget.List: %v", err)
	}
	idB, err := ID(ctx, idconflictb.Widget.List)
	if err != nil {
		t.Fatalf("ID for package-b Widget.List: %v", err)
	}

	if idA != "a-widget-list" {
		t.Errorf("package a id = %q, want %q", idA, "a-widget-list")
	}
	if idB != "b-widget-list" {
		t.Errorf("package b id = %q, want %q", idB, "b-widget-list")
	}
	if idA == idB {
		t.Fatalf("ids still collide: %q", idA)
	}
}

// TestCrossPackageRefAmbiguity verifies a qualified Ref("Widget.List")
// now reports an ambiguity error (listing both routes) instead of
// silently resolving to whichever same-named page is reached first.
func TestCrossPackageRefAmbiguity(t *testing.T) {
	pc, err := parsePageTree("/", &idConflictRoot{})
	if err != nil {
		t.Fatalf("parsePageTree failed: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	id, err := ID(ctx, Ref("Widget.List"))
	if err == nil {
		t.Fatalf("expected ambiguity error, got id=%q", id)
	}
	for _, want := range []string{"ambiguous", "/sa/a", "/sb/b"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}
}

// TestCrossPackageStandaloneFunctionID verifies that standalone function
// components are prefixed by their package name, so two same-named
// functions in different packages get distinct ids. Standalone functions
// are not mounted in the tree, so the tree here is irrelevant.
func TestCrossPackageStandaloneFunctionID(t *testing.T) {
	pc, err := parsePageTree("/", &idConflictRoot{})
	if err != nil {
		t.Fatalf("parsePageTree failed: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	idA, err := ID(ctx, idconflicta.StatsWidget)
	if err != nil {
		t.Fatalf("ID for package-a StatsWidget: %v", err)
	}
	idB, err := ID(ctx, idconflictb.StatsWidget)
	if err != nil {
		t.Fatalf("ID for package-b StatsWidget: %v", err)
	}

	if idA != "idconflicta-stats-widget" {
		t.Errorf("package a function id = %q, want %q", idA, "idconflicta-stats-widget")
	}
	if idB != "idconflictb-stats-widget" {
		t.Errorf("package b function id = %q, want %q", idB, "idconflictb-stats-widget")
	}
	if idA == idB {
		t.Fatalf("standalone function ids still collide: %q", idA)
	}

	// IDTarget adds the "#" selector prefix.
	sel, err := IDTarget(ctx, idconflicta.StatsWidget)
	if err != nil {
		t.Fatalf("IDTarget for package-a StatsWidget: %v", err)
	}
	if sel != "#idconflicta-stats-widget" {
		t.Errorf("package a function selector = %q, want %q", sel, "#idconflicta-stats-widget")
	}
}
