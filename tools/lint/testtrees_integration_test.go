package lint

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

// TestTestTrees_Integration loads testdata/testtrees, a module shaped
// like a large app: a production tree mounting a section through an
// alias, test trees that stub that section or mount it at a different
// root, a test tree built with Parse, trees mounted through helpers
// that take the page as an interface (chained, generic, with the route
// as a parameter, and from another package), nav refs held as data,
// and a separate client binary.
// Call sites are checked against the trees that can serve them, so only
// the genuine typos are reported, each worded from a tree that can run
// it.
func TestTestTrees_Integration(t *testing.T) {
	cfg := &packages.Config{
		Dir: "testdata/testtrees",
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedSyntax |
			packages.NeedTypesInfo | packages.NeedDeps | packages.NeedImports |
			packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedModule,
		Tests: true,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if n := packages.PrintErrors(pkgs); n > 0 {
		t.Fatalf("testdata module has %d load errors", n)
	}

	tree, treeDiags := BuildTree(pkgs)
	for _, d := range treeDiags {
		t.Errorf("unexpected tree diagnostic: %s: %s", d.Pos, d.Message)
	}

	// Test variants analyse the same file more than once; collect a set.
	seen := map[string]bool{}
	analyzer := NewAnalyzer(tree)
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil || pkg.Types == nil || len(pkg.Syntax) == 0 {
			continue
		}
		pass := &analysis.Pass{
			Analyzer:  analyzer,
			Fset:      pkg.Fset,
			Files:     pkg.Syntax,
			Pkg:       pkg.Types,
			TypesInfo: pkg.TypesInfo,
			Report: func(d analysis.Diagnostic) {
				pos := pkg.Fset.Position(d.Pos)
				seen[filepath.Base(pos.Filename)+": "+d.Message] = true
			},
			ResultOf: map[*analysis.Analyzer]any{},
		}
		if _, err := analyzer.Run(pass); err != nil {
			t.Fatalf("analyzer run %s: %v", pkg.PkgPath, err)
		}
	}
	got := make([]string, 0, len(seen))
	for d := range seen {
		got = append(got, d)
	}
	sort.Strings(got)
	want := []string{
		`orbit.go: [ref] Ref "Orbit.Nope": segment 1 ("Nope") not found as child of "Orbit"; available: Data, Home`,
		`rail_test.go: [ref] Ref "Plain.Home": anchor "Plain" not found in the page tree`,
		`routes_test.go: [ref] Ref "Orbit.Data.Nope": segment 2 ("Nope") not found as child of "Data"; available: Detail`,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("diagnostics (-want +got):\n%s", diff)
	}
}
