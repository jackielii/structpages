package lint

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"strings"

	"github.com/jackielii/structpages/tools/lint/gsxscan"
	"github.com/jackielii/structpages/tools/lint/internal/htmxattr"
	"github.com/jackielii/structpages/tools/lint/templscan"
	"golang.org/x/tools/go/packages"
)

// selectorCategory is the diagnostic category of a Mount that serves
// htmx markup without a target selector.
const selectorCategory = "selector"

// htmxMounts maps the position key of every structpages.Mount call
// (see mountCall.mount) to the first htmx markup found in the
// templates of the trees it serves. A tree's templates are the .gsx
// and .templ files in the directories of the packages that declare its
// page types, the files the linter's template scans read.
//
// mountRoots maps a Mount key to the root keys of the trees it serves
// (more than one when it sits in a helper called with several pages);
// roots maps a root key to its tree.
func htmxMounts(
	pkgs []*packages.Package, mountRoots map[string][]string, roots map[string]*PageNode,
) map[string]htmxattr.Use {
	dirs := pkgDirs(pkgs)
	scanned := map[string]*htmxattr.Use{} // dir -> first use (nil: none)
	scanDir := func(dir string) *htmxattr.Use {
		if use, ok := scanned[dir]; ok {
			return use
		}
		use := firstHTMX(dir)
		scanned[dir] = use
		return use
	}
	out := map[string]htmxattr.Use{}
	for mount, rks := range mountRoots {
		for _, rk := range rks {
			if use := treeHTMX(roots[rk], dirs, scanDir); use != nil {
				out[mount] = *use
				break
			}
		}
	}
	return out
}

// treeHTMX returns the first htmx use in the templates of the packages
// declaring root's page types, walking the tree depth-first.
func treeHTMX(root *PageNode, dirs map[string]string, scanDir func(string) *htmxattr.Use) *htmxattr.Use {
	if root == nil {
		return nil
	}
	var found *htmxattr.Use
	seen := map[string]bool{}
	root.All(func(n *PageNode) bool {
		pkg := n.Type.Obj().Pkg()
		if pkg == nil || seen[pkg.Path()] {
			return true
		}
		seen[pkg.Path()] = true
		dir, ok := dirs[pkg.Path()]
		if !ok {
			return true
		}
		found = scanDir(dir)
		return found == nil
	})
	return found
}

// pkgDirs maps every package reachable from pkgs to its source
// directory. Test variants share their package's path and directory.
func pkgDirs(pkgs []*packages.Package) map[string]string {
	dirs := map[string]string{}
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		if _, ok := dirs[p.PkgPath]; ok || len(p.GoFiles) == 0 {
			return
		}
		dirs[p.PkgPath] = filepath.Dir(p.GoFiles[0])
	})
	return dirs
}

// firstHTMX returns the first htmx markup in dir's .gsx files, then its
// .templ files, in file-name order; nil when there is none. A template
// that fails to parse contributes nothing here: the url-attr scan of
// the same file reports the parse error.
func firstHTMX(dir string) *htmxattr.Use {
	scans := []struct {
		glob string
		scan func(string) (htmxattr.Use, bool, error)
	}{
		{"*.gsx", gsxscan.HTMX},
		{"*.templ", templscan.HTMX},
	}
	for _, s := range scans {
		matches, err := filepath.Glob(filepath.Join(dir, s.glob))
		if err != nil {
			continue
		}
		for _, m := range matches {
			if use, ok, err := s.scan(m); err == nil && ok {
				return &use
			}
		}
	}
	return nil
}

// checkSelectors flags each structpages.Mount call in the package
// whose trees' templates use htmx while the call passes no
// structpages.WithTargetSelector option. Since v0.9.0 there is no
// default selector, so every htmx partial request to such a tree
// renders the full Page.
//
// Mounts in _test.go files are skipped: tests often mount a tree only to
// resolve URLs and ids, and a test that sends htmx requests without a
// selector fails on its own.
func checkSelectors(ctx *checkCtx) {
	if len(ctx.tree.htmxMounts) == 0 || ctx.pass.TypesInfo == nil {
		return
	}
	for _, f := range ctx.pass.Files {
		if tf := ctx.pass.Fset.File(f.Pos()); tf != nil &&
			strings.HasSuffix(tf.Name(), "_test.go") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fn := calleeFunc(ctx.pass.TypesInfo, call)
			if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != structpagesPkg || fn.Name() != "Mount" {
				return true
			}
			use, ok := ctx.tree.htmxMounts[posKey(ctx.pass.Fset, call.Pos())]
			if !ok {
				return true
			}
			if mountOptions(ctx.pass.TypesInfo, call) != optionsWithoutSelector {
				return true
			}
			where := use.Filename
			mountDir := filepath.Dir(ctx.pass.Fset.Position(call.Pos()).Filename)
			if rel, err := filepath.Rel(mountDir, use.Filename); err == nil {
				where = rel
			}
			ctx.report(call.Pos(), selectorCategory,
				"Mount has no structpages.WithTargetSelector option, but its pages' templates use htmx "+
					"(%s at %s:%d:%d); structpages has no default target selector since v0.9.0, "+
					"so htmx partial requests render the full Page. Pass "+
					"structpages.WithTargetSelector(structpages.HTMXRenderTarget) for htmx 1/2, or "+
					"structpages.WithTargetSelector(structpages.HTMXv4RenderTarget) for htmx 4",
				use.Name, where, use.Line, use.Col)
			return true
		})
	}
}

// optionsKind classifies the options a Mount call passes.
type optionsKind int

const (
	// optionsUnknown: some option can't be resolved statically — a
	// variable, a spread slice that isn't a literal, a function value
	// or a call to a function outside structpages, any of which may
	// wrap WithTargetSelector.
	optionsUnknown optionsKind = iota
	optionsWithSelector
	optionsWithoutSelector
)

// mountOptions classifies the options of the structpages.Mount call
// call. Each option must be a direct call to a structpages function
// for the call to count as having no selector; anything else the
// analyzer can't see through, and the call isn't flagged.
func mountOptions(info *types.Info, call *ast.CallExpr) optionsKind {
	const firstOption = 4 // options follow mux, page, route and title
	if len(call.Args) < firstOption {
		return optionsUnknown
	}
	opts := call.Args[firstOption:]
	if call.Ellipsis.IsValid() {
		// Mount(..., []structpages.Option{a, b}...) lists its options;
		// Mount(..., opts...) hides them.
		lit, ok := unparen(opts[0]).(*ast.CompositeLit)
		if !ok || len(opts) != 1 {
			return optionsUnknown
		}
		opts = lit.Elts
	}
	kind := optionsWithoutSelector
	for _, o := range opts {
		c, ok := unparen(o).(*ast.CallExpr)
		if !ok {
			return optionsUnknown
		}
		fn := calleeFunc(info, c)
		if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != structpagesPkg {
			return optionsUnknown
		}
		if fn.Name() == "WithTargetSelector" {
			kind = optionsWithSelector
		}
	}
	return kind
}
