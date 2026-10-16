package lint

import (
	"go/ast"

	"golang.org/x/tools/go/packages"
)

// mountCall is a single discovered structpages.Mount(...) or
// structpages.Parse(...) call: both build a page tree. The
// fields are stored as AST nodes so callers can re-resolve types via
// the owning package's TypesInfo.
type mountCall struct {
	Pkg     *packages.Package
	Call    *ast.CallExpr
	PageArg ast.Expr // the page value
	Route   string   // the route prefix
	// Options is the slice of variadic option expressions. Used by
	// future checks that want to look at WithMiddlewares etc. For now
	// we don't extract WithURLPrefix because the prefix doesn't affect
	// any of the v1 checks (see design doc, "URL prefix" rationale).
	Options []ast.Expr
}

// treeBuilders maps each structpages function that builds a page tree
// to the position of its page argument; the route prefix follows it.
// Parse is Mount without the mux: tests use it to get a PageContext.
var treeBuilders = map[string]int{"Mount": 1, "Parse": 0}

// findMounts returns every call to structpages.Mount or Parse in pkg.
func findMounts(pkg *packages.Package) []mountCall {
	var calls []mountCall
	for _, file := range pkg.Syntax {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			pageIdx, ok := treeBuilderPageIndex(pkg, call)
			if !ok || len(call.Args) < pageIdx+3 {
				return true
			}
			route, ok := stringConstant(pkg, call.Args[pageIdx+1])
			if !ok {
				return true
			}
			mc := mountCall{
				Pkg:     pkg,
				Call:    call,
				PageArg: call.Args[pageIdx],
				Route:   route,
			}
			if len(call.Args) > pageIdx+3 {
				mc.Options = call.Args[pageIdx+3:]
			}
			calls = append(calls, mc)
			return true
		})
	}
	return calls
}

// treeBuilderPageIndex reports whether call is a structpages tree
// builder and, if so, the index of its page argument.
func treeBuilderPageIndex(pkg *packages.Package, call *ast.CallExpr) (int, bool) {
	for name, idx := range treeBuilders {
		if callFromPackage(pkg, call, name) {
			return idx, true
		}
	}
	return 0, false
}
