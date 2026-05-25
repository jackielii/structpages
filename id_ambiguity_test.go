package structpages

import (
	"context"
	"strings"
	"testing"
)

// Cross-page method expression: when the receiver type is mounted
// multiple times under different field names, the ids produced by
// the kebab(Name) + "-" + kebab(method) rule differ. Today's
// first-match behavior silently picks one — a latent bug. The fix
// is to collapse same-id matches (the entryPage / topology B case
// — three mounts with identical field name still produce one id)
// and error on truly divergent matches with a disambiguation hint.

type ambDashboard struct{}

func (ambDashboard) Header() component { return testComponent{"Header"} }

type ambSameName struct{}

func (ambSameName) Overlays() component { return testComponent{"Overlays"} }

// Topology B: same field name across all parents → same kebab id.
type ambBRoot struct {
	Foundations struct {
		EntryDetail ambSameName `route:"/{slug} Foundation"`
	} `route:"/foundations Foundations"`
	Components struct {
		EntryDetail ambSameName `route:"/{slug} Component"`
	} `route:"/components Components"`
}

func TestID_CrossPageSameKebab_Accepts(t *testing.T) {
	pc, err := parsePageTree("/", &ambBRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	got, err := IDTarget(ctx, ambSameName.Overlays)
	if err != nil {
		t.Fatalf("IDTarget: %v (expected silent success; same field name everywhere → same id)", err)
	}
	if got != "#entry-detail-overlays" {
		t.Errorf("IDTarget = %q, want %q", got, "#entry-detail-overlays")
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
