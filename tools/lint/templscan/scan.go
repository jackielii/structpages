// Package templscan walks .templ source files and emits diagnostics
// for URL-bearing HTML attributes whose values look like they should
// have used structpages.URLFor.
package templscan

import (
	"fmt"
	"go/token"

	parser "github.com/a-h/templ/parser/v2"
	"github.com/a-h/templ/parser/v2/visitor"
	"github.com/jackielii/structpages/tools/lint/internal/urlattr"
)

// Diagnostic is a single finding to report. Line/Col are 1-indexed
// positions in the .templ source file.
type Diagnostic = urlattr.Diagnostic

// SuppressLookup lets callers seed extra suppressions from outside
// the file (e.g. a project-wide ignore list). Currently unused;
// kept in the signature so we can wire it later without breaking
// callers.
type SuppressLookup func(filename string, line int, category string) bool

// Scan parses filename and returns diagnostics. A parse error in
// the templ file yields a single diagnostic and a nil error — Scan
// never returns a non-nil error for parse problems, only for I/O.
func Scan(filename string, _ SuppressLookup) ([]Diagnostic, error) {
	r := memo(filename, "scan", func() result[[]Diagnostic] {
		d, err := scan(filename)
		return result[[]Diagnostic]{d, err}
	})
	return r.val, r.err
}

func scan(filename string) ([]Diagnostic, error) {
	p := parse(filename)
	tf, err := p.tf, p.err
	if err != nil {
		return []Diagnostic{{
			Filename: filename,
			Line:     1,
			Col:      1,
			Category: urlattr.Category,
			Message:  "[" + urlattr.Category + "] templ parse error: " + err.Error(),
		}}, nil
	}

	var diags []Diagnostic
	ds := urlattr.NewDirectives()
	emit := func(d Diagnostic) {
		if ds.Suppressed(d.Line, d.Category) {
			return
		}
		diags = append(diags, d)
	}

	v := visitor.New()
	collectDirectives(v, ds)
	v.ConstantAttribute = func(n *parser.ConstantAttribute) error {
		key, ok := n.Key.(parser.ConstantAttributeKey)
		if !ok {
			return nil
		}
		if !urlattr.IsURLAttr(key.Name) {
			return nil
		}
		if urlattr.IsInternalPath(n.Value) {
			emit(Diagnostic{
				Filename: filename,
				Line:     int(n.ValueRange.From.Line) + 1,
				Col:      int(n.ValueRange.From.Col) + 1,
				Category: urlattr.Category,
				Message:  urlattr.LiteralMessage(key.Name, n.Value),
			})
		}
		return nil
	}
	v.ExpressionAttribute = func(n *parser.ExpressionAttribute) error {
		key, ok := n.Key.(parser.ConstantAttributeKey)
		if !ok {
			return nil
		}
		if !urlattr.IsURLAttr(key.Name) {
			return nil
		}
		for _, f := range urlattr.CheckGoExpr(key.Name, n.Expression.Value) {
			emit(Diagnostic{
				Filename: filename,
				Line:     mapLine(n.Expression.Range, f.Pos),
				Col:      mapCol(n.Expression.Range, f.Pos),
				Category: urlattr.Category,
				Message:  f.Message,
			})
		}
		return nil
	}

	if err := v.VisitTemplateFile(tf); err != nil {
		return diags, fmt.Errorf("walk %s: %w", filename, err)
	}
	return diags, nil
}

// Directives parses filename and returns its structpages:lint:ignore
// directives. The analyzer uses it to honour directives written in a
// .templ file for [region] diagnostics it reports there: templ comments
// are not carried into the generated Go.
func Directives(filename string) (*urlattr.Directives, error) {
	r := memo(filename, "directives", func() result[*urlattr.Directives] {
		p := parse(filename)
		if p.err != nil {
			return result[*urlattr.Directives]{err: p.err}
		}
		ds := urlattr.NewDirectives()
		v := visitor.New()
		collectDirectives(v, ds)
		if err := v.VisitTemplateFile(p.tf); err != nil {
			return result[*urlattr.Directives]{err: err}
		}
		return result[*urlattr.Directives]{val: ds}
	})
	return r.val, r.err
}

// collectDirectives makes v record the directives in HTML comments and
// Go-style comments into ds.
func collectDirectives(v *visitor.Visitor, ds *urlattr.Directives) {
	v.HTMLComment = func(n *parser.HTMLComment) error {
		if cats, ok := urlattr.ParseDirective(n.Contents); ok {
			ds.Add(commentLine(n), cats)
		}
		return nil
	}
	v.GoComment = func(n *parser.GoComment) error {
		// Multi-line /* */ comments may span multiple source
		// lines; the directive only suppresses the line the
		// comment starts on plus the following line, matching
		// the // form's semantics. Authors who want to suppress
		// a block should put the directive on its own line.
		if cats, ok := urlattr.ParseDirective(n.Contents); ok {
			ds.Add(goCommentLine(n), cats)
		}
		return nil
	}
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
