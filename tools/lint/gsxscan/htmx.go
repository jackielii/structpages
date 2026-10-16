package gsxscan

import (
	"go/token"

	"github.com/gsxhq/gsx/ast"
	"github.com/jackielii/structpages/tools/lint/internal/htmxattr"
)

// HTMX parses filename and returns the first piece of htmx markup in
// it: an attribute htmxattr.IsAttr recognises, or an htmx element
// (htmxattr.IsElement). ok is false when the file has none. A file
// that fails to parse returns the parse error.
//
// Only names written in the template count; an attribute that reaches
// an element through a spread (`{ attrs... }`) is a runtime value.
func HTMX(filename string) (use htmxattr.Use, ok bool, err error) {
	type found struct {
		use htmxattr.Use
		ok  bool
	}
	r := memo(filename, "htmx", func() result[found] {
		u, ok, err := htmx(filename)
		return result[found]{found{u, ok}, err}
	})
	return r.val.use, r.val.ok, r.err
}

func htmx(filename string) (use htmxattr.Use, ok bool, err error) {
	p := parse(filename)
	if err := firstErr(p.readErr, p.err); err != nil {
		return htmxattr.Use{}, false, err
	}
	fset, f := p.fset, p.file
	found := func(name string, pos token.Pos) {
		p := fset.Position(pos)
		use, ok = htmxattr.Use{Filename: filename, Line: p.Line, Col: p.Column, Name: name}, true
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if ok {
			return false
		}
		switch a := n.(type) {
		case *ast.Element:
			if htmxattr.IsElement(a.Tag) {
				found(a.Tag, a.TagPos)
			}
		case *ast.StaticAttr:
			if htmxattr.IsAttr(a.Name) {
				found(a.Name, a.Pos())
			}
		case *ast.ExprAttr:
			if htmxattr.IsAttr(a.Name) {
				found(a.Name, a.Pos())
			}
		case *ast.BoolAttr:
			if htmxattr.IsAttr(a.Name) {
				found(a.Name, a.Pos())
			}
		case *ast.EmbeddedAttr:
			if htmxattr.IsAttr(a.Name) {
				found(a.Name, a.Pos())
			}
		case *ast.MarkupAttr:
			if htmxattr.IsAttr(a.Name) {
				found(a.Name, a.Pos())
			}
		case *ast.OrderedAttrsAttr:
			// An attribute bag literal ({{ "hx-get": u }}) names the
			// attributes it renders by its keys.
			for i := range a.Pairs {
				if htmxattr.IsAttr(a.Pairs[i].Key) { // a pair records only its value's position
					found(a.Pairs[i].Key, a.Pairs[i].Pos())
					break
				}
			}
		}
		return !ok
	})
	return use, ok, nil
}
