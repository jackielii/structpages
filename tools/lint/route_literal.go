package lint

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

// checkRouteLiterals flags string literals whose value exactly equals a
// mounted page's full route and suggests structpages.URLFor with the page
// type instead. Hand-written URL strings drift silently when routes are
// renamed or remounted; routing them through URLFor makes this lint catch
// the break at build time. Category: "route-literal".
//
// The check is deliberately conservative to keep false positives near zero:
//   - only an EXACT match against a concrete full route counts — a route
//     with a {param}/{$} segment, a trailing-slash variant, or a query
//     suffix never matches, and a path prefix never matches;
//   - the bare root "/" is never flagged: it appears constantly as a
//     non-URL literal (path joins, defaults);
//   - structpages.Ref("/route") arguments are skipped — that's a sanctioned
//     route-string API the urlfor/ref checks already validate;
//   - literals in a comparison or switch-case are skipped — those read/match
//     a route (e.g. `switch node.Route { case "/foundations": }`) rather than
//     generate a URL, so URLFor does not apply;
//   - generated files (templ output) and _test.go files are skipped.
//
// Suppress an intentional literal with
// `//structpages:lint:ignore route-literal`.
func checkRouteLiterals(ctx *checkCtx) {
	routes := concreteRoutes(ctx.tree)
	if len(routes) == 0 {
		return
	}
	for _, f := range ctx.pass.Files {
		if tf := ctx.pass.Fset.File(f.Pos()); tf != nil &&
			strings.HasSuffix(tf.Name(), "_test.go") {
			continue
		}
		if isGeneratedFile(f) {
			continue
		}
		skip := skippableLiterals(ctx, f)
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING || skip[lit.Pos()] {
				return true
			}
			val, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			node, ok := routes[val]
			if !ok {
				return true
			}
			ctx.report(lit.Pos(), "route-literal",
				"route literal %q matches mounted page %s; resolve it with "+
					"structpages.URLFor(ctx, %s{}) so renames are caught here "+
					"instead of drifting", val, node.Name, node.Type.Obj().Name())
			return true
		})
	}
}

// concreteRoutes maps every concrete full route in the tree to a page that
// serves it. A route is concrete when it has no wildcard/param segment
// ({name}, {name...}, {$}) and is not the bare root "/". When several pages
// share a route (method splits like GET+POST on one path), a navigable
// (GET/ALL) page is preferred as the suggestion since URLFor builds URLs to
// follow, not to POST to.
func concreteRoutes(tree *PageTree) map[string]*PageNode {
	out := map[string]*PageNode{}
	for _, root := range tree.Roots {
		root.All(func(n *PageNode) bool {
			r := n.FullRoute
			if r == "" || r == "/" || strings.ContainsAny(r, "{}") {
				return true
			}
			if prev, ok := out[r]; !ok || preferAsSuggestion(n, prev) {
				out[r] = n
			}
			return true
		})
	}
	return out
}

// preferAsSuggestion reports whether cand is a better URLFor suggestion than
// the currently-recorded cur for the same route — a navigable method wins
// over a non-navigable one (POST/PUT/…).
func preferAsSuggestion(cand, cur *PageNode) bool {
	return isNavigable(cand.Method) && !isNavigable(cur.Method)
}

func isNavigable(method string) bool {
	return method == "" || method == "GET" || method == "ALL"
}

// skippableLiterals returns the positions of string literals in f that the
// route-literal check must not flag, because they are not URL-generation
// sites:
//   - arguments to structpages.Ref(...) — the sanctioned route-string API;
//   - operands of an == / != comparison — a route match, not a URL;
//   - values in a switch-case — likewise a route match (often `switch
//     node.Route { case "/x": }`).
func skippableLiterals(ctx *checkCtx, f *ast.File) map[token.Pos]bool {
	out := map[token.Pos]bool{}
	info := ctx.pass.TypesInfo
	mark := func(e ast.Expr) {
		if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			out[lit.Pos()] = true
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if isRefConversion(info, x) {
				mark(x.Args[0])
			}
		case *ast.BinaryExpr:
			if x.Op == token.EQL || x.Op == token.NEQ {
				mark(x.X)
				mark(x.Y)
			}
		case *ast.CaseClause:
			for _, e := range x.List {
				mark(e)
			}
		}
		return true
	})
	return out
}

// isGeneratedFile reports whether f carries the standard
// "// Code generated … DO NOT EDIT." marker on any line.
func isGeneratedFile(f *ast.File) bool {
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			if strings.HasPrefix(c.Text, "// Code generated ") &&
				strings.HasSuffix(c.Text, " DO NOT EDIT.") {
				return true
			}
		}
	}
	return false
}
