package lint

import (
	"go/token"
	"testing"
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
