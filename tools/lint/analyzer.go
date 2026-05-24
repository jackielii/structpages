package lint

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// NewAnalyzer returns an analyzer that validates structpages calls
// against the supplied page tree. Callers build the tree once via
// BuildTree and inject it into every Pass through this factory.
func NewAnalyzer(tree *PageTree) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "structpageslint",
		Doc:  "checks structpages.URLFor / ID / IDTarget / Ref calls against the page tree",
		Run: func(pass *analysis.Pass) (any, error) {
			run(pass, tree)
			return nil, nil
		},
	}
}

// checkCtx is the shared context passed to each per-call check.
type checkCtx struct {
	pass *analysis.Pass
	tree *PageTree
	dm   *directiveMap
	// silent suppresses diagnostic emission when true. Used by helper
	// callers (e.g. urlfor.go using resolveRefSilent) to obtain a
	// resolved node without re-reporting failures the Ref-conversion
	// visitor has already surfaced.
	silent bool
}

func run(pass *analysis.Pass, tree *PageTree) {
	dm := newDirectiveMap(pass.Fset, pass.Files)
	ctx := &checkCtx{pass: pass, tree: tree, dm: dm}
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			visitCall(ctx, call)
			return true
		})
	}
}

// visitCall identifies what kind of structpages call this is and
// invokes the appropriate per-family check.
func visitCall(ctx *checkCtx, call *ast.CallExpr) {
	info := ctx.pass.TypesInfo
	if info == nil {
		return
	}
	// First: `structpages.Ref(...)` is a type conversion, not a func
	// call, so calleeFunc returns nil for it. Check that case
	// independently.
	if isRefConversion(info, call) {
		checkRefConversion(ctx, call)
		return
	}
	fn := calleeFunc(info, call)
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != structpagesPkg {
		return
	}
	switch fn.Name() {
	case "URLFor":
		checkURLFor(ctx, call)
	case "ID":
		checkIDOrIDTarget(ctx, call, "ID")
	case "IDTarget":
		checkIDOrIDTarget(ctx, call, "IDTarget")
	}
}

// report emits a diagnostic unless a directive suppresses it or the
// context is in silent mode.
func (ctx *checkCtx) report(pos token.Pos, category, format string, args ...any) {
	if ctx.silent {
		return
	}
	if ctx.dm.suppressed(pos, category) {
		return
	}
	ctx.pass.Report(analysis.Diagnostic{
		Pos:      pos,
		Category: category,
		Message:  "[" + category + "] " + fmt.Sprintf(format, args...),
	})
}

// isRefConversion reports whether call is a type conversion of the
// form `structpages.Ref(expr)`. Type conversions show up as
// *ast.CallExpr with a non-func callee (a *types.TypeName).
func isRefConversion(info *types.Info, call *ast.CallExpr) bool {
	if len(call.Args) != 1 {
		return false
	}
	var ident *ast.Ident
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		ident = fun
	case *ast.SelectorExpr:
		ident = fun.Sel
	default:
		return false
	}
	if ident == nil {
		return false
	}
	use, ok := info.Uses[ident]
	if !ok {
		return false
	}
	tn, ok := use.(*types.TypeName)
	if !ok {
		return false
	}
	if tn.Pkg() == nil || tn.Pkg().Path() != structpagesPkg {
		return false
	}
	return tn.Name() == "Ref"
}
