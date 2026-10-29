package gsxscan

import (
	"go/token"

	"github.com/gsxhq/gsx/ast"
	"github.com/jackielii/structpages/tools/lint/internal/attrsite"
)

// Values parses filename and returns the values of its expression
// attributes (attr={x}) and f`…` template attributes, each split into
// its literal text and Go expressions. An expression is located by the
// site gsx's generated .x.go positions the code producing it at, as in
// ExprAttrs. A template piped as a whole (attr={f`…` |> f}) isn't
// returned: the filter's result is the value, not the parts.
// Attributes inside an in-tag if/switch belong to their element.
func Values(filename string) ([]attrsite.Value, error) {
	r := memo(filename, "values", func() result[[]attrsite.Value] {
		p := parse(filename)
		if err := firstErr(p.readErr, p.err); err != nil {
			return result[[]attrsite.Value]{err: err}
		}
		return result[[]attrsite.Value]{val: values(p.fset, p.file)}
	})
	return r.val, r.err
}

func values(fset *token.FileSet, f *ast.File) []attrsite.Value {
	site := func(pos token.Pos) attrsite.Site {
		p := fset.Position(pos)
		return attrsite.Site{Line: p.Line, Col: p.Column}
	}
	// expr returns the part for a pipeline seed and its stages; the
	// value's code is the last stage's call when there is one, the
	// seed otherwise.
	expr := func(seed string, seedPos token.Pos, stages []ast.PipeStage) attrsite.Part {
		at := site(seedPos)
		if n := len(stages); n > 0 {
			at = site(stages[n-1].NamePos)
		}
		return attrsite.Part{
			Expr: true, Site: at, Staged: len(stages) > 0, Stages: len(stages),
			Source: seed, Src: site(seedPos),
		}
	}
	var out []attrsite.Value
	ast.Inspect(f, func(n ast.Node) bool {
		el, ok := n.(*ast.Element)
		if !ok {
			return true
		}
		for _, a := range flatten(el.Attrs) {
			v := attrsite.Value{Tag: el.Tag, Component: ast.IsComponentTag(el.Tag)}
			switch a := a.(type) {
			case *ast.ExprAttr:
				v.Name = a.Name
				v.Parts = []attrsite.Part{expr(a.Expr, a.ExprPos, a.Stages)}
			case *ast.EmbeddedAttr:
				if a.Lang != ast.EmbeddedText || len(a.Stages) > 0 {
					continue
				}
				v.Name = a.Name
				for _, s := range a.Segments {
					switch s := s.(type) {
					case *ast.Text:
						v.Parts = append(v.Parts, attrsite.Part{Text: s.Value})
					case *ast.Interp:
						v.Parts = append(v.Parts, expr(s.Expr, s.ExprPos, s.Stages))
					}
				}
			default:
				continue
			}
			out = append(out, v)
		}
		return true
	})
	return out
}
