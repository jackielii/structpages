package lint

import (
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

// urlforDiags loads testdata/buildtags under the given build flags and returns
// the [urlfor] diagnostics the analyzer reports.
func urlforDiags(t *testing.T, buildFlags ...string) []string {
	t.Helper()
	cfg := &packages.Config{
		Dir: "testdata/buildtags",
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedSyntax |
			packages.NeedTypesInfo | packages.NeedDeps | packages.NeedImports |
			packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedModule,
		Tests:      true,
		BuildFlags: buildFlags,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if n := packages.PrintErrors(pkgs); n > 0 {
		t.Fatalf("testdata load errors: %d", n)
	}
	tree, _ := BuildTree(pkgs)
	analyzer := NewAnalyzer(tree)
	var diags []string
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
					diags = append(diags, d.Message)
				}
			},
			ResultOf: map[*analysis.Analyzer]any{},
		}
		if _, err := analyzer.Run(pass); err != nil {
			t.Fatalf("analyzer run %s: %v", pkg.PkgPath, err)
		}
	}
	return diags
}

// TestBuildTags_URLForResolvesUnderTag covers the -tags flag: a route gated
// behind //go:build devtools is unmounted in the default (lean) build, so a
// URLFor to it from always-compiled code is flagged; loading with
// -tags=devtools mounts it and the call resolves. The empty devGroup stub must
// also mount cleanly (no tree error) in the lean case.
func TestBuildTags_URLForResolvesUnderTag(t *testing.T) {
	if d := urlforDiags(t); len(d) == 0 {
		t.Error("without -tags: expected a [urlfor] diagnostic for the unmounted dev page, got none")
	}
	if d := urlforDiags(t, "-tags=devtools"); len(d) != 0 {
		t.Errorf("with -tags=devtools: expected no [urlfor] diagnostics, got %v", d)
	}
}
