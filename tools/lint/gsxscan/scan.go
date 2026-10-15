// Package gsxscan walks .gsx source files and emits [url-attr]
// diagnostics for URL-bearing HTML attributes whose values look like
// they should have used structpages.URLFor (the gsx `|> url` filter).
//
// It is the gsx counterpart of templscan: the attribute set, value
// classification, messages and structpages:lint:ignore semantics all
// come from the shared urlattr package; this package only maps gsx's
// AST onto them.
package gsxscan

import (
	"go/token"
	"os"
	"strings"

	"github.com/gsxhq/gsx/ast"
	"github.com/gsxhq/gsx/parser"
	"github.com/jackielii/structpages/tools/lint/internal/urlattr"
)

// Diagnostic is a single finding to report. Line/Col are 1-indexed
// positions in the .gsx source file.
type Diagnostic = urlattr.Diagnostic

// Scan parses filename and returns diagnostics. A parse error in the
// gsx file yields a single diagnostic and a nil error — Scan only
// returns a non-nil error for I/O problems.
func Scan(filename string) ([]Diagnostic, error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return []Diagnostic{{
			Filename: filename,
			Line:     1,
			Col:      1,
			Category: urlattr.Category,
			Message:  "[" + urlattr.Category + "] gsx parse error: " + err.Error(),
		}}, nil
	}

	// Directives are collected in a first pass: a trailing comment in
	// an attribute list (`href="/x" // structpages:lint:ignore`) is
	// visited after the attribute it suppresses.
	ds := directives(fset, f)

	var diags []Diagnostic
	emit := func(line, col int, msg string) {
		if ds.Suppressed(line, urlattr.Category) {
			return
		}
		diags = append(diags, Diagnostic{
			Filename: filename,
			Line:     line,
			Col:      col,
			Category: urlattr.Category,
			Message:  msg,
		})
	}

	ast.Inspect(f, func(n ast.Node) bool {
		switch a := n.(type) {
		case *ast.StaticAttr:
			if urlattr.IsURLAttr(a.Name) && urlattr.IsInternalPath(a.Value) {
				pos := staticValuePos(fset, src, a)
				emit(pos.Line, pos.Column, urlattr.LiteralMessage(a.Name, a.Value))
			}
		case *ast.ExprAttr:
			if !urlattr.IsURLAttr(a.Name) {
				return true
			}
			// Expr is the pipeline seed; a `|> url` stage replaces it
			// with a URLFor result, so only a hand-built seed (a string
			// literal, concatenation or fmt.Sprint*) is flagged.
			base := fset.Position(a.ExprPos)
			for _, fd := range urlattr.CheckGoExpr(a.Name, a.Expr) {
				line, col := snippetLineCol(base, fd.Pos)
				emit(line, col, fd.Message)
			}
		case *ast.EmbeddedAttr:
			if !urlattr.IsURLAttr(a.Name) || a.Lang != ast.EmbeddedText || len(a.Segments) == 0 {
				return true
			}
			// f`/items/@{id}`: the URL's path is whatever literal text
			// leads the template.
			lead, ok := a.Segments[0].(*ast.Text)
			if !ok || !urlattr.IsInternalPath(lead.Value) {
				return true
			}
			// The message quotes the value as authored: everything
			// after the `=` (attribute names cannot contain one).
			attrSrc := string(src[fset.Position(a.Pos()).Offset:fset.Position(a.End()).Offset])
			_, value, _ := strings.Cut(attrSrc, "=")
			pos := fset.Position(lead.Pos())
			emit(pos.Line, pos.Column, urlattr.InterpolationMessage(a.Name, value))
		}
		return true
	})
	return diags, nil
}

// Directives parses filename and returns its structpages:lint:ignore
// directives. The analyzer uses it to honour directives written in a
// .gsx file for diagnostics reported there through the generated
// .x.go's //line directives: gsx comments are not carried into the
// generated Go, so the analyzer can't see them in its own syntax trees.
func Directives(filename string) (*urlattr.Directives, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		return nil, err
	}
	return directives(fset, f), nil
}

// directives collects the suppression directives in f: source-only
// comments between elements (bare `// …` or braced `{/* … */}`),
// comments inside an attribute list, and HTML comments.
func directives(fset *token.FileSet, f *ast.File) *urlattr.Directives {
	ds := urlattr.NewDirectives()
	ast.Inspect(f, func(n ast.Node) bool {
		var text string
		switch c := n.(type) {
		case *ast.Comment:
			text = c.Text
		case *ast.CommentAttr:
			text = c.Text
		case *ast.HTMLComment:
			text = c.Text
		default:
			return true
		}
		if cats, ok := urlattr.ParseDirective(text); ok {
			ds.Add(fset.Position(n.Pos()).Line, cats)
		}
		return true
	})
	return ds
}

// staticValuePos returns the position of the first character of a
// static attribute's value — just past its opening quote — so the
// column points at the URL rather than the attribute name. The value
// is located in the source because StaticAttr only records the span
// of the whole `name="value"`.
func staticValuePos(fset *token.FileSet, src []byte, a *ast.StaticAttr) token.Position {
	start := fset.Position(a.Pos())
	end := fset.Position(a.End())
	if start.Offset < 0 || end.Offset > len(src) || start.Offset >= end.Offset {
		return start
	}
	if q := strings.IndexAny(string(src[start.Offset:end.Offset]), `"'`); q >= 0 {
		return fset.Position(a.Pos() + token.Pos(q+1))
	}
	return start
}

// snippetLineCol maps a position inside a Go-expression snippet
// (1-indexed, line 1 being the snippet's first line) back to a .gsx
// line and column, given base, the source position of the snippet's
// first character.
func snippetLineCol(base, p token.Position) (line, col int) {
	line = base.Line + p.Line - 1
	if p.Line == 1 {
		return line, base.Column + p.Column - 1
	}
	return line, p.Column
}
