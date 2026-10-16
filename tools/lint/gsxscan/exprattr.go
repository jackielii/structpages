package gsxscan

import (
	"go/token"

	"github.com/gsxhq/gsx/ast"
	"github.com/gsxhq/gsx/parser"
)

// ExprAttr is an element attribute whose value is a Go expression,
// name={seed |> stage …}, located the way gsx's generated .x.go
// positions the call that produces the value: gsx writes a /*line*/
// directive before every lowered pipeline stage and before the seed, so
// a call in the .x.go maps back to exactly one of these sites.
type ExprAttr struct {
	// Name is the attribute name as authored ("hx-target:inherited").
	Name string
	// Tag is the tag of the element carrying the attribute.
	Tag string
	// Component reports that Tag is a component tag (ast.IsComponentTag):
	// its attributes are props, not HTML attributes of an element.
	Component bool
	// Staged reports that the value is the result of a pipeline stage
	// (`|> id`); the site is then that last stage's name. Otherwise the
	// value is the seed itself and the site is the seed's first char.
	Staged bool
	// Expr is the seed expression source, as authored.
	Expr string
}

// ExprSite is a 1-indexed line and column in a .gsx file.
type ExprSite struct {
	Line, Col int
}

// ExprAttrs parses filename and indexes its expression attributes by
// the site the value's producing call is positioned at: the name of
// the last pipeline stage when there is one (the stage whose result is
// the attribute value), the seed's first character otherwise.
// Attributes inside an in-tag if/switch belong to their element.
func ExprAttrs(filename string) (map[ExprSite]ExprAttr, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		return nil, err
	}
	out := map[ExprSite]ExprAttr{}
	site := func(pos token.Pos) ExprSite {
		p := fset.Position(pos)
		return ExprSite{Line: p.Line, Col: p.Column}
	}
	var visit func(el *ast.Element, attrs []ast.Attr)
	visit = func(el *ast.Element, attrs []ast.Attr) {
		for _, a := range attrs {
			switch a := a.(type) {
			case *ast.ExprAttr:
				ea := ExprAttr{Name: a.Name, Tag: el.Tag, Component: ast.IsComponentTag(el.Tag), Expr: a.Expr}
				if n := len(a.Stages); n > 0 {
					ea.Staged = true
					out[site(a.Stages[n-1].NamePos)] = ea
				} else {
					out[site(a.ExprPos)] = ea
				}
			case *ast.CondAttr:
				visit(el, a.Then)
				visit(el, a.Else)
			case *ast.SwitchAttr:
				for _, c := range a.Cases {
					visit(el, c.Body)
				}
			}
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if el, ok := n.(*ast.Element); ok {
			visit(el, el.Attrs)
		}
		return true
	})
	return out, nil
}
