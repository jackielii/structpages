// Package templscan walks .templ source files and emits diagnostics
// for URL-bearing HTML attributes whose values look like they should
// have used structpages.URLFor.
package templscan

import (
	"fmt"
	"go/token"
	"strings"

	parser "github.com/a-h/templ/parser/v2"
	"github.com/a-h/templ/parser/v2/visitor"
)

// Diagnostic is a single finding to report. Line/Col are 1-indexed
// positions in the .templ source file.
type Diagnostic struct {
	Filename string
	Line     int
	Col      int
	Category string
	Message  string
}

// SuppressLookup lets callers seed extra suppressions from outside
// the file (e.g. a project-wide ignore list). Currently unused;
// kept in the signature so we can wire it later without breaking
// callers.
type SuppressLookup func(filename string, line int, category string) bool

// Scan parses filename and returns diagnostics. A parse error in
// the templ file yields a single diagnostic and a nil error — Scan
// never returns a non-nil error for parse problems, only for I/O.
func Scan(filename string, _ SuppressLookup) ([]Diagnostic, error) {
	tf, err := parser.Parse(filename)
	if err != nil {
		return []Diagnostic{{
			Filename: filename,
			Line:     1,
			Col:      1,
			Category: categoryURLAttr,
			Message:  "[" + categoryURLAttr + "] templ parse error: " + err.Error(),
		}}, nil
	}

	var diags []Diagnostic
	ds := newDirectiveSet()
	emit := func(d Diagnostic) {
		if ds.suppressed(d.Line, d.Category) {
			return
		}
		diags = append(diags, d)
	}

	v := visitor.New()
	v.HTMLComment = func(n *parser.HTMLComment) error {
		cats, ok := parseDirective(n.Contents)
		if !ok {
			return nil
		}
		ds.add(commentLine(n), cats)
		return nil
	}
	v.GoComment = func(n *parser.GoComment) error {
		// Multi-line /* */ comments may span multiple source
		// lines; the directive only suppresses the line the
		// comment starts on plus the following line, matching
		// the // form's semantics. Authors who want to suppress
		// a block should put the directive on its own line.
		cats, ok := parseDirective(n.Contents)
		if !ok {
			return nil
		}
		ds.add(goCommentLine(n), cats)
		return nil
	}
	v.ConstantAttribute = func(n *parser.ConstantAttribute) error {
		key, ok := n.Key.(parser.ConstantAttributeKey)
		if !ok {
			return nil
		}
		if !urlAttrs[strings.ToLower(key.Name)] {
			return nil
		}
		if isInternalPath(n.Value) {
			emit(Diagnostic{
				Filename: filename,
				Line:     int(n.ValueRange.From.Line) + 1,
				Col:      int(n.ValueRange.From.Col) + 1,
				Category: categoryURLAttr,
				Message:  literalDiagnostic(key.Name, n.Value),
			})
		}
		return nil
	}
	v.ExpressionAttribute = func(n *parser.ExpressionAttribute) error {
		key, ok := n.Key.(parser.ConstantAttributeKey)
		if !ok {
			return nil
		}
		if !urlAttrs[strings.ToLower(key.Name)] {
			return nil
		}
		for _, f := range checkGoExpr(key.Name, n.Expression.Value) {
			emit(Diagnostic{
				Filename: filename,
				Line:     mapLine(n.Expression.Range, f.pos),
				Col:      mapCol(n.Expression.Range, f.pos),
				Category: categoryURLAttr,
				Message:  f.message,
			})
		}
		return nil
	}

	if err := v.VisitTemplateFile(tf); err != nil {
		return diags, fmt.Errorf("walk %s: %w", filename, err)
	}
	return diags, nil
}

// mapLine maps a 1-indexed snippet line back to the 1-indexed
// .templ source line. Snippet line 1 sits on the templ line that
// contains the `{` of the expression block (templ Range is
// 0-indexed; we convert to 1-indexed).
func mapLine(r parser.Range, p token.Position) int {
	return int(r.From.Line) + p.Line
}

// mapCol maps a snippet column back to the .templ source column.
// On the snippet's first line the snippet text starts a fixed
// distance past the `{` in the templ source — we offset by the
// templ column. On subsequent snippet lines the snippet column is
// the templ column (no offset).
func mapCol(r parser.Range, p token.Position) int {
	if p.Line == 1 {
		return int(r.From.Col) + p.Column
	}
	return p.Column
}

const categoryURLAttr = "url-attr"

// urlAttrs is the set of HTML attribute names whose values should
// flow through structpages.URLFor rather than being hand-built.
// Compare names case-insensitively.
var urlAttrs = map[string]bool{
	"href":           true,
	"action":         true,
	"formaction":     true,
	"hx-get":         true,
	"hx-post":        true,
	"hx-put":         true,
	"hx-patch":       true,
	"hx-delete":      true,
	"hx-push-url":    true,
	"hx-replace-url": true,
}
