package lint

import (
	"go/ast"
	"go/token"
	"strings"
)

// directiveMap maps a token.Pos (line of a call expression) to the
// set of categories suppressed on that line. An entry with the empty
// string key means "all categories suppressed".
type directiveMap struct {
	fset *token.FileSet
	// byLine indexes suppressions by (file token.Pos, line number).
	byLine map[token.Pos]map[int]map[string]bool
}

// newDirectiveMap scans every comment in every file in files and
// records `//structpages:lint:ignore [cat,...]` directives. A
// directive applies to the line it sits on AND the following line, so
// a comment placed above a call also suppresses it.
func newDirectiveMap(fset *token.FileSet, files []*ast.File) *directiveMap {
	dm := &directiveMap{
		fset:   fset,
		byLine: map[token.Pos]map[int]map[string]bool{},
	}
	for _, f := range files {
		fileBase := f.Package
		if _, ok := dm.byLine[fileBase]; !ok {
			dm.byLine[fileBase] = map[int]map[string]bool{}
		}
		for _, cg := range f.Comments {
			for _, c := range cg.List {
				cats, ok := parseDirective(c.Text)
				if !ok {
					continue
				}
				line := fset.Position(c.Slash).Line
				// suppress on this line AND the next.
				dm.suppress(fileBase, line, cats)
				dm.suppress(fileBase, line+1, cats)
			}
		}
	}
	return dm
}

func (dm *directiveMap) suppress(fileBase token.Pos, line int, cats map[string]bool) {
	cur, ok := dm.byLine[fileBase][line]
	if !ok {
		cur = map[string]bool{}
		dm.byLine[fileBase][line] = cur
	}
	for k := range cats {
		cur[k] = true
	}
}

// suppressed reports whether the diagnostic at pos with the given
// category is suppressed by any directive.
func (dm *directiveMap) suppressed(pos token.Pos, category string) bool {
	if dm == nil || dm.fset == nil {
		return false
	}
	p := dm.fset.Position(pos)
	if !p.IsValid() {
		return false
	}
	// find the file base by matching the file name; token.Pos is
	// monotonic per FileSet, so we walk byLine entries and pick the
	// one whose file matches.
	for base, lines := range dm.byLine {
		basePos := dm.fset.Position(base)
		if basePos.Filename != p.Filename {
			continue
		}
		cats, ok := lines[p.Line]
		if !ok {
			return false
		}
		if cats[""] {
			return true
		}
		return cats[category]
	}
	return false
}

// parseDirective parses a single comment text and returns the set of
// categories it suppresses. The empty-string key represents "all".
func parseDirective(text string) (map[string]bool, bool) {
	// Strip leading //, /*, trailing */ and whitespace.
	t := text
	switch {
	case strings.HasPrefix(t, "//"):
		t = t[2:]
	case strings.HasPrefix(t, "/*"):
		t = strings.TrimSuffix(t[2:], "*/")
	default:
		return nil, false
	}
	t = strings.TrimSpace(t)
	const prefix = "structpages:lint:ignore"
	if !strings.HasPrefix(t, prefix) {
		return nil, false
	}
	rest := strings.TrimSpace(t[len(prefix):])
	cats := map[string]bool{}
	if rest == "" {
		cats[""] = true
		return cats, true
	}
	for _, part := range strings.Split(rest, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		cats[p] = true
	}
	if len(cats) == 0 {
		cats[""] = true
	}
	return cats, true
}
