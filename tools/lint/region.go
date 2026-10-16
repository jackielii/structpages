package lint

import (
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"strconv"
	"strings"

	"github.com/jackielii/structpages/tools/lint/gsxscan"
	"github.com/jackielii/structpages/tools/lint/internal/htmxattr"
)

// regionCategory is the diagnostic category of a page that renders a
// region carrying another page's component id.
const regionCategory = "region"

// checkRegion flags a page that borrows another page's region identity.
// In a component method of mounted page P, an element whose
//
//   - id is ID(ctx, Q.M), or
//   - hx-target (any htmx spelling: hx-target:inherited, data-hx-target)
//     is IDTarget(ctx, Q.M), outside an <hx-partial>,
//
// for another page Q mounted in the same tree as P makes requests from
// that region come back to P with a target that is Q.M's id. P has no such component, so P
// renders its Page into the region. Q.M is the method expression or
// bound method value passed to ID/IDTarget, or the trailing element of
// its []any chain form. It isn't flagged when P's Props recognises the
// region with t.Is(Q.M).
//
// The attribute is read from the .gsx source: gsx writes a
// /*line file:line:col*/ directive right before each call it lowers a
// pipeline stage to, naming that stage's name, and before a seed,
// naming its first character, so the directive names exactly one
// attribute (gsxscan.ExprAttrs). Calls outside .gsx files have no
// attribute to check.
//
// args starts at the component argument, as for checkIDOrIDTarget.
func checkRegion(ctx *checkCtx, call *ast.CallExpr, args []ast.Expr, fnName string) {
	if len(args) != 1 || len(ctx.roots) == 0 {
		return
	}
	owner, ok := regionComponent(ctx, args[0])
	if !ok {
		return
	}
	attr, ok := ctx.gsxAttrOf(call)
	if !ok || attr.Component {
		return
	}
	switch fnName {
	case "ID":
		if !strings.EqualFold(attr.Name, "id") {
			return
		}
	case "IDTarget":
		// An <hx-partial> is an out-of-band swap in a response: its
		// hx-target places content and no request targets it.
		if htmxattr.Base(attr.Name) != "hx-target" || htmxattr.IsElement(attr.Tag) {
			return
		}
	default:
		return
	}
	page, ok := componentPage(ctx, enclosingFunc(ctx, call.Pos()))
	if !ok || typeKey(page) == typeKey(owner.recvType) || !ctx.mountedTogether(page, owner.recvType) {
		return
	}
	if propsRecognises(ctx, page, owner) {
		return
	}
	p, q := page.Obj().Name(), owner.recvType.Obj().Name()+"."+owner.fnName
	ctx.report(call.Pos(), regionCategory,
		"%s renders an element whose %s is %s's id: requests from that element reach %s "+
			"targeting %s's id, which %s has no component for, so %s renders its Page into the region. "+
			"Give the region a component of %s's own, or recognise it in %s's Props with "+
			"t.Is(%s) and RenderComponent",
		p, attr.Name, q, p, q, p, p, p, p, q)
}

// mountedTogether reports whether some tree that can serve this
// package mounts both p and q. The region bug happens in any such
// tree, so unlike the resolution checks (inSomeTree), one tree that
// mounts only one of them, such as a test tree stubbing a section,
// doesn't clear it.
func (ctx *checkCtx) mountedTogether(p, q *types.Named) bool {
	pk, qk := typeKey(p), typeKey(q)
	for _, root := range ctx.roots {
		var hasP, hasQ bool
		root.All(func(n *PageNode) bool {
			switch typeKey(n.Type) {
			case pk:
				hasP = true
			case qk:
				hasQ = true
			}
			return !hasP || !hasQ
		})
		if hasP && hasQ {
			return true
		}
	}
	return false
}

// regionComponent returns the component an ID/IDTarget argument names
// when it is a method: a method expression or bound method value, or
// the trailing element of the []any chain form (the chain's leading
// steps place the id, they don't change whose component it is).
func regionComponent(ctx *checkCtx, arg ast.Expr) (methodExprInfo, bool) {
	a := unparen(arg)
	if comp, ok := asAnySliceLiteral(ctx, a); ok {
		if len(comp.Elts) == 0 {
			return methodExprInfo{}, false
		}
		a = unparen(comp.Elts[len(comp.Elts)-1])
	}
	return lastAsMethodExpr(ctx, a)
}

// gsxAttrOf returns the .gsx attribute whose value call produces: the
// attribute whose last pipeline stage sits at call's .gsx position
// (`id={X |> id}`), or whose whole seed is call (`id={structpages.ID(ctx, X)}`).
func (ctx *checkCtx) gsxAttrOf(call *ast.CallExpr) (gsxscan.ExprAttr, bool) {
	p := ctx.pass.Fset.Position(call.Pos())
	if !strings.HasSuffix(p.Filename, ".gsx") {
		return gsxscan.ExprAttr{}, false
	}
	site, ok := ctx.lineDirectiveAt(call.Pos())
	if !ok {
		return gsxscan.ExprAttr{}, false
	}
	attrs, ok := ctx.gsxAttrs[p.Filename]
	if !ok {
		var err error
		if attrs, err = gsxscan.ExprAttrs(p.Filename); err != nil {
			attrs = nil // gsx generation would have failed on it first
		}
		if ctx.gsxAttrs == nil {
			ctx.gsxAttrs = map[string]map[gsxscan.ExprSite]gsxscan.ExprAttr{}
		}
		ctx.gsxAttrs[p.Filename] = attrs
	}
	attr, ok := attrs[site]
	if !ok {
		return gsxscan.ExprAttr{}, false
	}
	// A seed site only says the seed starts with call; the value is
	// call's result only if the seed is call and nothing more.
	if !attr.Staged && ctx.source(call) != strings.TrimSpace(attr.Expr) {
		return gsxscan.ExprAttr{}, false
	}
	return attr, true
}

// lineDirectiveAt returns the .gsx line and column named by the
// `/*line file:line:col*/` directive written immediately before pos,
// separated from it by blanks only. That is the site gsx records for
// the lowered call starting at pos. The column is the directive's, not
// pos's: Go places the directive's position on the character right
// after the comment, which is the blank gsx writes before the call.
func (ctx *checkCtx) lineDirectiveAt(pos token.Pos) (gsxscan.ExprSite, bool) {
	f := ctx.fileOf(pos)
	if f == nil {
		return gsxscan.ExprSite{}, false
	}
	var last *ast.Comment
	for _, cg := range f.Comments {
		if cg.Pos() >= pos {
			break
		}
		for _, c := range cg.List {
			if c.End() <= pos {
				last = c
			}
		}
	}
	if last == nil || !strings.HasPrefix(last.Text, "/*line ") ||
		strings.Trim(ctx.text(last.End(), pos), " \t") != "" {
		return gsxscan.ExprSite{}, false
	}
	spec := strings.TrimSuffix(strings.TrimPrefix(last.Text, "/*line "), "*/")
	// file:line:col, where file may itself contain colons.
	rest, colStr, ok := cutLast(spec, ":")
	if !ok {
		return gsxscan.ExprSite{}, false
	}
	_, lineStr, ok := cutLast(rest, ":")
	if !ok {
		return gsxscan.ExprSite{}, false
	}
	line, err1 := strconv.Atoi(lineStr)
	col, err2 := strconv.Atoi(colStr)
	if err1 != nil || err2 != nil {
		return gsxscan.ExprSite{}, false
	}
	return gsxscan.ExprSite{Line: line, Col: col}, true
}

// cutLast slices s around the last instance of sep.
func cutLast(s, sep string) (before, after string, found bool) {
	i := strings.LastIndex(s, sep)
	if i < 0 {
		return s, "", false
	}
	return s[:i], s[i+len(sep):], true
}

// fileOf returns the syntax tree of the pass file containing pos.
func (ctx *checkCtx) fileOf(pos token.Pos) *ast.File {
	for _, f := range ctx.pass.Files {
		if f.FileStart <= pos && pos < f.FileEnd {
			return f
		}
	}
	return nil
}

// source returns n's source text in the generated file it was parsed
// from, or "" when the file can't be read.
func (ctx *checkCtx) source(n ast.Node) string {
	return ctx.text(n.Pos(), n.End())
}

// text returns the source text between start and end, which lie in
// one file, or "" when the file can't be read.
func (ctx *checkCtx) text(start, end token.Pos) string {
	tf := ctx.pass.Fset.File(start)
	if tf == nil {
		return ""
	}
	src, ok := ctx.srcs[tf.Name()]
	if !ok {
		src, _ = os.ReadFile(tf.Name())
		if ctx.srcs == nil {
			ctx.srcs = map[string][]byte{}
		}
		ctx.srcs[tf.Name()] = src
	}
	s, e := tf.Offset(start), tf.Offset(end)
	if e > len(src) || s > e {
		return ""
	}
	return string(src[s:e])
}

// enclosingFunc returns the function declaration containing pos.
func enclosingFunc(ctx *checkCtx, pos token.Pos) *ast.FuncDecl {
	f := ctx.fileOf(pos)
	if f == nil {
		return nil
	}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Pos() <= pos && pos < fd.End() {
			return fd
		}
	}
	return nil
}

// componentPage returns the receiver type of fd when fd is a component
// method: a method on a named struct returning one value that has a
// Render(context.Context, io.Writer) error method. gsx's
// `component (p P) M()` lowers to one.
func componentPage(ctx *checkCtx, fd *ast.FuncDecl) (*types.Named, bool) {
	if fd == nil || fd.Recv == nil {
		return nil, false
	}
	fn, ok := ctx.pass.TypesInfo.Defs[fd.Name].(*types.Func)
	if !ok {
		return nil, false
	}
	sig := fn.Type().(*types.Signature)
	if sig.Recv() == nil || sig.Results().Len() != 1 || !isComponentType(sig.Results().At(0).Type()) {
		return nil, false
	}
	return resolveNamedStruct(sig.Recv().Type())
}

// isComponentType reports whether t has the structpages component
// method Render(context.Context, io.Writer) error.
func isComponentType(t types.Type) bool {
	obj, _, _ := types.LookupFieldOrMethod(t, false, nil, "Render")
	fn, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	sig := fn.Type().(*types.Signature)
	return sig.Params().Len() == 2 && sig.Results().Len() == 1 &&
		isNamedType(sig.Params().At(0).Type(), "context", "Context") &&
		isNamedType(sig.Params().At(1).Type(), "io", "Writer") &&
		types.Identical(sig.Results().At(0).Type(), types.Universe.Lookup("error").Type())
}

// isNamedType reports whether t is the named type pkg.name.
func isNamedType(t types.Type, pkg, name string) bool {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == pkg && obj.Name() == name
}

// propsRecognises reports whether page's own Props method calls Is on a
// structpages.RenderTarget with component (a method expression or
// bound method value of the same method): page then handles requests
// targeting that region itself. Props is declared in page's package,
// which is the package being analysed, since the component method that
// borrows the region is a method of page too.
func propsRecognises(ctx *checkCtx, page *types.Named, component methodExprInfo) bool {
	info := ctx.pass.TypesInfo
	for _, f := range ctx.pass.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || fd.Body == nil || fd.Name.Name != "Props" {
				continue
			}
			fn, ok := info.Defs[fd.Name].(*types.Func)
			if !ok {
				continue
			}
			recv, ok := resolveNamedStruct(fn.Type().(*types.Signature).Recv().Type())
			if !ok || typeKey(recv) != typeKey(page) {
				continue
			}
			found := false
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if found {
					return false
				}
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) != 1 {
					return true
				}
				callee := calleeFunc(info, call)
				if callee == nil || !isRenderTargetMethod(callee) {
					return true
				}
				m, ok := lastAsMethodExpr(ctx, unparen(call.Args[0]))
				found = ok && m.fnName == component.fnName && typeKey(m.recvType) == typeKey(component.recvType)
				return !found
			})
			if found {
				return true
			}
		}
	}
	return false
}
