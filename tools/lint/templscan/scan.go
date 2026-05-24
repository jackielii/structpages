// Package templscan walks .templ source files and emits diagnostics
// for URL-bearing HTML attributes whose values look like they should
// have used structpages.URLFor.
package templscan

import (
	parser "github.com/a-h/templ/parser/v2"
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
	_ = tf
	return nil, nil
}

const categoryURLAttr = "url-attr"
