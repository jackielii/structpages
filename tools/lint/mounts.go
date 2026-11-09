package lint

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
)

// mountCall is a single discovered call that builds a page tree:
// structpages.Mount(...) or structpages.Parse(...), or a helper that
// forwards its page argument to one of them. The fields are stored as
// AST nodes so callers can re-resolve types via the owning package's
// TypesInfo.
type mountCall struct {
	Pkg     *packages.Package
	Call    *ast.CallExpr
	PageArg ast.Expr // the page value
	Route   string   // the route prefix
	// RouteArg is the call's route argument, nil when the route is a
	// constant inside the helper called.
	RouteArg ast.Expr
	// mount is the position key (see posKey) of the structpages.Mount
	// call that serves this tree: Call itself when it is a Mount, the
	// Mount inside the helper it calls otherwise. Empty when the tree
	// is built by Parse, which serves nothing.
	mount string
}

// treeBuilder describes how a function builds a page tree from its
// arguments: which parameter is the page, and where the route prefix
// comes from — a parameter, or a constant fixed inside the function.
type treeBuilder struct {
	page  int
	route int // parameter index of the route; -1 when routeConst is fixed
	// routeConst is the route the function passes on itself, when
	// route is -1.
	routeConst string
	// mountsHere marks structpages.Mount itself: a call to it is the
	// Mount that serves the tree.
	mountsHere bool
	// mount is the position key of the structpages.Mount call a helper
	// reaches, or "" when it reaches Parse.
	mount string
}

// treeBuilders maps a function's full name (types.Func.FullName) to how
// it builds a page tree.
type treeBuilders struct {
	byName map[string]treeBuilder
	// short holds the builders' bare names (types.Func.Name), checked
	// before building a full name: most calls are to other functions.
	short map[string]bool
}

func (b treeBuilders) add(fn string, tb treeBuilder) {
	b.byName[fn] = tb
	b.short[fn[strings.LastIndexByte(fn, '.')+1:]] = true
}

// lookup returns how fn builds a page tree, if it does.
func (b treeBuilders) lookup(fn *types.Func) (treeBuilder, bool) {
	if !b.short[fn.Name()] {
		return treeBuilder{}, false
	}
	tb, ok := b.byName[fn.Origin().FullName()]
	return tb, ok
}

// findTreeBuilders returns structpages.Mount and Parse (Parse is Mount
// without the mux: tests use it to get a PageContext), plus every
// function in pkgs that passes one of its own interface-typed
// parameters on as the page argument of a tree builder — a test helper
// such as
//
//	func serve(t *testing.T, pages any) { structpages.Mount(mux, pages, "/", "t") }
//
// A call serve(t, appRoot{}) then builds appRoot's tree, just as the
// runtime does. Helpers that forward to other helpers are found by
// iterating to a fixed point, across packages.
//
// The route must be a constant at some level: a helper whose route
// argument is neither a constant nor one of its parameters is not a
// builder, since no call site can tell where its tree is mounted.
func findTreeBuilders(pkgs []*packages.Package) treeBuilders {
	builders := treeBuilders{byName: map[string]treeBuilder{}, short: map[string]bool{}}
	builders.add(structpagesPkg+".Mount", treeBuilder{page: 1, route: 2, mountsHere: true})
	builders.add(structpagesPkg+".Parse", treeBuilder{page: 0, route: 1})

	// Collect the functions that could forward a page once: those with
	// an interface-typed parameter (the page must be one), each with the
	// calls in its body that have a static callee. A function seen in
	// several package variants is collected once. The fixed-point
	// iteration below then scans only these call lists.
	type candidate struct {
		pkg   *packages.Package
		fn    *types.Func
		name  string
		calls []*ast.CallExpr
	}
	var cands []*candidate
	seen := map[string]bool{}
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				fn, ok := pkg.TypesInfo.Defs[fd.Name].(*types.Func)
				if !ok || !hasInterfaceParam(fn) {
					continue
				}
				name := fn.FullName()
				if seen[name] {
					continue
				}
				seen[name] = true
				c := &candidate{pkg: pkg, fn: fn, name: name}
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					if call, ok := n.(*ast.CallExpr); ok && calleeFunc(pkg.TypesInfo, call) != nil {
						c.calls = append(c.calls, call)
					}
					return true
				})
				cands = append(cands, c)
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, c := range cands {
			if _, known := builders.byName[c.name]; known {
				continue
			}
			if b, ok := forwardedBuilder(c.pkg, c.fn, c.calls, builders); ok {
				builders.add(c.name, b)
				changed = true
			}
		}
	}
	return builders
}

// hasInterfaceParam reports whether fn has a parameter of interface type.
func hasInterfaceParam(fn *types.Func) bool {
	params := fn.Type().(*types.Signature).Params()
	for i := range params.Len() {
		if types.IsInterface(params.At(i).Type()) {
			return true
		}
	}
	return false
}

// forwardedBuilder reports whether fn, through one of calls (its body's
// calls, in source order), calls a tree builder with one of its own
// interface-typed parameters as the page, and if so how fn itself builds
// a tree.
func forwardedBuilder(
	pkg *packages.Package, fn *types.Func, calls []*ast.CallExpr, builders treeBuilders,
) (treeBuilder, bool) {
	params := fn.Type().(*types.Signature).Params()
	paramIndex := func(e ast.Expr) int {
		id, ok := unparen(e).(*ast.Ident)
		if !ok {
			return -1
		}
		v, ok := pkg.TypesInfo.Uses[id].(*types.Var)
		if !ok {
			return -1
		}
		for i := range params.Len() {
			if params.At(i) == v {
				return i
			}
		}
		return -1
	}
	for _, call := range calls {
		b, isBuilder := builderFor(pkg.TypesInfo, call, builders)
		if !isBuilder {
			continue
		}
		page := paramIndex(call.Args[b.page])
		if page < 0 || !types.IsInterface(params.At(page).Type()) {
			continue
		}
		route, routeConst, routed := callRoute(pkg, call, b)
		if !routed {
			if route = paramIndex(call.Args[b.route]); route < 0 {
				continue
			}
		}
		return treeBuilder{page: page, route: route, routeConst: routeConst, mount: reachedMount(pkg, call, b)}, true
	}
	return treeBuilder{}, false
}

// builderFor returns how call builds a page tree, if its callee is a
// tree builder and the call passes the page and route arguments.
func builderFor(info *types.Info, call *ast.CallExpr, builders treeBuilders) (treeBuilder, bool) {
	fn := calleeFunc(info, call)
	if fn == nil {
		return treeBuilder{}, false
	}
	b, ok := builders.lookup(fn)
	if !ok || len(call.Args) <= max(b.page, b.route) {
		return treeBuilder{}, false
	}
	// A spread (f(a, b, opts...)) fills only the variadic parameter,
	// which is never the page (an interface) or the route (a string):
	// Mount(mux, p, "/", "App", opts...) still names both. Reject the
	// call only if the spread sits where one of them is expected.
	if call.Ellipsis.IsValid() && len(call.Args)-1 <= max(b.page, b.route) {
		return treeBuilder{}, false
	}
	return b, true
}

// reachedMount returns the position key of the structpages.Mount call
// that a call to builder b ends in: the call itself when b is Mount.
func reachedMount(pkg *packages.Package, call *ast.CallExpr, b treeBuilder) string {
	if b.mountsHere {
		return posKey(pkg.Fset, call.Pos())
	}
	return b.mount
}

// posKey renders pos as "file:line:col", the key that identifies a call
// across the package variants packages.Load reports it under.
func posKey(fset *token.FileSet, pos token.Pos) string {
	p := fset.Position(pos)
	return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Column)
}

// callRoute returns the route prefix call mounts its tree at, when it
// is known there: the builder's own constant, or a constant argument.
// route is -1 whenever routed is true.
func callRoute(pkg *packages.Package, call *ast.CallExpr, b treeBuilder) (route int, routeConst string, routed bool) {
	if b.route < 0 {
		return -1, b.routeConst, true
	}
	if s, ok := stringConstant(pkg, call.Args[b.route]); ok {
		return -1, s, true
	}
	return b.route, "", false
}

// findMounts returns every call in pkg that builds a page tree with a
// known route.
func findMounts(pkg *packages.Package, builders treeBuilders) []mountCall {
	var calls []mountCall
	for _, file := range pkg.Syntax {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			b, ok := builderFor(pkg.TypesInfo, call, builders)
			if !ok {
				return true
			}
			_, route, routed := callRoute(pkg, call, b)
			if !routed {
				return true
			}
			var routeArg ast.Expr
			if b.route >= 0 {
				routeArg = call.Args[b.route]
			}
			calls = append(calls, mountCall{
				RouteArg: routeArg,
				Pkg:      pkg,
				Call:     call,
				PageArg:  call.Args[b.page],
				Route:    route,
				mount:    reachedMount(pkg, call, b),
			})
			return true
		})
	}
	return calls
}
