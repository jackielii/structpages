package lint

import (
	"strings"

	"golang.org/x/tools/go/packages"
)

// Load loads patterns the way the analyzer needs them: syntax and full
// type information for the matched packages (test variants included),
// and the complete transitive import graph through Package.Imports.
//
// It deliberately does not ask go/packages for NeedDeps alongside
// syntax: that parses and type-checks every dependency — the standard
// library and every third-party module — from source, which on a large
// module dominates the run (tens of seconds, gigabytes of memory). The
// analyzer only inspects the matched packages' syntax; dependencies are
// type-checked from compiler export data instead, as go vet does.
//
// Without NeedDeps the Imports of the loaded packages are ID-only
// placeholders, but binary scoping (mainClosures) walks the transitive
// import graph, so a second, metadata-only load (go list -deps, no
// parsing or type-checking) supplies it and the placeholders are
// replaced with its packages.
//
// Export data is written by the go toolchain and read by x/tools; a Go
// release newer than this build's x/tools can write a format it cannot
// decode. When that happens Load falls back to the slow source load
// rather than failing.
func Load(dir string, buildFlags []string, patterns ...string) ([]*packages.Package, error) {
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

	graphCfg := &packages.Config{
		Dir:        dir,
		Mode:       packages.NeedName | packages.NeedImports | packages.NeedDeps,
		Tests:      true,
		BuildFlags: buildFlags,
	}
	graph, err := packages.Load(graphCfg, patterns...)
	if err != nil {
		return nil, err
	}
	byID := map[string]*packages.Package{}
	packages.Visit(graph, nil, func(p *packages.Package) { byID[p.ID] = p })

	for _, p := range pkgs {
		for path, imp := range p.Imports {
			if full, ok := byID[imp.ID]; ok {
				p.Imports[path] = full
			}
		}
	}
	return pkgs, nil
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
