package gsxscan

import (
	"go/token"

	"github.com/gsxhq/gsx/ast"
	"github.com/jackielii/structpages/tools/lint/internal/attrsite"
	"github.com/jackielii/structpages/tools/lint/internal/htmxattr"
)

// ExprAttrs parses filename and indexes its expression attributes by
// the site gsx's generated .x.go positions the call producing the
// value at. gsx writes a /*line*/ directive before every call it
// lowers a pipeline stage to, naming that stage's name, and before a
// seed, naming its first character; the value's call is the last
// stage's when there is one (its result is the attribute value), the
// seed otherwise. Attributes inside an in-tag if/switch belong to
// their element.
func ExprAttrs(filename string) (map[attrsite.Site]attrsite.Attr, error) {
	r := memo(filename, "exprattrs", func() result[map[attrsite.Site]attrsite.Attr] {
		p := parse(filename)
		if err := firstErr(p.readErr, p.err); err != nil {
			return result[map[attrsite.Site]attrsite.Attr]{err: err}
		}
		return result[map[attrsite.Site]attrsite.Attr]{val: exprAttrs(p.fset, p.file)}
	})
	return r.val, r.err
}

func exprAttrs(fset *token.FileSet, f *ast.File) map[attrsite.Site]attrsite.Attr {
	out := map[attrsite.Site]attrsite.Attr{}
	site := func(pos token.Pos) attrsite.Site {
		p := fset.Position(pos)
		return attrsite.Site{Line: p.Line, Col: p.Column}
	}
	// valueSite returns where the call producing an ExprAttr's value
	// is positioned.
	valueSite := func(a *ast.ExprAttr) attrsite.Site {
		if n := len(a.Stages); n > 0 {
			return site(a.Stages[n-1].NamePos)
		}
		return site(a.ExprPos)
	}
	ast.Inspect(f, func(n ast.Node) bool {
		el, ok := n.(*ast.Element)
		if !ok {
			return true
		}
		attrs := flatten(el.Attrs)
		var requests []attrsite.Request
		spread := false
		for _, a := range attrs {
			switch a := a.(type) {
			case *ast.SpreadAttr:
				spread = true
			case *ast.ExprAttr:
				if htmxattr.IsRequest(a.Name) {
					requests = append(requests, attrsite.Request{
						Name: a.Name, Expr: true, Site: valueSite(a),
						Staged: len(a.Stages) > 0, Stages: len(a.Stages), Source: a.Expr,
					})
				}
			case *ast.StaticAttr:
				if htmxattr.IsRequest(a.Name) {
					requests = append(requests, attrsite.Request{Name: a.Name})
				}
			case *ast.BoolAttr:
				if htmxattr.IsRequest(a.Name) {
					requests = append(requests, attrsite.Request{Name: a.Name})
				}
			case *ast.EmbeddedAttr:
				if htmxattr.IsRequest(a.Name) {
					requests = append(requests, attrsite.Request{Name: a.Name})
				}
			case *ast.MarkupAttr:
				if htmxattr.IsRequest(a.Name) {
					requests = append(requests, attrsite.Request{Name: a.Name})
				}
			}
		}
		for _, a := range attrs {
			if a, ok := a.(*ast.ExprAttr); ok {
				out[valueSite(a)] = attrsite.Attr{
					Name: a.Name, Tag: el.Tag, Component: ast.IsComponentTag(el.Tag),
					Expr: a.Expr, Staged: len(a.Stages) > 0, Stages: len(a.Stages),
					Src: site(a.ExprPos), Requests: requests, Spread: spread,
				}
			}
		}
		return true
	})
	return out
}

// flatten returns attrs with the branches of in-tag if/switch groups
// spliced in: every attribute the element may carry.
func flatten(attrs []ast.Attr) []ast.Attr {
	var out []ast.Attr
	for _, a := range attrs {
		switch a := a.(type) {
		case *ast.CondAttr:
			out = append(out, flatten(a.Then)...)
			out = append(out, flatten(a.Else)...)
		case *ast.SwitchAttr:
			for _, c := range a.Cases {
				out = append(out, flatten(c.Body)...)
			}
		default:
			out = append(out, a)
		}
	}
	return out
}
