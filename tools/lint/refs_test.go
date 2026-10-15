package lint

import (
	"go/token"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
)

// buildNavTree returns a tree shaped like his-project's:
//
//	Root(/) → Wrapper(/) → Section(/section) → Leaf(/{$})
//	                     → Other(/other)
//
// "Section" is a grandchild, not a top-level node — mirroring Receptionist
// nested under the authed subtree.
func buildNavTree() *PageTree {
	leaf := &PageNode{Name: "Leaf", FullRoute: "/section/{$}"}
	section := &PageNode{Name: "Section", FullRoute: "/section", Children: []*PageNode{leaf}}
	leaf.Parent = section
	other := &PageNode{Name: "Other", FullRoute: "/other"}
	wrapper := &PageNode{Name: "Wrapper", FullRoute: "/", Children: []*PageNode{section, other}}
	section.Parent, other.Parent = wrapper, wrapper
	root := &PageNode{Name: "Root", FullRoute: "/", Children: []*PageNode{wrapper}}
	wrapper.Parent = root
	return &PageTree{Roots: []*PageNode{root}}
}

// TestResolveRefByQualified_nestedAnchor pins that the lint resolves a
// qualified Ref anchored on a uniquely-named non-top-level node — matching
// the runtime resolver (structpages parse.go), so a lint-passing Ref is
// runtime-valid.
func TestResolveRefByQualified_nestedAnchor(t *testing.T) {
	ctx := &checkCtx{tree: buildNavTree(), silent: true}

	got := resolveRefByQualified(ctx, token.NoPos, "Section.Leaf", "Section.Leaf", false)
	if got == nil {
		t.Fatal("Section.Leaf did not resolve; nested anchor should be found")
	}
	if got.Name != "Leaf" {
		t.Errorf("resolved to %q, want %q", got.Name, "Leaf")
	}

	// A bad descent segment must fail (the Receptionist.Nope case).
	if n := resolveRefByQualified(ctx, token.NoPos, "Section.Nope", "Section.Nope", false); n != nil {
		t.Errorf("Section.Nope resolved to %q, want nil", n.Name)
	}
}

// TestResolveRefByQualified_ambiguousAnchor pins that a non-top-level anchor
// matching more than one node is rejected, not silently picked.
func TestResolveRefByQualified_ambiguousAnchor(t *testing.T) {
	// Two wrappers, each with a "Section" → ambiguous anchor.
	mkWrapper := func(routePrefix string) *PageNode {
		leaf := &PageNode{Name: "Leaf", FullRoute: routePrefix + "/section/{$}"}
		section := &PageNode{Name: "Section", FullRoute: routePrefix + "/section", Children: []*PageNode{leaf}}
		leaf.Parent = section
		w := &PageNode{Name: "Wrapper", FullRoute: routePrefix, Children: []*PageNode{section}}
		section.Parent = w
		return w
	}
	a, b := mkWrapper("/a"), mkWrapper("/b")
	root := &PageNode{Name: "Root", FullRoute: "/", Children: []*PageNode{a, b}}
	a.Parent, b.Parent = root, root
	ctx := &checkCtx{tree: &PageTree{Roots: []*PageNode{root}}, silent: true}

	if n := resolveRefByQualified(ctx, token.NoPos, "Section.Leaf", "Section.Leaf", false); n != nil {
		t.Errorf("ambiguous 'Section' anchor resolved to %q, want nil", n.Name)
	}
}

// TestResolveRefByQualified_topLevelStillWins pins backward compatibility:
// a top-level anchor resolves exactly as before.
func TestResolveRefByQualified_topLevelStillWins(t *testing.T) {
	ctx := &checkCtx{tree: buildNavTree(), silent: true}
	if n := resolveRefByQualified(ctx, token.NoPos, "Wrapper.Section", "Wrapper.Section", false); n == nil || n.Name != "Section" {
		t.Errorf("Wrapper.Section: got %v, want Section", n)
	}
}

// TestResolveRefAtCallSite_oneDiagnosticPerRef pins that a Ref(...) passed
// straight to URLFor or ID/IDTarget yields exactly one diagnostic between
// checkRefConversion (which reports refs invalid in every context) and the
// call-site check (which reports only context-specific failures).
func TestResolveRefAtCallSite_oneDiagnosticPerRef(t *testing.T) {
	cases := []struct {
		ref        string
		idContext  bool
		wantSubstr string // "" = no diagnostic
	}{
		// Unknown anchor: invalid everywhere, the conversion check's
		// ID-context wording (with the available anchors) wins.
		{"Nope.X", false, `anchor "Nope" not found anywhere in the tree; available:`},
		{"Nope.X", true, `anchor "Nope" not found anywhere in the tree; available:`},
		// Simple name: ID context treats it as a method name, so the
		// conversion check stays silent and the URL call site reports.
		{"Ghost", false, `no page with this name`},
		// Valid in ID context only (Leaf.Method), so URLFor reports.
		{"Leaf.Method", false, `segment 1 ("Method") not found as child of "Leaf"`},
		{"Leaf.Method", true, ""},
		{"Section.Leaf", false, ""},
	}
	for _, tc := range cases {
		var got []string
		ctx := &checkCtx{
			tree: buildNavTree(),
			pass: &analysis.Pass{Report: func(d analysis.Diagnostic) { got = append(got, d.Message) }},
		}
		if refConversionReports(ctx, token.NoPos, tc.ref) {
			resolveRef(ctx, token.NoPos, tc.ref, true) // what checkRefConversion emits
		}
		resolveRefAtCallSite(ctx, token.NoPos, tc.ref, tc.idContext)

		switch {
		case tc.wantSubstr == "" && len(got) != 0:
			t.Errorf("Ref(%q) id=%v: want no diagnostic, got %q", tc.ref, tc.idContext, got)
		case tc.wantSubstr != "" && (len(got) != 1 || !strings.Contains(got[0], tc.wantSubstr)):
			t.Errorf("Ref(%q) id=%v: want one diagnostic containing %q, got %q",
				tc.ref, tc.idContext, tc.wantSubstr, got)
		}
	}
}
