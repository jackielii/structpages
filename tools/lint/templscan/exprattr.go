package templscan

import (
	"bytes"
	"errors"
	"go/format"
	"go/scanner"
	"go/token"
	"os"
	"strings"

	"github.com/a-h/templ"
	"github.com/a-h/templ/generator"
	parser "github.com/a-h/templ/parser/v2"
	"github.com/a-h/templ/parser/v2/visitor"
	"github.com/jackielii/structpages/tools/lint/internal/attrsite"
	"github.com/jackielii/structpages/tools/lint/internal/htmxattr"
)

// ErrStale reports that a .templ file's _templ.go isn't what the templ
// generator the linter is built with produces for it, so the generator's
// source map doesn't describe the file on disk.
var ErrStale = errors.New("_templ.go doesn't match this templ version's output; regenerate it with templ " +
	templ.Version())

// versionPrefix starts the comment line in which templ records the
// version that generated a file.
const versionPrefix = "// templ: version: "

// ExprAttrs indexes the expression attributes of the .templ file
// filename (attr={ expr }) by the position of their expression in the
// generated filename's _templ.go, where the call producing the value
// starts.
//
// templ writes no position mapping into the generated file, so the
// index is built from templ's own source map: the file is generated
// in-process with the templ version the linter is built with and
// formatted the way `templ generate` formats it. The map is used only
// if the result is the _templ.go on disk, with one difference allowed:
// the "// templ: version:" comment line, present or not
// (-include-version=false omits it). That line is a comment of its own,
// so it changes no token; positions are read from the file on disk, so
// the lines it shifts don't matter. Any other difference means the file
// was generated differently and its positions can't be trusted, and
// ExprAttrs returns ErrStale.
//
// The source map gives positions in the generator's unformatted output.
// Formatting changes only the layout between tokens, so the n-th token
// of the unformatted output is the n-th token of the file on disk.
func ExprAttrs(filename string) (map[attrsite.Site]attrsite.Attr, error) {
	r := memo(filename, "exprattrs", func() result[map[attrsite.Site]attrsite.Attr] {
		attrs, err := exprAttrs(filename)
		return result[map[attrsite.Site]attrsite.Attr]{attrs, err}
	})
	return r.val, r.err
}

func exprAttrs(filename string) (map[attrsite.Site]attrsite.Attr, error) {
	p := parse(filename)
	if p.err != nil {
		return nil, p.err
	}
	tf := p.tf
	disk, err := os.ReadFile(strings.TrimSuffix(filename, ".templ") + "_templ.go")
	if err != nil {
		return nil, err
	}
	// The file name templ reports in runtime errors is the path
	// `templ generate` was given it by; reuse the one on disk. The
	// comparison below decides whether the choice was right.
	name := tf.Filepath
	if _, rest, ok := strings.Cut(string(disk), "FileName: `"); ok {
		name, _, _ = strings.Cut(rest, "`")
	}
	var raw bytes.Buffer
	out, err := generator.Generate(tf, &raw, generator.WithVersion(templ.Version()), generator.WithFileName(name))
	if err != nil {
		return nil, err
	}
	formatted, err := format.Source(raw.Bytes())
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(withoutVersion(formatted), withoutVersion(disk)) {
		return nil, ErrStale
	}
	rawTokens, diskTokens := tokenStarts(raw.Bytes()), tokenStarts(disk)
	if len(rawTokens) != len(diskTokens) {
		return nil, ErrStale
	}
	tokenAt := make(map[int]int, len(rawTokens)) // raw offset -> token index
	for i, t := range rawTokens {
		tokenAt[t.Offset] = i
	}
	// valueSite returns the position in the _templ.go of the start of
	// the generated code for expr: where a call that is all of expr
	// starts.
	valueSite := func(expr parser.Expression) (attrsite.Site, bool) {
		from := expr.Range.From
		tgt, ok := out.SourceMap.TargetPositionFromSource(from.Line, from.Col)
		if !ok {
			return attrsite.Site{}, false
		}
		i, ok := tokenAt[int(tgt.Index)]
		if !ok {
			return attrsite.Site{}, false
		}
		return attrsite.Site{Line: diskTokens[i].Line, Col: diskTokens[i].Column}, true
	}

	attrs := map[attrsite.Site]attrsite.Attr{}
	v := visitor.New()
	visitElement := v.Element
	v.Element = func(el *parser.Element) error {
		all := flatten(el.Attributes)
		var requests []attrsite.Request
		spread := false
		for _, a := range all {
			switch a := a.(type) {
			case *parser.SpreadAttributes:
				spread = true
			case *parser.ExpressionAttribute:
				if key, ok := a.Key.(parser.ConstantAttributeKey); ok && htmxattr.IsRequest(key.Name) {
					req := attrsite.Request{Name: key.Name, Source: a.Expression.Value}
					// An expression with no generated position can't be
					// joined to its call: treat it as unknown, like a
					// literal.
					req.Site, req.Expr = valueSite(a.Expression)
					requests = append(requests, req)
				}
			case *parser.ConstantAttribute:
				if key, ok := a.Key.(parser.ConstantAttributeKey); ok && htmxattr.IsRequest(key.Name) {
					requests = append(requests, attrsite.Request{Name: key.Name})
				}
			case *parser.BoolConstantAttribute:
				if key, ok := a.Key.(parser.ConstantAttributeKey); ok && htmxattr.IsRequest(key.Name) {
					requests = append(requests, attrsite.Request{Name: key.Name})
				}
			case *parser.BoolExpressionAttribute:
				if key, ok := a.Key.(parser.ConstantAttributeKey); ok && htmxattr.IsRequest(key.Name) {
					requests = append(requests, attrsite.Request{Name: key.Name})
				}
			}
		}
		for _, a := range all {
			ea, ok := a.(*parser.ExpressionAttribute)
			if !ok {
				continue
			}
			key, ok := ea.Key.(parser.ConstantAttributeKey)
			if !ok {
				continue
			}
			site, ok := valueSite(ea.Expression)
			if !ok {
				continue
			}
			from := ea.Expression.Range.From
			attrs[site] = attrsite.Attr{
				Name:     key.Name,
				Tag:      el.Name,
				Expr:     ea.Expression.Value,
				Src:      attrsite.Site{Line: int(from.Line) + 1, Col: int(from.Col) + 1},
				Requests: requests,
				Spread:   spread,
			}
		}
		return visitElement(el)
	}
	if err := v.VisitTemplateFile(tf); err != nil {
		return nil, err
	}
	return attrs, nil
}

// flatten returns attrs with the branches of conditional attributes
// spliced in: every attribute the element may carry.
func flatten(attrs []parser.Attribute) []parser.Attribute {
	var out []parser.Attribute
	for _, a := range attrs {
		if c, ok := a.(*parser.ConditionalAttribute); ok {
			out = append(out, flatten(c.Then)...)
			out = append(out, flatten(c.Else)...)
			continue
		}
		out = append(out, a)
	}
	return out
}

// withoutVersion removes src's "// templ: version:" line, newline
// included, if it has one: `templ generate -include-version=false`
// writes none.
func withoutVersion(src []byte) []byte {
	i := bytes.Index(src, []byte(versionPrefix))
	if i < 0 || (i > 0 && src[i-1] != '\n') {
		return src
	}
	end := bytes.IndexByte(src[i:], '\n')
	if end < 0 {
		return src[:i]
	}
	out := make([]byte, 0, len(src))
	out = append(out, src[:i]...)
	return append(out, src[i+end+1:]...)
}

// tokenStarts returns the position of every token of the Go source src,
// in order. Comments aren't tokens, and neither is any semicolon: gofmt
// replaces an explicit one ending a statement with a newline
// ({{ a := 1; b := 2 }} in templ) and moves the newlines that insert
// the others, while every other token survives formatting unchanged.
func tokenStarts(src []byte) []token.Position {
	fset := token.NewFileSet()
	file := fset.AddFile("", -1, len(src))
	var s scanner.Scanner
	s.Init(file, src, nil, 0)
	var out []token.Position
	for {
		pos, tok, _ := s.Scan()
		if tok == token.EOF {
			return out
		}
		if tok == token.SEMICOLON {
			continue
		}
		out = append(out, fset.Position(pos))
	}
}
