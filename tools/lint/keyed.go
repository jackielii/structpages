package lint

import (
	"cmp"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"github.com/jackielii/structpages/tools/lint/gsxscan"
	"github.com/jackielii/structpages/tools/lint/internal/attrsite"
	"github.com/jackielii/structpages/tools/lint/internal/htmxattr"
	"github.com/jackielii/structpages/tools/lint/templscan"
)

// keyedCategory is the diagnostic category of an instance id built by
// hand from a generated id.
const keyedCategory = "keyed"

// checkKeyed flags an element id or hx-target that appends a runtime
// value to a generated id, the hand-built form of an instance id:
//
//	id=f`@{Page.Row |> id}-@{row.ID}`            → id={Page.Row |> key(row.ID)}
//	hx-target=f`@{Page.Row |> target}-@{row.ID}` → hx-target={Page.Row |> keytarget(row.ID)}
//	id={rowID + "-" + row.ID}, rowID from structpages.ID
//
// The result is no component's id, so the htmx target selectors can't
// match a request targeting it to the component, and Props can't read
// the key back with KeyOf. Key and KeyTarget generate the instance's id
// instead.
//
// The value is read from the template source, part by part (see
// gsxscan.Values; a .templ attribute is one part), and each expression
// part is joined exactly to the code generated for it, as for region:
// through gsx's /*line*/ directives for .gsx, through templ's source
// map for .templ. An expression is a generated id when it is a call of
// structpages.ID, IDTarget, Key or KeyTarget (the gsx id, target, key
// and keytarget filters), or a local variable assigned once, from such
// a call. A string concatenation is split into its operands, and a
// local variable assigned once from an expression stands for it.
//
// What is appended decides it:
//
//   - A compile-time constant, such as the -document of
//     f`@{Page.Form |> id}-document`, is the same in every render: the
//     id of a sub-element, not of an instance. It isn't flagged.
//   - Anything else (a variable, a field, a call) is a runtime value,
//     and the id differs per render: an instance id.
//
// In an id attribute the whole value is the id, so the generated id
// must start it and any runtime value after it counts. In an hx-target
// the value is a selector, and the generated #id one compound in it:
// what counts is a runtime value inside the id's token, after literal
// identifier text that extends it (f`@{P.Row |> target}-@{row.ID}`). A
// runtime value right after the generated target may start another
// selector, so it isn't flagged. ID and Key count in an hx-target only
// after a literal "#".
func checkKeyed(ctx *checkCtx) {
	for _, f := range ctx.pass.Files {
		tmpl, exprs := ctx.generatedExprs(f)
		if tmpl == "" {
			continue
		}
		values, err := templateValues(tmpl)
		if err != nil {
			// A template that doesn't parse failed generation first; a
			// stale _templ.go has no trustworthy positions.
			continue
		}
		for _, v := range values {
			ctx.checkKeyedValue(tmpl, v, exprs)
		}
	}
}

// templateValues returns the attribute values of the template file
// tmpl: every expression attribute, and in gsx every f`…` template.
func templateValues(tmpl string) ([]attrsite.Value, error) {
	if strings.HasSuffix(tmpl, ".gsx") {
		return gsxscan.Values(tmpl)
	}
	attrs, err := templscan.ExprAttrs(tmpl)
	if err != nil {
		return nil, err
	}
	out := make([]attrsite.Value, 0, len(attrs))
	for site, a := range attrs {
		out = append(out, attrsite.Value{
			Name: a.Name, Tag: a.Tag, Component: a.Component,
			Parts: []attrsite.Part{{Expr: true, Site: site, Source: a.Expr, Src: a.Src}},
		})
	}
	// In source order, so findings are reported in it.
	slices.SortFunc(out, func(a, b attrsite.Value) int {
		pa, pb := a.Parts[0].Src, b.Parts[0].Src
		return cmp.Or(cmp.Compare(pa.Line, pb.Line), cmp.Compare(pa.Col, pb.Col))
	})
	return out, nil
}

// generatedExprs returns the template f was generated from and, by the
// site the template's attribute index records each expression under,
// the outermost expression that starts there: for gsx, the expression
// right after each /*line*/ directive naming a .gsx position; for
// templ, every expression, by its own position in the _templ.go.
func (ctx *checkCtx) generatedExprs(f *ast.File) (tmpl string, exprs map[attrsite.Site]ast.Expr) {
	tf := ctx.pass.Fset.File(f.FileStart)
	if tf == nil {
		return "", nil
	}
	exprs = map[attrsite.Site]ast.Expr{}
	if name := tf.Name(); strings.HasSuffix(name, "_templ.go") {
		for pos, e := range outermostExprs(f) {
			p := ctx.pass.Fset.Position(pos)
			exprs[attrsite.Site{Line: p.Line, Col: p.Column}] = e
		}
		return strings.TrimSuffix(name, "_templ.go") + ".templ", exprs
	}
	// The position right after each directive, separated from it by
	// blanks only, as lineDirectiveAt reads it.
	src := ctx.text(f.FileStart, f.FileEnd)
	after := map[token.Pos]attrsite.Site{}
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			site, ok := parseLineDirective(c.Text)
			if !ok {
				continue
			}
			off := tf.Offset(c.End())
			for off < len(src) && (src[off] == ' ' || src[off] == '\t') {
				off++
			}
			if off < len(src) {
				after[tf.Pos(off)] = site
			}
		}
	}
	if len(after) == 0 {
		return "", nil
	}
	for pos, e := range outermostExprs(f) {
		site, ok := after[pos]
		if !ok {
			continue
		}
		name := ctx.pass.Fset.Position(pos).Filename
		if !strings.HasSuffix(name, ".gsx") {
			continue
		}
		tmpl = name
		exprs[site] = e
	}
	return tmpl, exprs
}

// outermostExprs returns the outermost expression of f starting at each
// position.
func outermostExprs(f *ast.File) map[token.Pos]ast.Expr {
	out := map[token.Pos]ast.Expr{}
	ast.Inspect(f, func(n ast.Node) bool {
		// A parent is visited before its children, so the first
		// expression seen at a position is the outermost one there.
		if e, ok := n.(ast.Expr); ok {
			if _, seen := out[e.Pos()]; !seen {
				out[e.Pos()] = e
			}
		}
		return true
	})
	return out
}

// idPartKind classifies a piece of an attribute value.
type idPartKind int

const (
	constPart     idPartKind = iota // compile-time constant text
	runtimePart                     // a value known only at runtime
	generatedPart                   // a structpages ID/IDTarget/Key/KeyTarget result
)

// idPart is one piece of an attribute value, its expressions split at
// string concatenation.
type idPart struct {
	kind idPartKind
	// text is a constant part's value.
	text string
	// fn, comp and nargs are a generated part's structpages function,
	// its component argument, and its argument count from the component
	// on.
	fn    string
	comp  ast.Expr
	nargs int
	// src is where the template writes the part, or the expression the
	// part was split from.
	src attrsite.Site
}

// checkKeyedValue reports v when it is an id or hx-target appending a
// runtime value to a generated id (see checkKeyed). exprs maps the
// sites of v's expression parts to their generated code.
func (ctx *checkCtx) checkKeyedValue(tmpl string, v attrsite.Value, exprs map[attrsite.Site]ast.Expr) {
	isID := strings.EqualFold(v.Name, "id")
	if v.Component || !isID && htmxattr.Base(v.Name) != "hx-target" {
		return
	}
	parts, ok := ctx.valueParts(v.Parts, exprs)
	if !ok {
		return
	}
	for i := range parts {
		if parts[i].kind == generatedPart && extendedAtRuntime(isID, parts, i) {
			ctx.reportKeyed(tmpl, v.Name, &parts[i])
		}
	}
}

// valueParts returns the pieces of an attribute value, its expressions
// joined to their generated code through exprs and split by idParts.
// ok is false when a join fails: the value is then unknown.
func (ctx *checkCtx) valueParts(parts []attrsite.Part, exprs map[attrsite.Site]ast.Expr) ([]idPart, bool) {
	var out []idPart
	for i := range parts {
		p := &parts[i]
		if !p.Expr {
			if p.Text != "" {
				out = append(out, idPart{kind: constPart, text: p.Text, src: p.Src})
			}
			continue
		}
		e, ok := exprs[p.Site]
		if !ok {
			return nil, false
		}
		if !p.Staged {
			if !slices.Equal(goTokens(ctx.source(e)), goTokens(p.Source)) {
				return nil, false
			}
			out = append(out, ctx.idParts(e, p.Src, 0)...)
			continue
		}
		// A pipeline's value is its last stage's result; gsx writes the
		// stage's call right after the directive.
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return nil, false
		}
		gp, ok := ctx.generatedID(call, p.Src)
		switch {
		case !ok:
			// Another filter's result.
			out = append(out, idPart{kind: runtimePart, src: p.Src})
		case p.Stages == 1 && !ctx.produces(call, call.Args[len(call.Args)-gp.nargs:], true, 1, p.Source):
			return nil, false
		default:
			out = append(out, gp)
		}
	}
	return slices.DeleteFunc(out, func(p idPart) bool { return p.kind == constPart && p.text == "" }), true
}

// extendedAtRuntime reports whether parts[i], a generated id, is
// extended with a runtime value in an id (isID) or hx-target value.
func extendedAtRuntime(isID bool, parts []idPart, i int) bool {
	raw := parts[i].fn == "ID" || parts[i].fn == "Key"
	if isID {
		if i != 0 || !raw {
			return false
		}
		for j := 1; j < len(parts); j++ {
			if parts[j].kind != constPart {
				return true
			}
		}
		return false
	}
	target := !raw || i > 0 && parts[i-1].kind == constPart && endsWithHash(parts[i-1].text)
	return target && runtimeInToken(parts[i+1:])
}

// runtimeInToken reports whether parts, which follow a generated #id
// in a selector, put a runtime value inside the id's token: after
// literal identifier characters that extend it, before any character
// that ends it.
func runtimeInToken(parts []idPart) bool {
	extended := false
	for _, p := range parts {
		if p.kind != constPart {
			return extended
		}
		n := identPrefix(p.text)
		if n > 0 {
			extended = true
		}
		if n < len(p.text) {
			return false
		}
	}
	return false
}

// identPrefix returns the length of the CSS identifier characters that
// start s: letters, digits, "-", "_", non-ASCII, and escapes. A
// trailing "\" escapes what follows s, so it counts.
func identPrefix(s string) int {
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c >= 0x80:
			i++
		case c == '\\':
			i++
			if i == len(s) {
				return i
			}
			if s[i] == '\n' {
				return i - 1 // not an escape
			}
			if !isHex(s[i]) {
				i++
				continue
			}
			for n := 0; n < 6 && i < len(s) && isHex(s[i]); n++ {
				i++
			}
			if i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
				i++ // a hex escape consumes one whitespace after it
			}
		default:
			return i
		}
	}
	return i
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// endsWithHash reports whether s ends in an unescaped "#", which makes
// the id that follows it an id selector.
func endsWithHash(s string) bool {
	if !strings.HasSuffix(s, "#") {
		return false
	}
	backslashes := len(s) - 1 - len(strings.TrimRight(s[:len(s)-1], `\`))
	return backslashes%2 == 0
}

// maxDefDepth bounds how many single-assignment variables idParts
// follows from a use to the expression that defines it.
const maxDefDepth = 8

// idParts splits e, an attribute value expression written at src, at
// string concatenation and classifies each operand.
func (ctx *checkCtx) idParts(e ast.Expr, src attrsite.Site, depth int) []idPart {
	e = unparen(e)
	info := ctx.pass.TypesInfo
	if tv, ok := info.Types[e]; ok && tv.Value != nil {
		text := tv.Value.ExactString()
		if tv.Value.Kind() == constant.String {
			text = constant.StringVal(tv.Value)
		}
		return []idPart{{kind: constPart, text: text, src: src}}
	}
	switch e := e.(type) {
	case *ast.BinaryExpr:
		if e.Op == token.ADD && isString(info.TypeOf(e)) {
			return append(ctx.idParts(e.X, src, depth), ctx.idParts(e.Y, src, depth)...)
		}
	case *ast.CallExpr:
		if p, ok := ctx.generatedID(e, src); ok {
			return []idPart{p}
		}
	case *ast.Ident:
		if depth >= maxDefDepth {
			break
		}
		if def, ok := ctx.singleDef(e); ok {
			if def.tuple {
				if call, ok := unparen(def.expr).(*ast.CallExpr); ok && def.index == 0 {
					if p, ok := ctx.generatedID(call, src); ok {
						return []idPart{p}
					}
				}
				break
			}
			return ctx.idParts(def.expr, src, depth+1)
		}
	}
	return []idPart{{kind: runtimePart, src: src}}
}

// generatedID returns call as a generated part when it is a call of
// structpages.ID, IDTarget, Key or KeyTarget, or of the *StructPages
// method of the same name.
func (ctx *checkCtx) generatedID(call *ast.CallExpr, src attrsite.Site) (idPart, bool) {
	fn, args, ok := structpagesCall(ctx.pass.TypesInfo, call)
	if !ok {
		return idPart{}, false
	}
	switch {
	case (fn.Name() == "ID" || fn.Name() == "IDTarget") && len(args) == 1,
		(fn.Name() == "Key" || fn.Name() == "KeyTarget") && len(args) == 2:
		return idPart{kind: generatedPart, fn: fn.Name(), comp: args[0], nargs: len(args), src: src}, true
	}
	return idPart{}, false
}

// isString reports whether t is a string type.
func isString(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsString != 0
}

// varDef is the expression a local variable is assigned from where it
// is declared: expr itself, or, for a tuple assignment such as
// `id, err := structpages.ID(ctx, x)`, result index of expr.
type varDef struct {
	expr  ast.Expr
	tuple bool
	index int
}

// singleDef returns the definition of the local variable id uses when
// it is the variable's only assignment: it is declared with a value
// (`v := x`, `var v = x`) and never assigned again or has its address
// taken, so it holds that value wherever it is used.
func (ctx *checkCtx) singleDef(id *ast.Ident) (varDef, bool) {
	info := ctx.pass.TypesInfo
	obj, ok := info.Uses[id].(*types.Var)
	if !ok || obj.IsField() || obj.Pkg() == nil || obj.Parent() == nil || obj.Parent() == obj.Pkg().Scope() {
		return varDef{}, false
	}
	fd := enclosingFunc(ctx, obj.Pos())
	if fd == nil || fd.Body == nil {
		return varDef{}, false
	}
	def, ok := declaredValue(info, fd.Body, obj)
	if !ok || assignedAgain(info, fd.Body, obj) {
		return varDef{}, false
	}
	return def, true
}

// declaredValue returns the value obj is declared with in body: by a
// short variable declaration or a var spec with values.
func declaredValue(info *types.Info, body *ast.BlockStmt, obj *types.Var) (def varDef, ok bool) {
	ast.Inspect(body, func(n ast.Node) bool {
		if ok {
			return false
		}
		var lhs []*ast.Ident
		var rhs []ast.Expr
		switch n := n.(type) {
		case *ast.AssignStmt:
			if n.Tok != token.DEFINE {
				return true
			}
			for _, l := range n.Lhs {
				li, _ := l.(*ast.Ident)
				lhs = append(lhs, li)
			}
			rhs = n.Rhs
		case *ast.ValueSpec:
			lhs, rhs = n.Names, n.Values
		default:
			return true
		}
		for i, name := range lhs {
			if name == nil || info.Defs[name] != obj {
				continue
			}
			switch {
			case len(lhs) == len(rhs):
				def, ok = varDef{expr: rhs[i]}, true
			case len(rhs) == 1:
				def, ok = varDef{expr: rhs[0], tuple: true, index: i}, true
			}
		}
		return !ok
	})
	return def, ok
}

// assignedAgain reports whether body assigns obj after its declaration
// or takes its address: &v, or a method call v.M() with a pointer
// receiver, through which it could be assigned.
func assignedAgain(info *types.Info, body *ast.BlockStmt, obj *types.Var) bool {
	is := func(e ast.Expr) bool {
		i, ok := unparen(e).(*ast.Ident)
		return ok && info.Uses[i] == obj
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.AssignStmt:
			found = slices.ContainsFunc(n.Lhs, is)
		case *ast.RangeStmt:
			found = n.Tok == token.ASSIGN && (n.Key != nil && is(n.Key) || n.Value != nil && is(n.Value))
		case *ast.IncDecStmt:
			found = is(n.X)
		case *ast.UnaryExpr:
			found = n.Op == token.AND && is(n.X)
		case *ast.SelectorExpr:
			found = is(n.X) && pointerMethod(info, n)
		}
		return !found
	})
	return found
}

// pointerMethod reports whether sel selects a pointer-receiver method
// on an operand that isn't a pointer, which Go calls on its address.
func pointerMethod(info *types.Info, sel *ast.SelectorExpr) bool {
	s, ok := info.Selections[sel]
	if !ok || s.Kind() != types.MethodVal {
		return false
	}
	sig, ok := s.Obj().Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}
	_, ptrRecv := sig.Recv().Type().(*types.Pointer)
	_, ptrOperand := s.Recv().Underlying().(*types.Pointer)
	return ptrRecv && !ptrOperand
}

// reportKeyed reports p, a generated id that attribute attr of
// template tmpl extends with a runtime value.
func (ctx *checkCtx) reportKeyed(tmpl, attr string, p *idPart) {
	pos := templatePos(ctx.pass.Fset, tmpl, p.src)
	if !pos.IsValid() {
		return
	}
	comp := ctx.source(unparen(p.comp))
	switch p.fn {
	case "ID", "IDTarget":
		fix, filter := "Key", "key"
		if p.fn == "IDTarget" || !strings.EqualFold(attr, "id") {
			fix, filter = "KeyTarget", "keytarget"
		}
		ctx.report(pos, keyedCategory,
			"%[1]s appends a runtime value to %[2]s's generated id, so it is no component's id: the htmx "+
				"target selectors can't match a request targeting it to %[2]s, and Props can't read the "+
				"key back. Build it with structpages.%[3]s (gsx: |> %[4]s(k)), whose key Props reads "+
				"with KeyOf",
			attr, comp, fix, filter)
	default:
		filter := "key"
		if p.fn == "KeyTarget" {
			filter = "keytarget"
		}
		ctx.report(pos, keyedCategory,
			"%[1]s appends a runtime value to the id %[2]s generates for %[3]s, so KeyOf reads the "+
				"appended text back as part of the key, or the id isn't a keyed id at all. Pass the whole "+
				"key to %[2]s (gsx: |> %[4]s(k))",
			attr, p.fn, comp, filter)
	}
}
