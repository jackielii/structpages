// structpages-lint validates structpages.URLFor / ID / IDTarget / Ref
// call sites against the page tree reconstructed from struct tags, and
// flags route string literals that should be resolved through URLFor.
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
	"path/filepath"

	"github.com/jackielii/structpages/tools/lint"
	"github.com/jackielii/structpages/tools/lint/templscan"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

func main() {
	tags := flag.String("tags", "", "comma-separated build tags to load packages under "+
		"(e.g. -tags devtools), so URLFor/ID call sites in code gated behind those tags "+
		"resolve against the tree that actually mounts them")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: structpages-lint [-tags tag,tag] [packages...]\n")
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
	if *tags != "" {
		cfg.BuildFlags = []string{"-tags=" + *tags}
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
		runTemplScan(pkg, emit)
	}
	if emitted > 0 {
		os.Exit(1)
	}
}

// runTemplScan finds every .templ file in pkg's directory and emits
// diagnostics from templscan.Scan via emit. Each package is scanned
// once even though packages.Load may surface it multiple times
// (e.g. `pkg` and `pkg.test`); dedup happens in emit.
func runTemplScan(pkg *packages.Package, emit func(string)) {
	if len(pkg.GoFiles) == 0 {
		return
	}
	dir := filepath.Dir(pkg.GoFiles[0])
	matches, err := filepath.Glob(filepath.Join(dir, "*.templ"))
	if err != nil {
		return
	}
	for _, m := range matches {
		diags, err := templscan.Scan(m, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "templscan %s: %v\n", m, err)
			continue
		}
		for _, d := range diags {
			emit(fmt.Sprintf("%s:%d:%d: %s", d.Filename, d.Line, d.Col, d.Message))
		}
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
