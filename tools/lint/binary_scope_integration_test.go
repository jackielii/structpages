package lint

import (
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

// TestBinaryScope_Integration loads testdata/twobins — a shared page
// tree mounted by two separate main packages (a standalone preview
// binary and the app, which nests the tree under /design-system) — and
// asserts the URLFor ambiguity check is scoped per binary (issue #22).
//
// A bare URLFor(homePage{}) in the shared package resolves to exactly
// one route in each binary, so it must produce no diagnostic even
// though, module-globally, the type is reachable at two routes.
func TestBinaryScope_Integration(t *testing.T) {
	cfg := &packages.Config{
		Dir: "testdata/twobins",
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

	tree, diags := BuildTree(pkgs)
	for _, d := range diags {
		t.Errorf("unexpected tree diagnostic: %s: %s", d.Pos, d.Message)
	}

	// Each root must be attributed to exactly the binary that mounts it.
	wantBins := map[string][]string{
		"ex/shared.Root":      {"ex/cmd/preview"}, // standalone gallery
		"ex/cmd/app.webPages": {"ex/cmd/app"},     // app wrapper
	}
	for _, root := range tree.Roots {
		key := typeKey(root.Type)
		want, ok := wantBins[key]
		if !ok {
			continue
		}
		var got []string
		for b := range root.binaries {
			got = append(got, b)
		}
		sort.Strings(got)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("root %s binaries = %v, want %v", key, got, want)
		}
		delete(wantBins, key)
	}
	for key := range wantBins {
		t.Errorf("root %s not found in tree", key)
	}

	// Run the analyzer over every package; assert no urlfor diagnostic.
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
				if strings.HasPrefix(d.Message, "[urlfor]") {
					t.Errorf("%s: unexpected diagnostic: %s",
						pkg.Fset.Position(d.Pos), d.Message)
				}
			},
			ResultOf: map[*analysis.Analyzer]any{},
		}
		if _, err := analyzer.Run(pass); err != nil {
			t.Fatalf("analyzer run %s: %v", pkg.PkgPath, err)
		}
	}
}
