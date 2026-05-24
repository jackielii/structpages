package templscan

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// isInternalPath reports whether s looks like a literal internal app
// URL: starts with "/" but not "//" (which would be a
// protocol-relative URL).
func isInternalPath(s string) bool {
	if !strings.HasPrefix(s, "/") {
		return false
	}
	if strings.HasPrefix(s, "//") {
		return false
	}
	return true
}

// literalDiagnostic formats the "hard-coded internal URL" message.
func literalDiagnostic(attrName, literal string) string {
	return fmt.Sprintf(
		"[%s] %s value %q is a hard-coded internal URL; use structpages.URLFor instead",
		categoryURLAttr, attrName, literal,
	)
}

// concatDiagnostic formats the "string concatenation" message.
func concatDiagnostic(attrName, src string) string {
	return fmt.Sprintf(
		"[%s] %s value `%s` builds an internal URL by string concatenation; use structpages.URLFor instead",
		categoryURLAttr, attrName, strings.TrimSpace(src),
	)
}

// sprintfDiagnostic formats the "fmt.Sprint*" message.
func sprintfDiagnostic(attrName, src string) string {
	return fmt.Sprintf(
		"[%s] %s value `%s` builds an internal URL via fmt.Sprint*; use structpages.URLFor instead",
		categoryURLAttr, attrName, strings.TrimSpace(src),
	)
}

// finding is one rule hit inside a parsed Go expression, with the
// position taken from the snippet's own fset (1-indexed line/col,
// where line 1 is the snippet's first line).
type finding struct {
	pos     token.Position
	message string
}

// checkGoExpr parses src as a Go expression and returns one finding
// per bad-URL shape discovered. attrName is used in the message.
//
// Recognised shapes:
//
//   - basic STRING literal that satisfies isInternalPath
//   - *ast.BinaryExpr with Op == token.ADD where either operand
//     transitively contains such a literal — flagged once at the
//     outermost concat node
//   - *ast.CallExpr whose callee selector is fmt.Sprintf / Sprint /
//     Sprintln and whose first arg is such a literal
//
// Parse errors return nil findings.
func checkGoExpr(attrName, src string) []finding {
	fset := token.NewFileSet()
	expr, err := parser.ParseExprFrom(fset, "", src, 0)
	if err != nil {
		return nil
	}
	var out []finding
	walkExpr(attrName, fset, src, expr, &out)
	return out
}

func walkExpr(attrName string, fset *token.FileSet, src string, e ast.Expr, out *[]finding) {
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind != token.STRING {
			return
		}
		s, err := strconv.Unquote(x.Value)
		if err != nil {
			return
		}
		if isInternalPath(s) {
			*out = append(*out, finding{
				pos:     fset.Position(x.Pos()),
				message: literalDiagnostic(attrName, s),
			})
		}
	case *ast.BinaryExpr:
		if x.Op != token.ADD {
			return
		}
		if containsInternalLit(x) {
			*out = append(*out, finding{
				pos:     fset.Position(x.Pos()),
				message: concatDiagnostic(attrName, exprText(fset, src, x)),
			})
		}
	case *ast.CallExpr:
		if !isFmtSprintLike(x.Fun) {
			return
		}
		if len(x.Args) == 0 {
			return
		}
		lit, ok := x.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			return
		}
		if !isInternalPath(s) {
			return
		}
		*out = append(*out, finding{
			pos:     fset.Position(x.Pos()),
			message: sprintfDiagnostic(attrName, exprText(fset, src, x)),
		})
	}
}

// containsInternalLit reports whether the expression tree under e
// contains any STRING literal that satisfies isInternalPath.
func containsInternalLit(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if found {
			return false
		}
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		if isInternalPath(s) {
			found = true
			return false
		}
		return true
	})
	return found
}

// isFmtSprintLike resolves syntactically — type info is unavailable
// when parsing an expression snippet in isolation. Matches the
// common case `fmt.Sprintf(...)`; aliased imports are not detected.
func isFmtSprintLike(fun ast.Expr) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "fmt" {
		return false
	}
	switch sel.Sel.Name {
	case "Sprintf", "Sprint", "Sprintln":
		return true
	}
	return false
}

// exprText returns the byte slice of src covered by e, using
// offsets from fset. Returns a placeholder if the range is invalid.
func exprText(fset *token.FileSet, src string, e ast.Expr) string {
	start := fset.Position(e.Pos())
	end := fset.Position(e.End())
	if !start.IsValid() || !end.IsValid() {
		return "<expr>"
	}
	if start.Offset < 0 || end.Offset > len(src) || start.Offset > end.Offset {
		return "<expr>"
	}
	return src[start.Offset:end.Offset]
}
