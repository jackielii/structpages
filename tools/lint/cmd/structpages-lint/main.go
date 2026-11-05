// structpages-lint validates structpages.URLFor / ID / IDTarget / Key /
// KeyTarget / Ref / RenderTarget.Is call sites against the page tree
// reconstructed from struct tags, flags route string literals that
// should be resolved through URLFor — in Go code and in the URL attributes of .templ and
// .gsx templates — flags Mount calls that serve htmx templates without
// a target selector, flags pages whose .gsx regions carry another
// page's component id, and flags instance ids built by appending a
// runtime value to a generated id instead of with Key / KeyTarget.
//
// Usage:
//
//	structpages-lint [packages...]
//
// Defaults to ./... when no packages are given. Exits 0 when there are
// no findings, 1 when there are, and 2 when the packages could not be
// loaded or type-checked (nothing was analysed).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackielii/structpages/tools/lint"
	"github.com/jackielii/structpages/tools/lint/gsxscan"
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

	var buildFlags []string
	if *tags != "" {
		buildFlags = []string{"-tags=" + *tags}
	}
	pkgs, err := lint.Load("", buildFlags, patterns...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load: %v\n", err)
		os.Exit(2)
	}
	if packages.PrintErrors(pkgs) > 0 {
		os.Exit(2)
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
	scannedDirs := map[string]bool{}
	for _, pkg := range pkgs {
		runPass(analyzer, pkg, emit)
		if dir := pkgDir(pkg); dir != "" && !scannedDirs[dir] {
			scannedDirs[dir] = true
			scanTemplates(dir, emit)
		}
	}
	if emitted > 0 {
		os.Exit(1)
	}
}

// pkgDir is the source directory of pkg. packages.Load surfaces one
// directory under several variants (pkg, pkg [pkg.test], pkg_test,
// pkg.test); all report the same Dir. GoFiles is no substitute: the
// synthesised pkg.test main's lone file lives in the go build cache,
// and globbing a build-cache directory once per test binary dominated
// the run on large modules.
func pkgDir(pkg *packages.Package) string {
	if pkg.Dir != "" {
		return pkg.Dir
	}
	if len(pkg.GoFiles) > 0 {
		return filepath.Dir(pkg.GoFiles[0])
	}
	return ""
}

// scanTemplates runs templscan over every .templ file and gsxscan over
// every .gsx file in dir, emitting their diagnostics. The directory is
// read once for both. templscan.Diagnostic and gsxscan.Diagnostic are
// the same type.
func scanTemplates(dir string, emit func(string)) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	run := func(ext, scanner string, scan func(string) ([]templscan.Diagnostic, error)) {
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ext {
				continue
			}
			m := filepath.Join(dir, e.Name())
			diags, err := scan(m)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s %s: %v\n", scanner, m, err)
				continue
			}
			for _, d := range diags {
				emit(fmt.Sprintf("%s:%d:%d: %s", d.Filename, d.Line, d.Col, d.Message))
			}
		}
	}
	run(".templ", "templscan", func(name string) ([]templscan.Diagnostic, error) { return templscan.Scan(name, nil) })
	run(".gsx", "gsxscan", gsxscan.Scan)
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
