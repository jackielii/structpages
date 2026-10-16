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
		Doc:  "checks structpages.URLFor / ID / IDTarget / Ref calls against the page tree, and flags route string literals that should use URLFor",
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
	// roots are the trees that can serve this package's code (see
	// PageTree.rootsFor). Inside inSomeTree it is narrowed to one root
	// at a time, so the resolvers simply walk ctx.roots.
	roots []*PageNode
	dm    *directiveMap
	// silent suppresses diagnostic emission when true. Used by helper
	// callers (e.g. urlfor.go using resolveRefSilent) to obtain a
	// resolved node without re-reporting failures the Ref-conversion
	// visitor has already surfaced.
	silent bool
	// silenced counts reports dropped because silent was set, so a
	// caller can resolve quietly yet still learn whether resolution
	// would have produced a diagnostic.
	silenced int
}

func run(pass *analysis.Pass, tree *PageTree) {
	dm := newDirectiveMap(pass.Fset, pass.Files)
	ctx := &checkCtx{pass: pass, tree: tree, roots: tree.rootsFor(pass.Pkg.Path()), dm: dm}
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
	checkRouteLiterals(ctx)
	checkRefLiterals(ctx)
}

// inSomeTree runs check against each candidate tree in turn and
// reports nothing if any tree passes it cleanly. Otherwise it reruns
// check against the preferred (first) tree, reporting its diagnostics.
//
// This mirrors the runtime, which resolves against the single tree
// serving the request: a test tree that stubs out a section, or mounts
// a sub-tree at "/" instead of "/orbit", doesn't make a call that is
// valid in the production tree wrong.
func (ctx *checkCtx) inSomeTree(check func()) {
	roots := ctx.roots
	if len(roots) <= 1 {
		check()
		return
	}
	defer func() { ctx.roots = roots }()
	silent, silenced := ctx.silent, ctx.silenced
	ctx.silent = true
	for _, r := range roots {
		ctx.roots = []*PageNode{r}
		before := ctx.silenced
		check()
		if ctx.silenced == before {
			ctx.silent, ctx.silenced = silent, silenced
			return
		}
	}
	ctx.silent, ctx.silenced = silent, silenced
	ctx.roots = roots[:1]
	check()
}

// visitCall identifies what kind of structpages call this is and
// checks it against the trees that can serve it.
func visitCall(ctx *checkCtx, call *ast.CallExpr) {
	ctx.inSomeTree(func() { checkCall(ctx, call) })
}

// checkCall invokes the per-family check for call.
func checkCall(ctx *checkCtx, call *ast.CallExpr) {
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
		ctx.silenced++
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
