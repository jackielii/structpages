package templscan

import (
	"strings"

	parser "github.com/a-h/templ/parser/v2"
)

const directivePrefix = "structpages:lint:ignore"

// directiveSet tracks which lines have which categories suppressed.
// The empty-string key means "all categories" (a bare directive).
// Line numbers are 1-indexed, matching Diagnostic.Line.
type directiveSet struct {
	byLine map[int]map[string]bool
}

func newDirectiveSet() *directiveSet {
	return &directiveSet{byLine: map[int]map[string]bool{}}
}

// add registers a directive sitting on srcLine. The directive
// applies to its own line and the line immediately following — so
// placing a comment above an element works the same as placing it
// inline.
func (ds *directiveSet) add(srcLine int, cats []string) {
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

// suppressed reports whether (line, category) is suppressed.
func (ds *directiveSet) suppressed(line int, category string) bool {
	set := ds.byLine[line]
	if set == nil {
		return false
	}
	return set[""] || set[category]
}

// parseDirective inspects an HTML-comment body and returns the
// listed categories. ok=false when the comment is not a directive.
// An empty category list (bare `structpages:lint:ignore`) means
// "suppress every category".
func parseDirective(commentBody string) (cats []string, ok bool) {
	s := strings.TrimSpace(commentBody)
	if !strings.HasPrefix(s, directivePrefix) {
		return nil, false
	}
	rest := strings.TrimSpace(s[len(directivePrefix):])
	if rest == "" {
		return nil, true
	}
	for _, c := range strings.Split(rest, ",") {
		c = strings.TrimSpace(c)
		if c != "" {
			cats = append(cats, c)
		}
	}
	return cats, true
}

// commentLine returns the 1-indexed source line of n.
// (parser.Range positions are 0-indexed.)
func commentLine(n *parser.HTMLComment) int {
	return int(n.Range.From.Line) + 1
}
