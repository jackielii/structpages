// structpages-lint validates structpages.URLFor / ID / IDTarget / Ref
// call sites against the page tree reconstructed from struct tags.
//
// Usage:
//
//	structpages-lint [packages...]
//
// Defaults to ./... when no packages are given. Exits 1 if any
// diagnostic is reported, 0 otherwise.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jackielii/structpages/tools/lint"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: structpages-lint [packages...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	patterns := flag.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedSyntax |
			packages.NeedTypesInfo | packages.NeedDeps | packages.NeedImports |
			packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedModule,
		Tests: true,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load: %v\n", err)
		os.Exit(2)
	}
	if packages.PrintErrors(pkgs) > 0 {
		os.Exit(1)
	}

	tree, treeDiags := lint.BuildTree(pkgs)
	emitted := 0
	for _, d := range treeDiags {
		fmt.Printf("%s: %s\n", d.Pos, d.Message)
		emitted++
	}
	if len(tree.Roots) == 0 {
		fmt.Fprintln(os.Stderr,
			"structpages-lint: no structpages.Mount(...) call found in scope; "+
				"pass packages containing Mount, or run from module root")
		if emitted == 0 {
			os.Exit(0)
		}
		os.Exit(1)
	}

	analyzer := lint.NewAnalyzer(tree)
	seen := map[string]bool{}
	emit := func(line string) {
		if seen[line] {
			return
		}
		seen[line] = true
		fmt.Println(line)
		emitted++
	}
	for _, pkg := range pkgs {
		runPass(analyzer, pkg, emit)
	}
	if emitted > 0 {
		os.Exit(1)
	}
}

// runPass runs analyzer.Run against a single package, handing every
// reported diagnostic to emit (which deduplicates across passes).
func runPass(a *analysis.Analyzer, pkg *packages.Package, emit func(string)) {
	if pkg.TypesInfo == nil || pkg.Types == nil || len(pkg.Syntax) == 0 {
		return
	}
	pass := &analysis.Pass{
		Analyzer:   a,
		Fset:       pkg.Fset,
		Files:      pkg.Syntax,
		OtherFiles: pkg.OtherFiles,
		Pkg:        pkg.Types,
		TypesInfo:  pkg.TypesInfo,
		Report: func(d analysis.Diagnostic) {
			pos := pkg.Fset.Position(d.Pos)
			emit(fmt.Sprintf("%s: %s", pos, d.Message))
		},
		ResultOf: map[*analysis.Analyzer]any{},
	}
	if _, err := a.Run(pass); err != nil {
		fmt.Fprintf(os.Stderr, "analyzer error in %s: %v\n", pkg.PkgPath, err)
	}
}
