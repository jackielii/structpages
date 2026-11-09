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
//	structpages-lint [-tags tag,tag]... [-cache=false] [packages...]
//
// Defaults to ./... when no packages are given. Exits 0 when there are
// no findings, 1 when there are, and 2 when the packages could not be
// loaded or type-checked (nothing was analysed).
//
// Repeat -tags to lint several build configurations in one run. A run
// whose inputs match an earlier one's reuses its findings (see
// cache.go); -cache=false always analyses.
package main

import (
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"slices"
	"strings"
	"sync"

	"github.com/jackielii/structpages/tools/lint"
	"github.com/jackielii/structpages/tools/lint/gsxscan"
	"github.com/jackielii/structpages/tools/lint/templscan"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

func main() {
	var tags tagSets
	flag.Var(&tags, "tags", "comma-separated build tags to load packages under "+
		"(e.g. -tags devtools), so URLFor/ID call sites in code gated behind those tags "+
		"resolve against the tree that actually mounts them. Repeat it to lint several "+
		"build configurations in one run: -tags '' -tags devtools lints the default build "+
		"and the devtools one, printing their findings together")
	useCache := flag.Bool("cache", true, "reuse the findings of an earlier run whose inputs "+
		"(files, modules, toolchain, flags, this binary) are unchanged; -cache=false always analyses. "+
		"Results live in $STRUCTPAGES_LINT_CACHE, default structpages-lint in the user cache directory")
	cpuprofile := flag.String("cpuprofile", "", "write a CPU profile to `file`")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: structpages-lint [-tags tag,tag]... [-cache=false] [packages...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cpuprofile: %v\n", err)
			os.Exit(2)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "cpuprofile: %v\n", err)
			os.Exit(2)
		}
	}
	if len(tags) == 0 {
		tags = tagSets{""}
	}
	code := run(os.Stdout, tags, *useCache, flag.Args())
	pprof.StopCPUProfile()
	os.Exit(code)
}

// tagSets collects the -tags flags, one build configuration each.
type tagSets []string

func (t *tagSets) String() string     { return strings.Join(*t, " ") }
func (t *tagSets) Set(v string) error { *t = append(*t, v); return nil }

// run lints patterns under each build configuration in tagSets and
// returns the exit code: 2 if any configuration failed to load, else 1
// if any has findings, else 0. The findings of all configurations are
// printed together, de-duplicated and sorted.
//
// The configurations' package graphs load concurrently; that's cheap,
// and all a configuration needs when its result is cached. Analyses run
// one at a time, since each holds its packages' syntax and types.
func run(stdout io.Writer, tagSets []string, useCache bool, patterns []string) int {
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	type config struct {
		tags  string
		graph *lint.Graph
		err   error
		out   string
		code  int
	}
	configs := make([]*config, len(tagSets))
	var wg sync.WaitGroup
	for i, tags := range tagSets {
		c := &config{tags: tags}
		configs[i] = c
		wg.Add(1)
		go func() {
			defer wg.Done()
			var buildFlags []string
			if c.tags != "" {
				buildFlags = []string{"-tags=" + c.tags}
			}
			c.graph, c.err = lint.LoadGraph("", buildFlags, patterns...)
		}()
	}
	wg.Wait()

	code := 0
	lines := map[string]bool{}
	for _, c := range configs {
		if c.err != nil {
			fmt.Fprintf(os.Stderr, "load: %v\n", c.err)
			code = 2
			continue
		}
		var key string
		cached := false
		if useCache {
			key = cacheKey(c.graph, c.tags, patterns)
			c.out, c.code, cached = cacheGet(key)
		}
		if !cached {
			c.out, c.code = analyse(c.graph)
			if key != "" && c.code != 2 {
				cachePut(key, c.out, c.code)
			}
		}
		code = max(code, c.code)
		for _, line := range strings.SplitAfter(c.out, "\n") {
			if line != "" {
				lines[line] = true
			}
		}
	}
	for _, line := range slices.Sorted(maps.Keys(lines)) {
		io.WriteString(stdout, line)
	}
	return code
}

// analyse loads and checks graph's packages, returning the findings,
// one per line, and the exit code. Load errors go to stderr, with exit
// code 2.
func analyse(graph *lint.Graph) (string, int) {
	pkgs, err := graph.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load: %v\n", err)
		return "", 2
	}
	if packages.PrintErrors(pkgs) > 0 {
		return "", 2
	}

	var out strings.Builder
	tree, treeDiags := lint.BuildTree(pkgs)
	for _, d := range treeDiags {
		fmt.Fprintf(&out, "%s: %s\n", d.Pos, d.Message)
	}
	if len(tree.Roots) == 0 {
		fmt.Fprintln(os.Stderr,
			"structpages-lint: no structpages.Mount(...) call found in scope; "+
				"pass packages containing Mount, or run from module root")
		if out.Len() == 0 {
			return "", 0
		}
		return out.String(), 1
	}

	analyzer := lint.NewAnalyzer(tree)
	var (
		mu    sync.Mutex
		lines = map[string]bool{}
	)
	emit := func(line string) {
		mu.Lock()
		lines[line] = true
		mu.Unlock()
	}
	// Packages and template directories are checked concurrently; the
	// findings are de-duplicated (one file is seen by each of its
	// package's variants) and printed sorted, so the output doesn't
	// depend on scheduling.
	var dirs []string
	scannedDirs := map[string]bool{}
	for _, pkg := range pkgs {
		if dir := pkgDir(pkg); dir != "" && !scannedDirs[dir] {
			scannedDirs[dir] = true
			dirs = append(dirs, dir)
		}
	}
	parallel(len(pkgs), func(i int) {
		pkg := pkgs[i]
		err := lint.Check(analyzer, tree, pkg, func(d analysis.Diagnostic) {
			emit(fmt.Sprintf("%s: %s", pkg.Fset.Position(d.Pos), d.Message))
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "analyzer error in %s: %v\n", pkg.PkgPath, err)
		}
	})
	parallel(len(dirs), func(i int) { scanTemplates(dirs[i], emit) })
	for _, line := range slices.Sorted(maps.Keys(lines)) {
		out.WriteString(line + "\n")
	}
	if out.Len() > 0 {
		return out.String(), 1
	}
	return "", 0
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

// parallel calls f(0) … f(n-1) on GOMAXPROCS goroutines and waits.
func parallel(n int, f func(int)) {
	next := make(chan int)
	var wg sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				f(i)
			}
		}()
	}
	for i := range n {
		next <- i
	}
	close(next)
	wg.Wait()
}
