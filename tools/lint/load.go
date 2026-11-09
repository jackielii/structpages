package lint

import (
	"go/ast"
	"go/parser"
	"go/token"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

// Load loads patterns the way the analyzer needs them, test variants
// included, and returns two kinds of package:
//
//   - Packages that depend on structpages (it is among their transitive
//     imports) come with syntax and full type information. Only these
//     can call structpages, mount a tree or turn a string into a Ref, so
//     only these need type-checking.
//   - Every other package comes with syntax only (Types and TypesInfo
//     nil), parsed from its non-test files. The one check that can fire
//     there, a route literal, reads the syntax alone (see Check).
//
// Type-checking dominates a run, and in a large module most packages
// (domain, storage, service code) never reach structpages, so this
// split is most of what a run costs. A metadata-only load (go list
// -deps, no parsing or type-checking) finds which packages depend on
// structpages and also supplies the transitive import graph that binary
// scoping (mainClosures) walks: the typed packages' Imports are replaced
// with its packages.
//
// Typed packages don't ask go/packages for NeedDeps: that would parse
// and type-check every dependency (the standard library and every
// third-party module) from source. Dependencies are type-checked from
// compiler export data instead, as go vet does. Export data is written
// by the go toolchain and read by x/tools; a Go release newer than this
// build's x/tools can write a format it cannot decode. When that happens
// Load falls back to the slow source load rather than failing.
func Load(dir string, buildFlags []string, patterns ...string) ([]*packages.Package, error) {
	g, err := LoadGraph(dir, buildFlags, patterns...)
	if err != nil {
		return nil, err
	}
	return g.Load()
}

// Graph is the metadata of the packages a run covers: every package
// matching the patterns, test variants included, with its transitive
// imports, files and module, but no syntax or types. Loading it is
// cheap (go list -deps), and it names every input a run reads, which
// is what Fingerprint hashes.
type Graph struct {
	dir        string
	buildFlags []string
	// Roots are the packages matching the patterns.
	Roots []*packages.Package
}

// LoadGraph loads the metadata of patterns (see Graph).
func LoadGraph(dir string, buildFlags []string, patterns ...string) (*Graph, error) {
	roots, err := packages.Load(&packages.Config{
		Dir: dir,
		Mode: packages.NeedName | packages.NeedImports | packages.NeedDeps |
			packages.NeedFiles | packages.NeedModule,
		Tests:      true,
		BuildFlags: buildFlags,
	}, patterns...)
	if err != nil {
		return nil, err
	}
	return &Graph{dir: dir, buildFlags: buildFlags, Roots: roots}, nil
}

// Load loads the graph's packages the way the analyzer needs them (see
// the package-level Load).
func (g *Graph) Load() ([]*packages.Package, error) {
	dir, buildFlags, graph := g.dir, g.buildFlags, g.Roots
	var err error

	reaches := map[*packages.Package]bool{}
	var dependsOnStructpages func(p *packages.Package) bool
	dependsOnStructpages = func(p *packages.Package) bool {
		if r, ok := reaches[p]; ok {
			return r
		}
		reaches[p] = false // import cycles can't occur; this only guards re-entry
		r := p.PkgPath == structpagesPkg
		for _, imp := range p.Imports {
			if r {
				break
			}
			r = dependsOnStructpages(imp)
		}
		reaches[p] = r
		return r
	}

	// Type-check every variant of a package if any variant depends on
	// structpages: the typed load names packages, not variants, and a
	// package whose tests mount a tree is loaded with its tests.
	var typedPatterns []string
	typedBase := map[string]bool{}
	for _, p := range graph {
		if dependsOnStructpages(p) {
			if base := basePackage(p); !typedBase[base] {
				typedBase[base] = true
				typedPatterns = append(typedPatterns, base)
			}
		}
	}

	var typed []*packages.Package
	if len(typedPatterns) > 0 {
		typed, err = loadTyped(dir, buildFlags, typedPatterns)
		if err != nil {
			return nil, err
		}
		byID := map[string]*packages.Package{}
		packages.Visit(graph, nil, func(p *packages.Package) { byID[p.ID] = p })
		for _, p := range typed {
			for path, imp := range p.Imports {
				if full, ok := byID[imp.ID]; ok {
					p.Imports[path] = full
				}
			}
		}
	}

	// The rest are parsed for the route-literal check, which skips test
	// files: a package's test variants add only test files, so its plain
	// variant is the only one to parse.
	var untyped []*packages.Package
	for _, p := range graph {
		if typedBase[basePackage(p)] || p.ID != p.PkgPath || p.Name == "main" && strings.HasSuffix(p.PkgPath, ".test") {
			continue
		}
		untyped = append(untyped, p)
	}
	parseSyntax(untyped)
	return append(typed, untyped...), nil
}

// loadTyped loads patterns with syntax and full type information, deps
// from export data (see Load).
func loadTyped(dir string, buildFlags []string, patterns []string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Dir: dir,
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedSyntax |
			packages.NeedTypesInfo | packages.NeedImports |
			packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedModule,
		Tests:      true,
		BuildFlags: buildFlags,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}
	if exportDataUnreadable(pkgs) {
		cfg.Mode |= packages.NeedDeps
		return packages.Load(cfg, patterns...)
	}
	return pkgs, nil
}

// basePackage is the import path of the package p is a variant of:
// "foo" for "foo", "foo [foo.test]", "foo_test [foo.test]" and the
// synthesised test main "foo.test".
func basePackage(p *packages.Package) string {
	id := p.ID
	if i := strings.Index(id, " ["); i >= 0 && strings.HasSuffix(id, ".test]") {
		return strings.TrimSuffix(id[i+2:len(id)-1], ".test")
	}
	if p.Name == "main" && strings.HasSuffix(id, ".test") {
		return strings.TrimSuffix(id, ".test")
	}
	return p.PkgPath
}

// parseSyntax parses each package's non-test Go files into its Syntax,
// sharing one FileSet, in parallel. A file that fails to parse is left
// out; the type-checked load reports real syntax errors in packages that
// matter, and the route-literal check has nothing to say about a file it
// can't read.
func parseSyntax(pkgs []*packages.Package) {
	fset := token.NewFileSet()
	type job struct {
		pkg  *packages.Package
		i    int
		file string
	}
	var jobs []job
	for _, p := range pkgs {
		p.Fset = fset
		p.Syntax = make([]*ast.File, len(p.GoFiles))
		for i, f := range p.GoFiles {
			jobs = append(jobs, job{p, i, f})
		}
	}
	work := make(chan job)
	var wg sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range work {
				f, err := parser.ParseFile(fset, j.file, nil, parser.ParseComments|parser.SkipObjectResolution)
				if err == nil {
					j.pkg.Syntax[j.i] = f
				}
			}
		}()
	}
	for _, j := range jobs {
		work <- j
	}
	close(work)
	wg.Wait()
	for _, p := range pkgs {
		files := p.Syntax[:0]
		for _, f := range p.Syntax {
			if f != nil {
				files = append(files, f)
			}
		}
		p.Syntax = files
	}
}

// exportDataUnreadable reports whether any package failed to import a
// dependency because its export data is in a format this x/tools cannot
// decode.
func exportDataUnreadable(pkgs []*packages.Package) bool {
	found := false
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			if strings.Contains(e.Msg, "export data version") ||
				strings.Contains(e.Msg, "cannot decode") {
				found = true
			}
		}
	})
	return found
}
