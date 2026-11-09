package lint

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

// TestLoad_typesOnlyWhereStructpagesIsReachable loads testdata/untyped
// and pins how Load splits it: packages that depend on structpages come
// type-checked; packages that don't (client, and pages, whose route
// structs never import structpages) come parsed, once, from their
// non-test files. The tree still reads pages' types, from the export
// data the Mount package imports. The route-literal check still reports
// in a parsed package, minus the literal in a comparison, and the test
// file's literal is left alone.
func TestLoad_typesOnlyWhereStructpagesIsReachable(t *testing.T) {
	pkgs, err := Load("testdata/untyped", nil, "./...")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if n := packages.PrintErrors(pkgs); n > 0 {
		t.Fatalf("testdata module has %d load errors", n)
	}

	var typed, parsed []string
	for _, p := range pkgs {
		if p.Types != nil && p.TypesInfo != nil {
			typed = append(typed, p.ID)
		} else {
			parsed = append(parsed, p.ID)
		}
	}
	slices.Sort(typed)
	if want := []string{"ex/cmd/app"}; !slices.Equal(typed, want) {
		t.Errorf("type-checked %q, want %q", typed, want)
	}
	slices.Sort(parsed)
	if want := []string{"ex/client", "ex/pages"}; !slices.Equal(parsed, want) {
		t.Fatalf("parsed %q, want %q", parsed, want)
	}
	for _, p := range pkgs {
		if p.ID != "ex/client" {
			continue
		}
		if len(p.Syntax) != 1 || !strings.HasSuffix(p.Fset.Position(p.Syntax[0].Pos()).Filename, "client.go") {
			t.Errorf("client parsed %d files, want client.go alone", len(p.Syntax))
		}
	}

	tree, diags := BuildTree(pkgs)
	for _, d := range diags {
		t.Errorf("unexpected tree diagnostic: %s: %s", d.Pos, d.Message)
	}
	analyzer := NewAnalyzer(tree)
	var got []string
	for _, pkg := range pkgs {
		report := func(d analysis.Diagnostic) {
			p := pkg.Fset.Position(d.Pos)
			got = append(got, filepath.Base(p.Filename)+":"+strconv.Itoa(p.Line)+": "+d.Message)
		}
		if err := Check(analyzer, tree, pkg, report); err != nil {
			t.Fatalf("check %s: %v", pkg.ID, err)
		}
	}
	want := []string{`client.go:7: [route-literal] route literal "/about" matches mounted page About; ` +
		`resolve it with structpages.URLFor(ctx, about{}) so renames are caught here instead of drifting`}
	if !slices.Equal(got, want) {
		t.Errorf("diagnostics:\n got %q\nwant %q", got, want)
	}
}
