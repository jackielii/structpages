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
	framework := &packages.Package{
		ID: structpagesPkg, Name: "structpages", PkgPath: structpagesPkg,
	}
	shared := &packages.Package{
		ID: "ex/shared", Name: "shared", PkgPath: "ex/shared",
		Imports: map[string]*packages.Package{framework.PkgPath: framework},
	}
	// shared's test variant: only shared's own test binary compiles it.
	sharedTest := &packages.Package{
		ID: "ex/shared [ex/shared.test]", Name: "shared", PkgPath: "ex/shared",
		Imports: map[string]*packages.Package{framework.PkgPath: framework},
	}
	appMain := &packages.Package{
		ID: "ex/cmd/app", Name: "main", PkgPath: "ex/cmd/app",
		Imports: map[string]*packages.Package{shared.PkgPath: shared},
	}
	previewMain := &packages.Package{
		ID: "ex/cmd/preview", Name: "main", PkgPath: "ex/cmd/preview",
		Imports: map[string]*packages.Package{shared.PkgPath: shared},
	}
	// A test variant of the app main should fold into the same binary.
	appTestMain := &packages.Package{
		ID: "ex/cmd/app.test", Name: "main", PkgPath: "ex/cmd/app.test",
		Imports: map[string]*packages.Package{shared.PkgPath: shared},
	}
	sharedTestMain := &packages.Package{
		ID: "ex/shared.test", Name: "main", PkgPath: "ex/shared.test",
		Imports: map[string]*packages.Package{shared.PkgPath: sharedTest},
	}

	closures := mainClosures([]*packages.Package{framework, shared, appMain, previewMain, appTestMain, sharedTestMain})

	wantKeys := []string{"ex/cmd/app", "ex/cmd/preview", "ex/shared"}
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
			if !c.pkgs[p] {
				t.Errorf("closure[%q] missing package %q", k, p)
			}
		}
	}
	// A Mount in shared's _test.go file runs only in shared's tests.
	if closures["ex/cmd/app"].ids[sharedTest.ID] {
		t.Errorf("app binary compiles in %q", sharedTest.ID)
	}
	if !closures["ex/shared"].ids[sharedTest.ID] {
		t.Errorf("shared test binary missing %q", sharedTest.ID)
	}
}

func TestBinariesFor(t *testing.T) {
	bin := func(ids ...string) *binary {
		b := &binary{ids: map[string]bool{}, pkgs: map[string]bool{}}
		for _, id := range ids {
			b.ids[id] = true
			b.pkgs[normalizePkgPath(id)] = true
		}
		return b
	}
	closures := map[string]*binary{
		"ex/cmd/app":     bin("ex/cmd/app", "ex/shared"),
		"ex/cmd/preview": bin("ex/cmd/preview", "ex/shared"),
		"ex/shared":      bin("ex/shared.test", "ex/shared [ex/shared.test]"),
	}

	// A Mount call in the shared package belongs to both binaries.
	got := binariesFor([]string{"ex/shared"}, closures)
	if !got["ex/cmd/app"] || !got["ex/cmd/preview"] || got["ex/shared"] {
		t.Errorf("shared Mount: binaries = %v, want app and preview", got)
	}
	// A Mount call in a single main belongs only to that binary.
	if got := binariesFor([]string{"ex/cmd/app"}, closures); !got["ex/cmd/app"] || got["ex/cmd/preview"] {
		t.Errorf("app Mount: binaries = %v, want only app", got)
	}
	// A Mount call in shared's test file belongs only to its test binary.
	if got := binariesFor([]string{"ex/shared [ex/shared.test]"}, closures); len(got) != 1 || !got["ex/shared"] {
		t.Errorf("test-file Mount: binaries = %v, want only the shared test binary", got)
	}
	// A Mount call no main reaches yields nil (library-only fallback).
	if got := binariesFor([]string{"ex/orphan"}, closures); got != nil {
		t.Errorf("orphan Mount: binaries = %v, want nil", got)
	}
}

// TestRootsFor checks that a package is checked only against the trees
// whose binaries compile it in, that unattributed (library-only) roots
// apply everywhere, and that production roots come first.
func TestRootsFor(t *testing.T) {
	appTree := &PageNode{Name: "app", binaries: map[string]bool{"ex/cmd/app": true},
		reach: map[string]bool{"ex/cmd/app": true, "ex/shared": true}, prod: true}
	testTree := &PageNode{Name: "test", binaries: map[string]bool{"ex/shared": true},
		reach: map[string]bool{"ex/shared": true}}
	library := &PageNode{Name: "library"}
	tree := &PageTree{Roots: []*PageNode{testTree, library, appTree}}

	names := func(roots []*PageNode) []string {
		out := make([]string, 0, len(roots))
		for _, r := range roots {
			out = append(out, r.Name)
		}
		return out
	}
	for pkg, want := range map[string][]string{
		"ex/shared":                  {"app", "test", "library"},
		"ex/shared [ex/shared.test]": {"app", "test", "library"},
		"ex/cmd/app":                 {"app", "library"},
		"ex/cmd/seed":                {"library"},
	} {
		if got := names(tree.rootsFor(pkg)); !reflect.DeepEqual(got, want) {
			t.Errorf("rootsFor(%q) = %v, want %v", pkg, got, want)
		}
	}
}
