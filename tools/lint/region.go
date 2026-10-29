package lint

import (
	"go/ast"
	"go/scanner"
	"go/token"
	"go/types"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/jackielii/structpages/tools/lint/gsxscan"
	"github.com/jackielii/structpages/tools/lint/internal/attrsite"
	"github.com/jackielii/structpages/tools/lint/internal/htmxattr"
	"github.com/jackielii/structpages/tools/lint/templscan"
)

// regionCategory is the diagnostic category of a page that renders a
// region carrying another page's component id.
const regionCategory = "region"

// checkRegion flags a page that borrows another page's region identity.
// In a component method of mounted page P, an element whose
//
//   - id is ID(ctx, Q.M) or Key(ctx, Q.M, k), or
//   - hx-target (any htmx spelling: hx-target:inherited, data-hx-target)
//     is IDTarget(ctx, Q.M) or KeyTarget(ctx, Q.M, k), outside an
//     <hx-partial>,
//
// for another page Q mounted in the same tree as P, carries Q.M's id
// into P's markup. A request that targets it and comes back to P asks
// P for a component it doesn't have, so P renders its Page into the
// region. Which requests come back to P:
//
//   - The element's own request attributes (hx-get … hx-delete), if it
//     has any: the finding stands only if each is a URLFor of a page
//     that resolves to P. One reaching another page, or one whose URL
//     isn't a URLFor call the linter can resolve, clears it.
//   - Otherwise requests from inside the element (a region that its
//     descendants reload, or that hx-target:inherited hands down), which
//     are reported as coming back to P.
//
// Q.M is the method expression or bound method value passed to
// ID/IDTarget/Key/KeyTarget, or the trailing element of its []any chain
// form, owned
// the way structpages.ID resolves it at runtime (see runtimeOwner). It
// isn't flagged when P's Props recognises the region with t.Is(Q.M), or
// when the element takes a spread of attributes whose names are only
// known at runtime.
//
// The attribute is read from the template source, joined exactly to
// the call (see attrOf): through gsx's /*line*/ directives for .gsx,
// through templ's source map for .templ. Calls in other files have no
// attribute to check.
//
// args starts at the component argument, as for checkIDOrIDTarget; Key
// and KeyTarget take the instance key after it.
func checkRegion(ctx *checkCtx, call *ast.CallExpr, args []ast.Expr, fnName string) {
	nargs := 1
	if fnName == "Key" || fnName == "KeyTarget" {
		nargs = 2
	}
	if len(args) != nargs || len(ctx.roots) == 0 {
		return
	}
	owner, ok := regionComponent(ctx, args[0])
	if !ok {
		return
	}
	attr, pos, ok := ctx.attrOf(call, args)
	if !ok || attr.Component || attr.Spread {
		return
	}
	switch fnName {
	case "ID", "Key":
		if !strings.EqualFold(attr.Name, "id") {
			return
		}
	case "IDTarget", "KeyTarget":
		// An <hx-partial> is an out-of-band swap in a response: its
		// hx-target places content and no request targets it.
		if htmxattr.Base(attr.Name) != "hx-target" || htmxattr.IsElement(attr.Tag) {
			return
		}
	default:
		return
	}
	fd := enclosingFunc(ctx, call.Pos())
	page, ok := componentPage(ctx, fd)
	if !ok || typeKey(page) == typeKey(owner.recvType) || !ctx.mountedTogether(page, owner.recvType) {
		return
	}
	if propsRecognises(ctx, page, owner) {
		return
	}
	p, q := page.Obj().Name(), owner.recvType.Obj().Name()+"."+owner.fnName
	const fix = " Give the region a component of %[1]s's own, or recognise it in %[1]s's Props with " +
		"t.Is(%[3]s) and RenderComponent"
	if len(attr.Requests) == 0 {
		ctx.report(pos, regionCategory,
			"%[1]s renders an element whose %[2]s is %[3]s's id: requests from inside that element that "+
				"come back to %[1]s target %[3]s's id, which %[1]s has no component for, so %[1]s renders "+
				"its Page into the region."+fix,
			p, attr.Name, q)
		return
	}
	if !ctx.requestsReach(fd, attr.Requests, page, owner.recvType) {
		return
	}
	ctx.report(pos, regionCategory,
		"%[1]s renders an element whose %[2]s is %[3]s's id, and the element's %[4]s request reaches "+
			"%[1]s targeting that id, which %[1]s has no component for, so %[1]s renders its Page into "+
			"the region."+fix,
		p, attr.Name, q, attr.Requests[0].Name)
}

// requestsReach reports whether, in some tree mounting both page and
// q, every request attribute in reqs is a URLFor call in fd that
// resolves to page.
func (ctx *checkCtx) requestsReach(fd *ast.FuncDecl, reqs []attrsite.Request, page, q *types.Named) bool {
	var urlArgs [][]ast.Expr
	for _, r := range reqs {
		if !r.Expr {
			return false // a literal or template value: not a URLFor call
		}
		args, ok := ctx.urlForAt(fd, r)
		if !ok {
			return false
		}
		urlArgs = append(urlArgs, args)
	}
	roots, silent, silenced := ctx.roots, ctx.silent, ctx.silenced
	defer func() { ctx.roots, ctx.silent, ctx.silenced = roots, silent, silenced }()
	ctx.silent = true
	for _, root := range roots {
		ctx.roots = []*PageNode{root}
		if !ctx.mountedTogether(page, q) {
			continue
		}
		all := true
		for _, args := range urlArgs {
			node := resolvePageArg(ctx, args[0])
			if node == nil || typeKey(node.Type) != typeKey(page) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

// urlForAt returns the arguments, from the page on, of the
// structpages.URLFor call in fd that produces the value of request
// attribute r.
func (ctx *checkCtx) urlForAt(fd *ast.FuncDecl, r attrsite.Request) ([]ast.Expr, bool) {
	if fd == nil || fd.Body == nil {
		return nil, false
	}
	var found []ast.Expr
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fn, args, ok := structpagesCall(ctx.pass.TypesInfo, call)
		if !ok || fn.Name() != "URLFor" || len(args) == 0 {
			return true
		}
		if site, ok := ctx.joinSite(call); ok && site == r.Site && ctx.produces(call, args, r.Staged, r.Stages, r.Source) {
			found = args
		}
		return true
	})
	return found, found != nil
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
	return runtimeOwner(ctx, a)
}

// runtimeOwner returns the page type and method a method reference
// names the way structpages resolves it at runtime from the function
// value. A method expression (T.M, (*T).M) is owned by T, the type it
// is written on, even when M is promoted from an embedded field: Go
// generates a T.M wrapper whose receiver is T. A method value (v.M) is
// owned by the type declaring M, whose name the runtime reads from the
// bound function's symbol.
func runtimeOwner(ctx *checkCtx, expr ast.Expr) (methodExprInfo, bool) {
	m, ok := lastAsMethodExpr(ctx, expr)
	if !ok {
		return m, false
	}
	sel := expr.(*ast.SelectorExpr)
	if s, ok := ctx.pass.TypesInfo.Selections[sel]; ok && s.Kind() == types.MethodExpr {
		named, ok := resolveNamedStruct(s.Recv())
		if !ok {
			return methodExprInfo{}, false
		}
		m.recvType = named
	}
	return m, true
}

// attrOf returns the template attribute whose value call produces,
// and the position to report a finding on it at. args are call's
// arguments from the component on.
//
//   - In gsx code (positioned in a .gsx by gsx's //line directives) it
//     is the attribute whose last pipeline stage the /*line*/ directive
//     before call names (`id={X |> id}`), or whose whole seed is call
//     (`id={structpages.ID(ctx, X)}`). The report goes at the site the
//     directive names.
//   - In templ code (a _templ.go) it is the attribute whose whole
//     expression is call, found through templ's source map (see
//     templscan.ExprAttrs). The report goes at the expression in the
//     .templ, since templ writes no position mapping into its output.
func (ctx *checkCtx) attrOf(call *ast.CallExpr, args []ast.Expr) (attrsite.Attr, token.Pos, bool) {
	tmpl, index, ok := ctx.templateOf(call)
	if !ok {
		return attrsite.Attr{}, token.NoPos, false
	}
	site, ok := ctx.joinSite(call)
	if !ok {
		return attrsite.Attr{}, token.NoPos, false
	}
	attrs, err := index(tmpl)
	if err != nil {
		// A template that doesn't parse failed generation first; a stale
		// _templ.go has no trustworthy positions. Either way its
		// attributes are unknown.
		return attrsite.Attr{}, token.NoPos, false
	}
	attr, ok := attrs[site]
	if !ok || !ctx.produces(call, args, attr.Staged, attr.Stages, attr.Expr) {
		return attrsite.Attr{}, token.NoPos, false
	}
	report := attr.Src
	if strings.HasSuffix(tmpl, ".gsx") {
		report = site
	}
	pos := templatePos(ctx.pass.Fset, tmpl, report)
	return attr, pos, pos.IsValid()
}

// templateOf returns the template file call was generated from and the
// index of its attributes.
func (ctx *checkCtx) templateOf(call *ast.CallExpr) (
	file string, index func(string) (map[attrsite.Site]attrsite.Attr, error), ok bool,
) {
	p := ctx.pass.Fset.Position(call.Pos())
	switch {
	case strings.HasSuffix(p.Filename, ".gsx"):
		return p.Filename, gsxscan.ExprAttrs, true
	case strings.HasSuffix(p.Filename, "_templ.go"):
		return strings.TrimSuffix(p.Filename, "_templ.go") + ".templ", templscan.ExprAttrs, true
	}
	return "", nil, false
}

// joinSite returns the site under which the attribute index of call's
// template records the attribute call may produce: the .gsx position
// named by the /*line*/ directive before call for gsx, call's own
// position in the _templ.go for templ.
func (ctx *checkCtx) joinSite(call *ast.CallExpr) (attrsite.Site, bool) {
	p := ctx.pass.Fset.Position(call.Pos())
	switch {
	case strings.HasSuffix(p.Filename, ".gsx"):
		return ctx.lineDirectiveAt(call.Pos())
	case strings.HasSuffix(p.Filename, "_templ.go"):
		return attrsite.Site{Line: p.Line, Col: p.Column}, true
	}
	return attrsite.Site{}, false
}

// produces reports whether call computes the value of an attribute
// whose expression is expr. A site only says where call starts, so the
// code is compared too, token by token (comments and layout aside):
//
//   - unstaged, the expression must be call and nothing more;
//   - staged, call must be the attribute's only pipeline stage, and its
//     argument the seed. gsx passes the seed parenthesised. With more
//     stages call's argument is another stage's result, which names no
//     component the linter can see.
func (ctx *checkCtx) produces(call *ast.CallExpr, args []ast.Expr, staged bool, stages int, expr string) bool {
	if !staged {
		return slices.Equal(goTokens(ctx.source(call)), goTokens(expr))
	}
	if stages != 1 || len(args) == 0 {
		return false
	}
	seed := args[0]
	if p, ok := seed.(*ast.ParenExpr); ok {
		seed = p.X
	}
	return slices.Equal(goTokens(ctx.source(seed)), goTokens(expr))
}

// goTokens returns the tokens of the Go source src, without comments
// and semicolons, each as its kind and literal text.
func goTokens(src string) []string {
	fset := token.NewFileSet()
	file := fset.AddFile("", -1, len(src))
	var s scanner.Scanner
	s.Init(file, []byte(src), nil, 0)
	var out []string
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			return out
		}
		if tok == token.SEMICOLON {
			continue
		}
		out = append(out, tok.String()+" "+lit)
	}
}

// templateFiles holds the template files added to each FileSet by
// templatePos, so each is added once.
var templateFiles = struct {
	sync.Mutex
	m map[*token.FileSet]map[string]*token.File
}{m: map[*token.FileSet]map[string]*token.File{}}

// templatePos returns a position for site in the template file name,
// adding the file to fset on first use so a diagnostic can point into
// it. NoPos when the file can't be read or site is outside it.
func templatePos(fset *token.FileSet, name string, site attrsite.Site) token.Pos {
	templateFiles.Lock()
	defer templateFiles.Unlock()
	files := templateFiles.m[fset]
	if files == nil {
		files = map[string]*token.File{}
		templateFiles.m[fset] = files
	}
	f, ok := files[name]
	if !ok {
		if src, err := os.ReadFile(name); err == nil {
			f = fset.AddFile(name, -1, len(src))
			f.SetLinesForContent(src)
		}
		files[name] = f
	}
	if f == nil || site.Line < 1 || site.Line > f.LineCount() {
		return token.NoPos
	}
	start := f.LineStart(site.Line)
	if f.Offset(start)+site.Col-1 > f.Size() {
		return token.NoPos
	}
	return start + token.Pos(site.Col-1)
}

// lineDirectiveAt returns the .gsx line and column named by the
// `/*line file:line:col*/` directive written immediately before pos,
// separated from it by blanks only. That is the site gsx records for
// the lowered call starting at pos. The column is the directive's, not
// pos's: Go places the directive's position on the character right
// after the comment, which is the blank gsx writes before the call.
func (ctx *checkCtx) lineDirectiveAt(pos token.Pos) (attrsite.Site, bool) {
	f := ctx.fileOf(pos)
	if f == nil {
		return attrsite.Site{}, false
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
	if last == nil || strings.Trim(ctx.text(last.End(), pos), " \t") != "" {
		return attrsite.Site{}, false
	}
	return parseLineDirective(last.Text)
}

// parseLineDirective returns the line and column a
// `/*line file:line:col*/` comment names.
func parseLineDirective(text string) (attrsite.Site, bool) {
	if !strings.HasPrefix(text, "/*line ") {
		return attrsite.Site{}, false
	}
	spec := strings.TrimSuffix(strings.TrimPrefix(text, "/*line "), "*/")
	// file:line:col, where file may itself contain colons.
	rest, colStr, ok := cutLast(spec, ":")
	if !ok {
		return attrsite.Site{}, false
	}
	_, lineStr, ok := cutLast(rest, ":")
	if !ok {
		return attrsite.Site{}, false
	}
	line, err1 := strconv.Atoi(lineStr)
	col, err2 := strconv.Atoi(colStr)
	if err1 != nil || err2 != nil {
		return attrsite.Site{}, false
	}
	return attrsite.Site{Line: line, Col: col}, true
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
				m, ok := runtimeOwner(ctx, unparen(call.Args[0]))
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
