package lint

import (
	"go/ast"
	"go/token"
	"strconv"
)

// checkRefLiterals validates every structpages.Ref-typed string literal
// against the page tree, wherever it appears — a struct field value, a var
// or const, a return — not only an explicit structpages.Ref("…") call.
//
// Without this, a Ref stored as data (e.g. a nav table's `Page` column:
// `{Page: "Section.Leaf"}`) reaches URLFor unvalidated and only fails at
// runtime — exactly the false-green this analyzer exists to prevent.
//
// A stored Ref is validated as a URLFor ref (non-ID): the whole dotted path
// must resolve to a page. ID/IDTarget refs — whose last segment is a method,
// not a page — are virtually always written inline as IDTarget(ctx, Ref(…))
// calls, which checkRefConversion handles with the lenient either-context
// rule and which this check skips. A Ref deliberately stored for ID use is
// the rare exception; suppress it with //structpages:lint:ignore ref.
func checkRefLiterals(ctx *checkCtx) {
	info := ctx.pass.TypesInfo
	if info == nil {
		return
	}
	for _, f := range ctx.pass.Files {
		// Literals that are arguments to structpages.Ref(...) are validated
		// by checkRefConversion (with the same either-context rule), and
		// literals in comparisons read a ref rather than feed one to
		// URLFor/ID; skip both so we don't double-report.
		skip := skippableLiterals(ctx, f)
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING || skip[lit.Pos()] {
				return true
			}
			tv, ok := info.Types[lit]
			if !ok || !isRefType(tv.Type) {
				return true
			}
			val, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			// Validate as a URLFor ref: the full dotted path must resolve
			// to a page. resolveRef reports on failure.
			resolveRef(ctx, lit.Pos(), val, false)
			return true
		})
	}
}
