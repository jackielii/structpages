package templscan

import (
	parser "github.com/a-h/templ/parser/v2"
)

// commentLine returns the 1-indexed source line of n.
// (parser.Range positions are 0-indexed.)
func commentLine(n *parser.HTMLComment) int {
	return int(n.Range.From.Line) + 1
}

// goCommentLine returns the 1-indexed source line of a Go-style
// // comment in templ. Multiline /* */ comments would need a
// different mapping; we only handle single-line here.
func goCommentLine(n *parser.GoComment) int {
	return int(n.Range.From.Line) + 1
}
