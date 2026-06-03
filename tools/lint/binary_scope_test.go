package lint

import (
	"reflect"
	"sort"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestNormalizePkgPath(t *testing.T) {
	cases := map[string]string{
		"ex/shared":                       "ex/shared",
		"ex/shared.test":                  "ex/shared",
		"ex/shared [ex/shared.test]":      "ex/shared",
		"ex/shared_test [ex/shared.test]": "ex/shared_test",
		"cmd/app":                         "cmd/app",
	}
	for in, want := range cases {
		if got := normalizePkgPath(in); got != want {
			t.Errorf("normalizePkgPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMainClosures(t *testing.T) {
	framework := &packages.Package{Name: "structpages", PkgPath: "github.com/jackielii/structpages"}
	shared := &packages.Package{
		Name: "shared", PkgPath: "ex/shared",
		Imports: map[string]*packages.Package{framework.PkgPath: framework},
	}
	appMain := &packages.Package{
		Name: "main", PkgPath: "ex/cmd/app",
		Imports: map[string]*packages.Package{shared.PkgPath: shared},
	}
	previewMain := &packages.Package{
		Name: "main", PkgPath: "ex/cmd/preview",
		Imports: map[string]*packages.Package{shared.PkgPath: shared},
	}
	// A test variant of the app main should fold into the same binary.
	appTestMain := &packages.Package{
		Name: "main", PkgPath: "ex/cmd/app.test",
		Imports: map[string]*packages.Package{shared.PkgPath: shared},
	}

	closures := mainClosures([]*packages.Package{framework, shared, appMain, previewMain, appTestMain})

	wantKeys := []string{"ex/cmd/app", "ex/cmd/preview"}
	var gotKeys []string
	for k := range closures {
		gotKeys = append(gotKeys, k)
	}
	sort.Strings(gotKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("closure keys = %v, want %v", gotKeys, wantKeys)
	}
	for _, k := range wantKeys {
		c := closures[k]
		for _, p := range []string{k, "ex/shared", "github.com/jackielii/structpages"} {
			if !c[p] {
				t.Errorf("closure[%q] missing %q", k, p)
			}
		}
	}
}

func TestBinariesFor(t *testing.T) {
	closures := map[string]map[string]bool{
		"ex/cmd/app":     {"ex/cmd/app": true, "ex/shared": true},
		"ex/cmd/preview": {"ex/cmd/preview": true, "ex/shared": true},
	}

	// A Mount call in the shared package belongs to both binaries.
	if got := binariesFor([]string{"ex/shared"}, closures); !got["ex/cmd/app"] || !got["ex/cmd/preview"] {
		t.Errorf("shared Mount: binaries = %v, want both", got)
	}
	// A Mount call in a single main belongs only to that binary.
	if got := binariesFor([]string{"ex/cmd/app"}, closures); !got["ex/cmd/app"] || got["ex/cmd/preview"] {
		t.Errorf("app Mount: binaries = %v, want only app", got)
	}
	// A Mount call no main reaches yields nil (library-only fallback).
	if got := binariesFor([]string{"ex/orphan"}, closures); got != nil {
		t.Errorf("orphan Mount: binaries = %v, want nil", got)
	}
}

// TestAmbiguousRoutes_PerBinary is the core regression for issue #22:
// a page mounted once per binary across two separate binaries is
// unambiguous, while a page mounted at two routes within one binary
// (or with no binary attribution at all) is ambiguous.
func TestAmbiguousRoutes_PerBinary(t *testing.T) {
	mkRoot := func(bins ...string) *PageNode {
		if len(bins) == 0 {
			return &PageNode{}
		}
		set := map[string]bool{}
		for _, b := range bins {
			set[b] = true
		}
		return &PageNode{binaries: set}
	}

	t.Run("once per binary across two binaries — not ambiguous", func(t *testing.T) {
		rootA := mkRoot("ex/cmd/preview")
		rootB := mkRoot("ex/cmd/app")
		matches := []nodeMatch{
			{node: &PageNode{FullRoute: "/{$}"}, root: rootA},
			{node: &PageNode{FullRoute: "/design-system/{$}"}, root: rootB},
		}
		if routes, ok := ambiguousRoutes(matches); ok {
			t.Errorf("ambiguous = true (routes %v), want false", routes)
		}
	})

	t.Run("twice within one binary — ambiguous", func(t *testing.T) {
		root := mkRoot("ex/cmd/app")
		matches := []nodeMatch{
			{node: &PageNode{FullRoute: "/a/{$}"}, root: root},
			{node: &PageNode{FullRoute: "/b/{$}"}, root: root},
		}
		routes, ok := ambiguousRoutes(matches)
		if !ok {
			t.Fatalf("ambiguous = false, want true")
		}
		want := []string{"/a/{$}", "/b/{$}"}
		if !reflect.DeepEqual(routes, want) {
			t.Errorf("routes = %v, want %v", routes, want)
		}
	})

	t.Run("same route within one binary — not ambiguous", func(t *testing.T) {
		root := mkRoot("ex/cmd/app")
		matches := []nodeMatch{
			{node: &PageNode{FullRoute: "/x/{$}"}, root: root},
			{node: &PageNode{FullRoute: "/x/{$}"}, root: root},
		}
		if routes, ok := ambiguousRoutes(matches); ok {
			t.Errorf("ambiguous = true (routes %v), want false", routes)
		}
	})

	t.Run("no binary attribution, two routes — ambiguous (library fallback)", func(t *testing.T) {
		matches := []nodeMatch{
			{node: &PageNode{FullRoute: "/a"}, root: mkRoot()},
			{node: &PageNode{FullRoute: "/b"}, root: mkRoot()},
		}
		if _, ok := ambiguousRoutes(matches); !ok {
			t.Errorf("ambiguous = false, want true")
		}
	})

	t.Run("shared mount in one binary plus standalone in another — ambiguous only if same binary sees two", func(t *testing.T) {
		// homePage reachable from binary A at two distinct routes →
		// ambiguous even though binary B sees only one.
		rootA := mkRoot("ex/cmd/app")
		rootShared := mkRoot("ex/cmd/app", "ex/cmd/preview")
		matches := []nodeMatch{
			{node: &PageNode{FullRoute: "/app/home"}, root: rootA},
			{node: &PageNode{FullRoute: "/home"}, root: rootShared},
		}
		if _, ok := ambiguousRoutes(matches); !ok {
			t.Errorf("ambiguous = false, want true (binary A sees two routes)")
		}
	})
}
