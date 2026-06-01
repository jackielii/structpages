package structpages

import (
	"context"
	"strings"
	"testing"
)

// Chain + Ref-qualified tests. Both replace the removed Child API.
// Fixture is the his-project-shaped tree from url_for_test.go
// (ambiguousRoot + ambiguous*Root) — one shared leaf type mounted
// under three sibling parents.

func TestChain_resolvesViaTypedSlice(t *testing.T) {
	pc, err := parsePageTree("/", &ambiguousRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	cases := []struct {
		name   string
		page   any
		params map[string]any
		expect string
	}{
		{
			// A reference that stops at a subtree container resolves to
			// the container's index child, so the URL carries the
			// canonical trailing slash instead of the bare path that
			// would 307-redirect.
			name:   "single typed parent in slice",
			page:   []any{ambiguousComponentsRoot{}},
			expect: "/components/",
		},
		{
			name:   "chain parent -> Index (shared type, parent disambiguates)",
			page:   []any{ambiguousFoundationsRoot{}, sharedIndex{}},
			expect: "/foundations/",
		},
		{
			name:   "chain parent -> Detail",
			page:   []any{ambiguousComponentsRoot{}, sharedDetail{}},
			params: map[string]any{"slug": "button"},
			expect: "/components/button",
		},
		{
			name:   "chain to a different group",
			page:   []any{ambiguousPatternsRoot{}, sharedDetail{}},
			params: map[string]any{"slug": "form-layout"},
			expect: "/patterns/form-layout",
		},
		{
			name:   "chain + query fragment",
			page:   []any{ambiguousComponentsRoot{}, sharedDetail{}, "?tab={tab}"},
			params: map[string]any{"slug": "button", "tab": "props"},
			expect: "/components/button?tab=props",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			var err error
			if tc.params != nil {
				got, err = URLFor(ctx, tc.page, tc.params)
			} else {
				got, err = URLFor(ctx, tc.page)
			}
			if err != nil {
				t.Fatalf("URLFor: %v", err)
			}
			if got != tc.expect {
				t.Errorf("got %q, want %q", got, tc.expect)
			}
		})
	}
}

func TestChain_errors(t *testing.T) {
	pc, err := parsePageTree("/", &ambiguousRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	type unknownChild struct{}

	t.Run("descend by unknown type", func(t *testing.T) {
		_, err := URLFor(ctx, []any{ambiguousComponentsRoot{}, unknownChild{}})
		if err == nil {
			t.Fatal("expected error for unknown child type")
		}
		msg := err.Error()
		if !strings.Contains(msg, "has no child of type") {
			t.Errorf("error %q missing 'has no child of type' hint", msg)
		}
		// Must list the available children to be useful.
		for _, want := range []string{"Index", "Detail"} {
			if !strings.Contains(msg, want) {
				t.Errorf("error %q does not list available child %q", msg, want)
			}
		}
	})

	t.Run("typed value after string fragment is rejected", func(t *testing.T) {
		_, err := URLFor(ctx, []any{ambiguousComponentsRoot{}, "?q={q}", sharedDetail{}})
		if err == nil {
			t.Fatal("expected error for typed value after string fragment")
		}
		if !strings.Contains(err.Error(), "follows a string fragment") {
			t.Errorf("error %q missing 'follows a string fragment' hint", err.Error())
		}
	})

	t.Run("ambiguous descend with multiple same-type children", func(t *testing.T) {
		// Construct a tree where a parent has two children of the same type.
		type dupParent struct {
			A sharedDetail `route:"/a/{slug}"`
			B sharedDetail `route:"/b/{slug}"`
		}
		type rootDup struct {
			P dupParent `route:"/p P"`
		}
		dupPC, err := parsePageTree("/", &rootDup{})
		if err != nil {
			t.Fatalf("parsePageTree: %v", err)
		}
		dupCtx := pcCtx.WithValue(context.Background(), dupPC)
		_, err = URLFor(dupCtx, []any{dupParent{}, sharedDetail{}})
		if err == nil {
			t.Fatal("expected error for ambiguous descend")
		}
		msg := err.Error()
		if !strings.Contains(msg, "multiple children of type") {
			t.Errorf("error %q missing 'multiple children of type' hint", msg)
		}
		// Must mention the field names and Ref hint.
		for _, want := range []string{"A", "B", "Ref"} {
			if !strings.Contains(msg, want) {
				t.Errorf("error %q does not list %q", msg, want)
			}
		}
	})

	t.Run("Ref in non-first chain position is rejected", func(t *testing.T) {
		_, err := URLFor(ctx, []any{ambiguousComponentsRoot{}, Ref("Detail")})
		if err == nil {
			t.Fatal("expected error for Ref as chain step")
		}
		if !strings.Contains(err.Error(), "Ref is only valid as the first chain step") {
			t.Errorf("error %q missing 'first chain step' hint", err.Error())
		}
	})

}

// TestChain_nilGuards pins that nil inputs at any position never
// panic — they were a real foot-gun pre-fix because pointerType(reflect
// .TypeOf(nil)) segfaults. Split from TestChain_errors to keep gocyclo
// happy.
func TestChain_nilGuards(t *testing.T) {
	pc, err := parsePageTree("/", &ambiguousRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	cases := []struct {
		name     string
		page     any
		wantHint string
	}{
		{
			name:     "bare nil page argument",
			page:     nil,
			wantHint: "nil",
		},
		{
			name:     "nil first chain element",
			page:     []any{nil, sharedDetail{}},
			wantHint: "nil",
		},
		{
			name:     "nil descent chain element",
			page:     []any{ambiguousComponentsRoot{}, nil},
			wantHint: "chain step 1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := URLFor(ctx, tc.page)
			if err == nil {
				t.Fatalf("expected error, not panic")
			}
			if !strings.Contains(err.Error(), tc.wantHint) {
				t.Errorf("error %q missing hint %q", err.Error(), tc.wantHint)
			}
		})
	}
}

func TestRef_qualifiedPath(t *testing.T) {
	pc, err := parsePageTree("/", &ambiguousRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	cases := []struct {
		name   string
		ref    string
		params map[string]any
		expect string
	}{
		{
			// By-Name Ref resolving to a subtree container yields the
			// container's index URL (canonical trailing slash), matching
			// the typed-value and qualified-Ref forms.
			name:   "single-segment Ref by Name resolves to container index",
			ref:    "Foundations",
			expect: "/foundations/",
		},
		{
			name:   "qualified Ref descends by Name",
			ref:    "Components.Index",
			expect: "/components/",
		},
		{
			name:   "qualified Ref + slug param",
			ref:    "Patterns.Detail",
			params: map[string]any{"slug": "form-layout"},
			expect: "/patterns/form-layout",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			var err error
			if tc.params != nil {
				got, err = URLFor(ctx, Ref(tc.ref), tc.params)
			} else {
				got, err = URLFor(ctx, Ref(tc.ref))
			}
			if err != nil {
				t.Fatalf("URLFor(Ref(%q)): %v", tc.ref, err)
			}
			if got != tc.expect {
				t.Errorf("got %q, want %q", got, tc.expect)
			}
		})
	}
}

func TestRef_qualifiedPathErrors(t *testing.T) {
	pc, err := parsePageTree("/", &ambiguousRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	t.Run("unknown anchor", func(t *testing.T) {
		_, err := URLFor(ctx, Ref("Nope.Index"))
		if err == nil {
			t.Fatal("expected error for unknown anchor")
		}
		if !strings.Contains(err.Error(), "anchor") {
			t.Errorf("error %q missing 'anchor' hint", err.Error())
		}
	})

	t.Run("unknown descent segment", func(t *testing.T) {
		_, err := URLFor(ctx, Ref("Components.DoesNotExist"))
		if err == nil {
			t.Fatal("expected error for unknown segment")
		}
		msg := err.Error()
		if !strings.Contains(msg, "DoesNotExist") {
			t.Errorf("error %q does not name the missing segment", msg)
		}
		// Should list available children at the failure level.
		for _, want := range []string{"Index", "Detail"} {
			if !strings.Contains(msg, want) {
				t.Errorf("error %q does not list available %q", msg, want)
			}
		}
	})
}

// TestStrictAmbiguity_errorRecommendsChain pins that the strict-mode
// error message now points users at the []any chain form (recommended)
// and Ref (fallback), since Child is gone.
func TestStrictAmbiguity_errorRecommendsChain(t *testing.T) {
	pc, err := parsePageTree("/", &ambiguousRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	_, err = URLFor(ctx, sharedIndex{})
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	msg := err.Error()
	for _, want := range []string{"[]any", "chain", "Ref"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing recommendation %q", msg, want)
		}
	}
	// Should NOT mention the removed Child API anymore.
	if strings.Contains(msg, "Child(") {
		t.Errorf("error %q still mentions removed Child API", msg)
	}
}

// Fixtures for nested-anchor qualified Ref resolution. Mirrors the
// his-project shape (Authed > Receptionist > Patients): the anchor segment
// of a qualified Ref isn't a top-level node, but it's unique in the tree, so
// the Ref must resolve without naming the structural wrapper above it.
type naLeaf struct{}
type naOther struct{}

func (naLeaf) Page() component  { return testComponent{"na-leaf"} }
func (naOther) Page() component { return testComponent{"na-other"} }

type naSection struct {
	Leaf naLeaf `route:"/{$} Leaf"`
}
type naWrapper struct {
	Section naSection `route:"/section Section"`
	Other   naOther   `route:"/other Other"`
}
type naRoot struct {
	Wrapper naWrapper `route:"/ Wrapper"`
}

func TestRef_qualifiedNestedAnchor(t *testing.T) {
	pc, err := parsePageTree("/", &naRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	// "Section" is a grandchild (under Wrapper), not a top-level node, but
	// it's unique — a qualified Ref anchored on it must resolve.
	got, err := URLFor(ctx, Ref("Section.Leaf"))
	if err != nil {
		t.Fatalf("URLFor(Ref(\"Section.Leaf\")): %v", err)
	}
	if got != "/section/" {
		t.Errorf("got %q, want %q", got, "/section/")
	}
}

// naDupRoot mounts naWrapper twice, so "Section" names two nodes — a
// qualified Ref anchored on it is ambiguous and must error, not silently
// pick one.
type naDupRoot struct {
	A naWrapper `route:"/a A"`
	B naWrapper `route:"/b B"`
}

func TestRef_qualifiedAmbiguousAnchor(t *testing.T) {
	pc, err := parsePageTree("/", &naDupRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	_, err = URLFor(ctx, Ref("Section.Leaf"))
	if err == nil {
		t.Fatal("expected ambiguous-anchor error for duplicated 'Section'")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("error %q missing 'ambiguous' hint", err.Error())
	}
}
