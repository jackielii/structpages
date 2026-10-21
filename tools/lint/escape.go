package lint

import (
	"go/ast"
	"go/types"
)

// checkPreEscaped reports URLFor values already escaped with net/url.
// URLFor escapes every value for the part of the URL its placeholder
// sits in, so a value escaped first is escaped twice: the handler reads
// url.QueryEscape("A&B") back as "A%26B", not "A&B". args starts after
// the page argument. Values are checked positionally, as key/value
// pairs, and as the values of a literal map; a call spreading a slice
// (args...) hides its values and is skipped.
//
// The check needs no page tree: whichever tree serves the call, the
// value is escaped again.
func checkPreEscaped(ctx *checkCtx, call *ast.CallExpr, args []ast.Expr) {
	if call.Ellipsis.IsValid() {
		return
	}
	for _, arg := range args {
		if lit, ok := unparen(arg).(*ast.CompositeLit); ok && isStringAnyMap(ctx.pass.TypesInfo, lit) {
			for _, e := range lit.Elts {
				if kv, ok := e.(*ast.KeyValueExpr); ok {
					reportPreEscaped(ctx, kv.Value)
				}
			}
			continue
		}
		reportPreEscaped(ctx, arg)
	}
}

func reportPreEscaped(ctx *checkCtx, expr ast.Expr) {
	call, ok := unparen(expr).(*ast.CallExpr)
	if !ok {
		return
	}
	fn := calleeFunc(ctx.pass.TypesInfo, call)
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != "net/url" {
		return
	}
	sig, _ := fn.Type().(*types.Signature)
	switch {
	case sig != nil && sig.Recv() == nil && (fn.Name() == "QueryEscape" || fn.Name() == "PathEscape"):
		ctx.report(expr.Pos(), "escape",
			"URLFor: url.%s value is escaped again by URLFor, "+
				"which escapes each value for its place in the URL; pass it unescaped",
			fn.Name())
	case sig != nil && sig.Recv() != nil && fn.Name() == "Encode" && isURLValues(sig.Recv().Type()):
		ctx.report(expr.Pos(), "escape",
			"URLFor: url.Values.Encode() as a value is escaped again by URLFor, so its & and = become data; "+
				"put the encoded query in the template instead: []any{page, \"?\" + v.Encode()}")
	}
}

// isURLValues reports whether t is net/url.Values or a pointer to it.
func isURLValues(t types.Type) bool {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "net/url" && obj.Name() == "Values"
}
