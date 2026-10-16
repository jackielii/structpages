package templscan

import (
	"errors"

	parser "github.com/a-h/templ/parser/v2"
	"github.com/a-h/templ/parser/v2/visitor"
	"github.com/jackielii/structpages/tools/lint/internal/htmxattr"
)

// errFound stops the visitor at the first htmx use.
var errFound = errors.New("htmx markup found")

// HTMX parses filename and returns the first piece of htmx markup in
// it: an attribute htmxattr.IsAttr recognises, or an htmx element
// (htmxattr.IsElement). ok is false when the file has none. A file
// that fails to parse returns the parse error.
//
// Only constant attribute names written in the template count; an
// attribute whose name is an expression, or that reaches an element
// through a spread ({ attrs... }), is a runtime value.
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
	if p.err != nil {
		return htmxattr.Use{}, false, p.err
	}
	tf := p.tf
	found := func(name string, r parser.Range) error {
		use = htmxattr.Use{
			Filename: filename,
			Line:     int(r.From.Line) + 1,
			Col:      int(r.From.Col) + 1,
			Name:     name,
		}
		return errFound
	}
	checkKey := func(k parser.AttributeKey) error {
		if key, isConst := k.(parser.ConstantAttributeKey); isConst && htmxattr.IsAttr(key.Name) {
			return found(key.Name, key.NameRange)
		}
		return nil
	}

	v := visitor.New()
	visitElement := v.Element
	v.Element = func(n *parser.Element) error {
		if htmxattr.IsElement(n.Name) {
			return found(n.Name, n.NameRange)
		}
		return visitElement(n)
	}
	v.ConstantAttribute = func(n *parser.ConstantAttribute) error { return checkKey(n.Key) }
	v.BoolConstantAttribute = func(n *parser.BoolConstantAttribute) error { return checkKey(n.Key) }
	v.ExpressionAttribute = func(n *parser.ExpressionAttribute) error { return checkKey(n.Key) }
	v.BoolExpressionAttribute = func(n *parser.BoolExpressionAttribute) error { return checkKey(n.Key) }

	switch err := v.VisitTemplateFile(tf); {
	case errors.Is(err, errFound):
		return use, true, nil
	case err != nil:
		return htmxattr.Use{}, false, err
	}
	return htmxattr.Use{}, false, nil
}
