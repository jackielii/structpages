package structpages

import (
	"context"
	"strings"
	"testing"
)

// The "section roots" topology: one page type reused under several
// section roots, each via a field of the SAME name. This is the
// design-system gallery shape — a single entry-detail page mounted as
// EntryDetail under Foundations / Components / Patterns. Path-based ids
// give each mount a distinct id, so:
//
//   - a method expression self-renders to the CURRENT request's mount
//     (each section gets its own id), and
//   - a bare Ref by name is genuinely ambiguous and errors — there is no
//     single answer without saying which section you mean.
//
// This is the canonical example of the rule: structural ids are for
// disambiguating targets within a render; a value that must be the same
// across N mounts (e.g. a singleton slot referenced from another package)
// should not be derived from a multiply-mounted page in the first place.

type sectionEntry struct{}

func (sectionEntry) Overlays() component { return testComponent{"Overlays"} }

type foundationsSection struct {
	EntryDetail sectionEntry `route:"/{slug} Foundation"`
}
type componentsSection struct {
	EntryDetail sectionEntry `route:"/{slug} Component"`
}
type patternsSection struct {
	EntryDetail sectionEntry `route:"/{slug} Pattern"`
}
type sectionRoots struct {
	Foundations foundationsSection `route:"/foundations Foundations"`
	Components  componentsSection  `route:"/components Components"`
	Patterns    patternsSection    `route:"/patterns Patterns"`
}

// TestID_SectionRoots_MethodExprSelfRenders pins that a method expression
// on a type mounted under three section roots resolves to the current
// request's mount — each section yielding its own distinct id.
func TestID_SectionRoots_MethodExprSelfRenders(t *testing.T) {
	pc, err := parsePageTree("/", &sectionRoots{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}

	bySection := map[string]*PageNode{}
	for n := range pc.root.All() {
		if n.Name != "EntryDetail" {
			continue
		}
		// Parent field name identifies the section.
		bySection[n.Parent.Name] = n
	}
	for _, want := range []string{"Foundations", "Components", "Patterns"} {
		if bySection[want] == nil {
			t.Fatalf("could not find EntryDetail under %s", want)
		}
	}

	base := pcCtx.WithValue(context.Background(), pc)
	cases := []struct {
		section string
		want    string
	}{
		{"Foundations", "#foundations-entry-detail-overlays"},
		{"Components", "#components-entry-detail-overlays"},
		{"Patterns", "#patterns-entry-detail-overlays"},
	}
	for _, tc := range cases {
		t.Run(tc.section, func(t *testing.T) {
			ctx := currentPageCtx.WithValue(base, bySection[tc.section])
			got, err := IDTarget(ctx, sectionEntry.Overlays)
			if err != nil {
				t.Fatalf("IDTarget: %v", err)
			}
			if got != tc.want {
				t.Errorf("IDTarget = %q, want %q", got, tc.want)
			}
		})
	}

	// Each section must produce a DISTINCT id — that is the whole point of
	// path-based ids, and the reason a cross-mount Ref cannot pick one.
	seen := map[string]bool{}
	for _, tc := range cases {
		if seen[tc.want] {
			t.Fatalf("section ids are not distinct: %q repeats", tc.want)
		}
		seen[tc.want] = true
	}
}

// TestID_SectionRoots_BareRefIsAmbiguous pins that a bare qualified Ref to
// the multiply-mounted name errors and names every conflicting route — so
// nobody can lean on the (pre-path-based) coincidence that the mounts
// shared an id. A consumer that needs a stable cross-mount handle must use
// a fixed identity of its own, not a Ref into a section page.
func TestID_SectionRoots_BareRefIsAmbiguous(t *testing.T) {
	pc, err := parsePageTree("/", &sectionRoots{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	id, err := ID(ctx, Ref("EntryDetail.Overlays"))
	if err == nil {
		t.Fatalf("expected ambiguity error for a 3-way mounted Ref, got id=%q", id)
	}
	for _, want := range []string{"ambiguous", "/foundations/{slug}", "/components/{slug}", "/patterns/{slug}"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}
}
