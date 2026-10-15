// Package urlattr holds the [url-attr] rule shared by the template
// scanners (templscan for .templ, gsxscan for .gsx): which HTML
// attributes carry URLs, which values count as hard-coded internal
// URLs, the diagnostic wording, and the structpages:lint:ignore
// suppression-directive semantics. Each scanner only maps its own
// template AST onto these.
package urlattr

import (
	"strings"
)

// Category is the [url-attr] diagnostic category.
const Category = "url-attr"

// Diagnostic is a single finding to report. Line/Col are 1-indexed
// positions in the template source file.
type Diagnostic struct {
	Filename string
	Line     int
	Col      int
	Category string
	Message  string
}

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

// IsURLAttr reports whether name is an attribute whose value should
// be produced by structpages.URLFor.
func IsURLAttr(name string) bool {
	return urlAttrs[strings.ToLower(name)]
}

const directivePrefix = "structpages:lint:ignore"

// Directives tracks which lines have which categories suppressed.
// The empty-string key means "all categories" (a bare directive).
// Line numbers are 1-indexed, matching Diagnostic.Line.
type Directives struct {
	byLine map[int]map[string]bool
}

// NewDirectives returns an empty directive set.
func NewDirectives() *Directives {
	return &Directives{byLine: map[int]map[string]bool{}}
}

// Add registers a directive sitting on srcLine. The directive
// applies to its own line and the line immediately following — so
// placing a comment above an element works the same as placing it
// inline.
func (ds *Directives) Add(srcLine int, cats []string) {
	if len(cats) == 0 {
		cats = []string{""}
	}
	for _, line := range []int{srcLine, srcLine + 1} {
		set := ds.byLine[line]
		if set == nil {
			set = map[string]bool{}
			ds.byLine[line] = set
		}
		for _, c := range cats {
			set[c] = true
		}
	}
}

// Suppressed reports whether (line, category) is suppressed. A nil
// set suppresses nothing.
func (ds *Directives) Suppressed(line int, category string) bool {
	if ds == nil {
		return false
	}
	set := ds.byLine[line]
	if set == nil {
		return false
	}
	return set[""] || set[category]
}

// ParseDirective inspects a comment body and returns the listed
// categories. ok=false when the comment is not a directive. An empty
// category list (bare `structpages:lint:ignore`) means "suppress every
// category".
func ParseDirective(commentBody string) (cats []string, ok bool) {
	s := strings.TrimSpace(commentBody)
	if !strings.HasPrefix(s, directivePrefix) {
		return nil, false
	}
	rest := strings.TrimSpace(s[len(directivePrefix):])
	if rest == "" {
		return nil, true
	}
	for c := range strings.SplitSeq(rest, ",") {
		c = strings.TrimSpace(c)
		if c != "" {
			cats = append(cats, c)
		}
	}
	return cats, true
}
