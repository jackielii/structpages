package structpages

import (
	"context"
	"strings"
	"testing"
)

// Cross-page method expression: when the receiver type is mounted
// multiple times under different parents, the path-based id scheme
// gives each mount a DISTINCT id (e.g. "foundations-entry-detail-…"
// vs "components-entry-detail-…"). A bare method expression doesn't
// say which mount you mean, so it is genuinely ambiguous and errors
// with the available mounts listed. Disambiguate via the []any chain
// form, a Ref, or the self-render path (currentPage context).

type ambDashboard struct{}

func (ambDashboard) Header() component { return testComponent{"Header"} }

type ambSameName struct{}

func (ambSameName) Overlays() component { return testComponent{"Overlays"} }

// Same field name across both parents — but distinct paths now yield
// distinct ids, so a bare cross-page expression is ambiguous.
type ambBRoot struct {
	Foundations struct {
		EntryDetail ambSameName `route:"/{slug} Foundation"`
	} `route:"/foundations Foundations"`
	Components struct {
		EntryDetail ambSameName `route:"/{slug} Component"`
	} `route:"/components Components"`
}

func TestID_CrossPageSameFieldName_Errors(t *testing.T) {
	pc, err := parsePageTree("/", &ambBRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	_, err = IDTarget(ctx, ambSameName.Overlays)
	if err == nil {
		t.Fatal("expected ambiguity error: same field name now yields distinct path-based ids")
	}
	for _, want := range []string{
		"multiple fields",
		"foundations-entry-detail-overlays",
		"components-entry-detail-overlays",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}
}

// Topology C / D: different field names → different kebab ids.
// Cross-page (no current page set) should error with the available
// mounts listed.
func TestID_CrossPageDifferentKebab_Errors(t *testing.T) {
	pc, err := parsePageTree("/", &topologyCRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	_, err = IDTarget(ctx, dashboardC.Header)
	if err == nil {
		t.Fatal("expected error for ambiguous cross-page method expression, got nil")
	}
	for _, want := range []string{"multiple fields", "admin-dash-header", "user-dash-header"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}
}
