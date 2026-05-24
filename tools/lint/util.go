package lint

import (
	"go/ast"
	"go/constant"

	"golang.org/x/tools/go/packages"
)

// stringConstant reports the constant string value of expr in pkg's
// type info, or ("", false) if expr is not a constant string.
func stringConstant(pkg *packages.Package, expr ast.Expr) (string, bool) {
	if pkg.TypesInfo == nil {
		return "", false
	}
	tv, ok := pkg.TypesInfo.Types[expr]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

// stringConstantFromPass is the analysis.Pass variant of
// stringConstant.
func stringConstantFromPass(ctx *checkCtx, expr ast.Expr) (string, bool) {
	if ctx.pass.TypesInfo == nil {
		return "", false
	}
	tv, ok := ctx.pass.TypesInfo.Types[expr]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}
