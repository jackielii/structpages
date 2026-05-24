// Package templscan walks .templ source files and emits diagnostics
// for URL-bearing HTML attributes whose values look like they should
// have used structpages.URLFor.
package templscan

import (
	"fmt"
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
	emit := func(d Diagnostic) {
		diags = append(diags, d)
	}

	v := visitor.New()
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

	if err := v.VisitTemplateFile(tf); err != nil {
		return diags, fmt.Errorf("walk %s: %w", filename, err)
	}
	return diags, nil
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
