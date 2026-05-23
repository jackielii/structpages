package lint

import (
	"go/ast"

	"golang.org/x/tools/go/packages"
)

// mountCall is a single discovered structpages.Mount(...) call. The
// fields are stored as AST nodes so callers can re-resolve types via
// the owning package's TypesInfo.
type mountCall struct {
	Pkg     *packages.Package
	Call    *ast.CallExpr
	PageArg ast.Expr // second positional argument to Mount: the page value
	Route   string   // third positional argument: route prefix
	// Options is the slice of variadic option expressions. Used by
	// future checks that want to look at WithMiddlewares etc. For now
	// we don't extract WithURLPrefix because the prefix doesn't affect
	// any of the v1 checks (see design doc, "URL prefix" rationale).
	Options []ast.Expr
}

// findMounts returns every call to structpages.Mount in pkg.
func findMounts(pkg *packages.Package) []mountCall {
	var calls []mountCall
	for _, file := range pkg.Syntax {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if !callFromPackage(pkg, call, "Mount") {
				return true
			}
			if len(call.Args) < 3 {
				return true
			}
			route, ok := stringConstant(pkg, call.Args[2])
			if !ok {
				return true
			}
			mc := mountCall{
				Pkg:     pkg,
				Call:    call,
				PageArg: call.Args[1],
				Route:   route,
			}
			if len(call.Args) > 3 {
				mc.Options = call.Args[3:]
			}
			calls = append(calls, mc)
			return true
		})
	}
	return calls
}
