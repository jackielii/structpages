package structpages

import (
	"net/http"
	"reflect"
	"slices"
	"testing"
)

// The lookup indexes stand in for tree scans and id building on every
// call, so these tests check each against the uncached computation over
// whole trees.

// TestComponentIDCache_matchesBuild pins that cached component ids equal
// freshly built ones, raw and as selectors, with the default and a
// configured id length.
func TestComponentIDCache_matchesBuild(t *testing.T) {
	for _, maxLen := range []int{0, 8} {
		var opts []Option
		if maxLen > 0 {
			opts = append(opts, WithMaxIDLength(maxLen))
		}
		sp, err := Mount(http.NewServeMux(), &degRoot{}, "/", "App", opts...)
		if err != nil {
			t.Fatal(err)
		}
		pc := sp.pc
		n := 0
		for pn := range pc.root.All() {
			for name := range pn.Components {
				n++
				for _, raw := range []bool{true, false} {
					if got, want := pc.componentID(pn, name, raw), pc.buildComponentID(pn, name, raw); got != want {
						t.Errorf("maxLen %d: componentID(%s, %s, %v) = %q, want %q",
							maxLen, pn.FullRoute(), name, raw, got, want)
					}
				}
			}
		}
		if n == 0 {
			t.Fatal("tree has no components")
		}
	}
}

// TestComponentIDCache_staleLength pins that a cache built for one id
// length is not used after maxIDLen changes, as on a hand-built
// parseContext.
func TestComponentIDCache_staleLength(t *testing.T) {
	pc, err := parsePageTree("/", &degRoot{})
	if err != nil {
		t.Fatal(err)
	}
	pc.buildIndexes()
	pc.maxIDLen = 8
	for pn := range pc.root.All() {
		for name := range pn.Components {
			if got, want := pc.componentID(pn, name, true), pc.buildComponentID(pn, name, true); got != want {
				t.Errorf("componentID(%s, %s) = %q, want %q", pn.FullRoute(), name, got, want)
			}
		}
	}
}

// TestMethodIndex_matchesTreeScan pins that method-based page lookups
// find the nodes a scan of the tree does: the receiver's mounts for
// method expressions and method values, the nodes having a method for a
// Ref, and method presence on pointer types.
func TestMethodIndex_matchesTreeScan(t *testing.T) {
	sp, err := Mount(http.NewServeMux(), lookupRoot{}, "/", "Root")
	if err != nil {
		t.Fatal(err)
	}
	pc := sp.pc

	scan := func(keep func(*PageNode) bool) []*PageNode {
		var out []*PageNode
		for node := range pc.root.All() {
			if keep(node) {
				out = append(out, node)
			}
		}
		return out
	}

	exprs := []any{
		lkRegion.Rows, lkRegion{}.Rows,
		lkTwice.Rows, lkTwice{}.Rows,
		lookupLeaf[lkS3, lkL2].Page, lookupLeaf[lkS3, lkL2]{}.Page,
	}
	for _, expr := range exprs {
		info, err := extractMethodInfo(expr)
		if err != nil {
			t.Fatal(err)
		}
		want := scan(func(n *PageNode) bool {
			if !info.receiverIs(n.Value.Type()) {
				return false
			}
			if info.isBound {
				_, found := n.Value.Type().MethodByName(info.methodName)
				return found
			}
			return true
		})
		if len(want) == 0 {
			t.Fatalf("%s: scan found no nodes", info.receiverString())
		}
		// Twice, so the second call reads the bound-method memo.
		for range 2 {
			if got := pc.collectPageNodesForMethod(info); !slices.Equal(got, want) {
				t.Errorf("%s.%s (bound %v): got %d nodes, want %d",
					info.receiverString(), info.methodName, info.isBound, len(got), len(want))
			}
		}
	}

	for _, name := range []string{"Page", "Rows", "Nope"} {
		want := scan(func(n *PageNode) bool {
			_, found := n.Value.Type().MethodByName(name)
			return found
		})
		if got := findPagesWithMethod(pc, name); !slices.Equal(got, want) {
			t.Errorf("findPagesWithMethod(%q): got %d nodes, want %d", name, len(got), len(want))
		}
	}

	for node := range pc.root.All() {
		pt := pointerType(node.Value.Type())
		for _, name := range []string{"Page", "Rows", "Nope"} {
			_, want := pt.MethodByName(name)
			if got := pc.hasMethod(pt, name); got != want {
				t.Errorf("hasMethod(%s, %q) = %v, want %v", pt, name, got, want)
			}
		}
	}
	// A type outside the tree falls back to reflection.
	outside := reflect.TypeFor[*testComponent]()
	if !pc.hasMethod(outside, "Render") || pc.hasMethod(outside, "Nope") {
		t.Error("hasMethod disagrees with reflection for a type outside the index")
	}
}

// TestEscapeKey_unescapedKeyIsReturnedAsIs pins the fast path: a key of
// only id-safe bytes escapes to itself.
func TestEscapeKey_unescapedKeyIsReturnedAsIs(t *testing.T) {
	for _, key := range []string{"42", "row-7", "AbC-09"} {
		if got := escapeKey(key); got != key {
			t.Errorf("escapeKey(%q) = %q", key, got)
		}
	}
	for _, key := range []string{"a_b", "a b", "é", "a.b"} {
		if got := escapeKey(key); got == key {
			t.Errorf("escapeKey(%q) left it unescaped", key)
		}
	}
}
