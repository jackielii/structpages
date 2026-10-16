package lint

import (
	"fmt"
	"go/token"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

// moduleDiags loads the testdata module in dir and returns its
// diagnostics in the given categories as "file:line: message",
// deduplicated across the package variants packages.Load reports.
func moduleDiags(t *testing.T, dir string, categories ...string) []string {
	t.Helper()
	seen := map[string]bool{}
	for _, d := range loadDiags(t, dir, categories) {
		seen[fmt.Sprintf("%s:%d: %s", filepath.Base(d.pos.Filename), d.pos.Line, d.msg)] = true
	}
	got := make([]string, 0, len(seen))
	for d := range seen {
		got = append(got, d)
	}
	sort.Strings(got)
	return got
}

// moduleDiagsAt is moduleDiags reduced to the "file:line:col" of each
// diagnostic.
func moduleDiagsAt(t *testing.T, dir string, categories ...string) map[string]bool {
	t.Helper()
	at := map[string]bool{}
	for _, d := range loadDiags(t, dir, categories) {
		at[fmt.Sprintf("%s:%d:%d", filepath.Base(d.pos.Filename), d.pos.Line, d.pos.Column)] = true
	}
	return at
}

// diag is a diagnostic with its position resolved.
type diag struct {
	pos token.Position
	msg string
}

// loadDiags loads the testdata module in dir and runs the analyzer over
// it, returning the diagnostics in the given categories.
func loadDiags(t *testing.T, dir string, categories []string) []diag {
	t.Helper()
	cfg := &packages.Config{
		Dir: dir,
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
	var out []diag
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
				if slices.Contains(categories, d.Category) {
					out = append(out, diag{pkg.Fset.Position(d.Pos), d.Message})
				}
			},
			ResultOf: map[*analysis.Analyzer]any{},
		}
		if _, err := analyzer.Run(pass); err != nil {
			t.Fatalf("analyzer run %s: %v", pkg.PkgPath, err)
		}
	}
	return out
}

// TestRender_Integration covers RenderTarget.Is arguments (idfor) and
// Mount calls serving htmx templates without a target selector
// (selector) in testdata/render, and that the mounts' own route
// arguments aren't reported as route literals. Each "want" comment there has exactly
// one entry here; every other case must stay quiet.
func TestRender_Integration(t *testing.T) {
	const sel = "Mount has no structpages.WithTargetSelector option, but its pages' templates use htmx " +
		"(%s); structpages has no default target selector since v0.9.0, so htmx partial requests " +
		"render the full Page. Pass structpages.WithTargetSelector(structpages.HTMXRenderTarget) " +
		"for htmx 1/2, or structpages.WithTargetSelector(structpages.HTMXv4RenderTarget) for htmx 4"
	pagesUse := "hx-target at ../../pages/pages.gsx:5:11"
	const notFunc = "not a method or function: structpages' target selectors only ever match a method or " +
		"function component (a custom RenderTarget might match it); pass the component method or function itself"
	want := []string{
		`main.go:19: [selector] ` + fmt.Sprintf(sel, pagesUse),
		`main.go:26: [selector] ` + fmt.Sprintf(sel, pagesUse),
		`main.go:32: [selector] ` + fmt.Sprintf(sel, pagesUse),
		`main.go:33: [selector] ` + fmt.Sprintf(sel, pagesUse),
		`main.go:40: [selector] ` + fmt.Sprintf(sel, "hx-target:inherited at ../../tpl/home.templ:4:7"),
		`main.go:42: [selector] ` + fmt.Sprintf(sel, pagesUse),
		`pages.go:48: [idfor] RenderTarget.Is: p.Content() calls the component; ` +
			`pass the method or function itself (p.Content), not its result`,
		`pages.go:49: [idfor] RenderTarget.Is: index{}.Content() calls the component; ` +
			`pass the method or function itself (index{}.Content), not its result`,
		`pages.go:50: [idfor] RenderTarget.Is: p.Page() calls the component; ` +
			`pass the method or function itself (p.Page), not its result`,
		`pages.go:51: [idfor] RenderTarget.Is: method expression Widget.Body: ` +
			`receiver type Widget is not mounted as a page`,
		`pages.go:52: [idfor] RenderTarget.Is: method value Widget.Body: ` +
			`receiver type Widget is not mounted as a page`,
		`pages.go:53: [idfor] RenderTarget.Is: argument structpages.Ref("index.Content") has type ` +
			`github.com/jackielii/structpages.Ref, ` + notFunc,
		`pages.go:54: [idfor] RenderTarget.Is: argument "content" has type string, ` + notFunc,
	}
	if diff := cmp.Diff(want, moduleDiags(t, "testdata/render", "idfor", selectorCategory, "route-literal")); diff != "" {
		t.Errorf("diagnostics (-want +got):\n%s", diff)
	}
}
