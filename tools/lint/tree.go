// Package lint provides a static analyzer for the structpages framework.
//
// It catches runtime failure modes (Ref typos, URLFor chain drift,
// param-map mismatches, IDTarget receivers that aren't mounted) at CI
// time by reconstructing the page tree from struct tags and validating
// call sites with go/types.
package lint

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path"
	"reflect"
	"strings"

	"golang.org/x/tools/go/packages"
)

// structpagesPkg is the canonical import path the analyzer matches
// against to identify framework calls. Localised here so a rename in
// the framework only needs one update.
const structpagesPkg = "github.com/jackielii/structpages"

// PageTree is the static reconstruction of every structpages.Mount(...)
// rooted tree in the analyzed module. Each root in Roots corresponds to
// exactly one Mount call site.
type PageTree struct {
	Roots []*PageNode
}

// PageNode mirrors the runtime structpages.PageNode but holds
// go/types information instead of reflect values.
type PageNode struct {
	Name      string
	Type      *types.Named // pointer-normalised to the underlying named struct type
	TypePos   token.Pos
	Route     string
	Method    string
	FullRoute string
	Parent    *PageNode
	Children  []*PageNode
	Methods   map[string]*types.Func

	// binaries is the set of main-package import paths whose transitive
	// import graph reaches this root's Mount call. Populated only on
	// root nodes; descendants inherit their root's set. It is nil when
	// no main package was in scope (library-only analysis), in which
	// case all such roots share one implicit binary. Used to scope the
	// URLFor type-ambiguity check per binary (issue #22): a type
	// mounted once per binary across separate main packages is
	// unambiguous in each process.
	binaries map[string]bool
}

// Diagnostic is a tree-build-time diagnostic. Call-site diagnostics
// flow through the analysis.Analyzer pipeline instead.
type Diagnostic struct {
	Pos     token.Position
	Message string
}

// All iterates this PageNode and every descendant in depth-first order.
func (pn *PageNode) All(yield func(*PageNode) bool) {
	pn.walk(yield)
}

func (pn *PageNode) walk(yield func(*PageNode) bool) bool {
	if !yield(pn) {
		return false
	}
	for _, c := range pn.Children {
		if !c.walk(yield) {
			return false
		}
	}
	return true
}

// BuildTree finds every structpages.Mount call across the supplied
// packages and reconstructs the corresponding page trees. Tree-build
// diagnostics (e.g. Mount with a non-struct page argument) are
// returned alongside the tree so the driver can surface them.
//
// Two deduplication passes:
//
//  1. By source position — packages.Load(Tests: true) reports the
//     same call once per test variant; collapse those.
//
//  2. By (root type, root route) — multiple Mount call sites that
//     mount the same root type at the same route (production main +
//     test setup) describe the same logical tree; collapse to one
//     root to avoid spurious "ambiguous: type mounted N times" errors.
//
// After deduplication each kept root is tagged with the set of
// binaries (main packages) that can execute its Mount call, so the
// URLFor ambiguity check can be scoped per binary rather than module-
// global (issue #22).
func BuildTree(pkgs []*packages.Package) (*PageTree, []Diagnostic) {
	tree := &PageTree{}
	var diags []Diagnostic
	seenPos := map[string]bool{}
	roots := map[string]*PageNode{}    // root key -> kept root
	mountPkgs := map[string][]string{} // root key -> Mount-call package paths

	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for _, m := range findMounts(pkg) {
			pos := pkg.Fset.Position(m.Call.Pos())
			key := fmt.Sprintf("%s:%d:%d", pos.Filename, pos.Line, pos.Column)
			if seenPos[key] {
				continue
			}
			seenPos[key] = true
			root, ds := buildRoot(pkg, m)
			diags = append(diags, ds...)
			if root == nil {
				continue
			}
			rk := typeKey(root.Type) + "|" + root.Route
			// Record the Mount-call package for every call that maps to
			// this root — even the dedup-collapsed ones — so a root
			// mounted by two binaries is attributed to both.
			mountPkgs[rk] = append(mountPkgs[rk], normalizePkgPath(pkg.PkgPath))
			if _, ok := roots[rk]; ok {
				continue
			}
			roots[rk] = root
			tree.Roots = append(tree.Roots, root)
		}
	}

	closures := mainClosures(pkgs)
	for rk, root := range roots {
		root.binaries = binariesFor(mountPkgs[rk], closures)
	}
	return tree, diags
}

// mainClosures maps each main package's normalised import path to the
// set of package paths reachable from it (including itself). A Mount
// call in package P belongs to binary M iff P is in M's closure.
func mainClosures(pkgs []*packages.Package) map[string]map[string]bool {
	closures := map[string]map[string]bool{}
	for _, p := range pkgs {
		if p == nil || p.Name != "main" {
			continue
		}
		mainPath := normalizePkgPath(p.PkgPath)
		closure := closures[mainPath]
		if closure == nil {
			closure = map[string]bool{}
			closures[mainPath] = closure
		}
		// A main package may surface under several test variants; union
		// their closures under the one normalised path.
		collectImports(p, closure)
	}
	return closures
}

// collectImports records p and every package transitively imported by
// p into seen, keyed by normalised import path.
func collectImports(p *packages.Package, seen map[string]bool) {
	np := normalizePkgPath(p.PkgPath)
	if seen[np] {
		return
	}
	seen[np] = true
	for _, imp := range p.Imports {
		collectImports(imp, seen)
	}
}

// binariesFor returns the set of main-package paths whose closure
// reaches any of the given Mount-call packages. Returns nil when no
// main package reaches them (library-only analysis), so such roots
// fall into the shared default binary group during ambiguity checks.
func binariesFor(mountPkgPaths []string, closures map[string]map[string]bool) map[string]bool {
	bins := map[string]bool{}
	for _, mp := range mountPkgPaths {
		for mainPath, closure := range closures {
			if closure[mp] {
				bins[mainPath] = true
			}
		}
	}
	if len(bins) == 0 {
		return nil
	}
	return bins
}

// normalizePkgPath strips go/packages test-variant decorations so a
// package's source-level import path is stable across the regular,
// in-test, and external-test compilations packages.Load(Tests:true)
// produces ("foo [foo.test]" and "foo.test" both normalise to "foo").
func normalizePkgPath(p string) string {
	if i := strings.IndexByte(p, ' '); i >= 0 {
		p = p[:i]
	}
	return strings.TrimSuffix(p, ".test")
}

// buildRoot constructs a PageNode tree starting at the page argument
// of a single Mount call. Returns nil when the page argument is not a
// named struct.
func buildRoot(pkg *packages.Package, m mountCall) (*PageNode, []Diagnostic) {
	var diags []Diagnostic
	pageType, ok := resolveNamedStruct(pkg.TypesInfo.TypeOf(m.PageArg))
	if !ok {
		diags = append(diags, Diagnostic{
			Pos: pkg.Fset.Position(m.PageArg.Pos()),
			Message: fmt.Sprintf("[mount] Mount page argument has non-struct type %s; skipping",
				typeString(pkg.TypesInfo.TypeOf(m.PageArg))),
		})
		return nil, diags
	}
	_, route, _ := parseTag(m.Route)
	root := &PageNode{
		Name:    pageType.Obj().Name(),
		Type:    pageType,
		TypePos: m.PageArg.Pos(),
		Route:   route,
		Method:  methodAll,
		Methods: map[string]*types.Func{},
	}
	root.FullRoute = root.Route
	if root.FullRoute == "" {
		root.FullRoute = "/"
	}
	collectMethods(pageType, root)
	populateChildren(pkg, root, map[*types.Named]bool{pageType: true}, &diags)
	return root, diags
}

// populateChildren walks the struct fields of node.Type, picking up
// every `route:"..."` tag and recursing into the child's struct type.
// seen guards against cycles introduced by recursive page types.
func populateChildren(pkg *packages.Package, node *PageNode, seen map[*types.Named]bool, diags *[]Diagnostic) {
	st, ok := node.Type.Underlying().(*types.Struct)
	if !ok {
		return
	}

	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		tag := reflect.StructTag(st.Tag(i)).Get("route")
		if tag == "" {
			continue
		}
		childType, ok := resolveNamedStruct(field.Type())
		if !ok {
			*diags = append(*diags, Diagnostic{
				Pos: pkg.Fset.Position(field.Pos()),
				Message: fmt.Sprintf(
					"[mount] field %s.%s has route tag but type %s is not a named struct; skipping",
					node.Type.Obj().Name(), field.Name(), field.Type()),
			})
			continue
		}
		if seen[childType] {
			continue
		}
		method, route, _ := parseTag(tag)
		child := &PageNode{
			Name:    field.Name(),
			Type:    childType,
			TypePos: field.Pos(),
			Route:   route,
			Method:  method,
			Parent:  node,
			Methods: map[string]*types.Func{},
		}
		child.FullRoute = path.Join(node.FullRoute, child.Route)
		collectMethods(childType, child)
		// extend seen with the child so recursive types don't loop, but
		// each branch gets its own copy so siblings aren't blocked by
		// each other.
		seenCopy := make(map[*types.Named]bool, len(seen)+1)
		for k, v := range seen {
			seenCopy[k] = v
		}
		seenCopy[childType] = true
		populateChildren(pkg, child, seenCopy, diags)
		node.Children = append(node.Children, child)
	}
}

// typeKey returns a stable cross-package identifier for a named
// type. The same source declaration loaded under different package
// variants (regular vs. internal-test vs. external-test) produces
// distinct *types.Named pointers, so we can't use pointer-equality
// for type matching. The qualified path is stable.
func typeKey(named *types.Named) string {
	obj := named.Obj()
	if obj == nil {
		return ""
	}
	if obj.Pkg() == nil {
		return obj.Name()
	}
	return obj.Pkg().Path() + "." + obj.Name()
}

// resolveNamedStruct peels a single layer of pointer indirection and
// returns the underlying *types.Named iff it backs a struct type.
func resolveNamedStruct(t types.Type) (*types.Named, bool) {
	if t == nil {
		return nil, false
	}
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return nil, false
	}
	if _, ok := named.Underlying().(*types.Struct); !ok {
		return nil, false
	}
	return named, true
}

// collectMethods enumerates every non-promoted method on both the
// value and pointer receiver of named into node.Methods. Methods
// reached via an embedded field (selection index length > 1) are
// skipped to match the runtime's isPromotedMethod rule.
func collectMethods(named *types.Named, node *PageNode) {
	if node.Methods == nil {
		node.Methods = map[string]*types.Func{}
	}
	add := func(m *types.Func, index []int) {
		if len(index) > 1 {
			return
		}
		if _, dup := node.Methods[m.Name()]; dup {
			return
		}
		node.Methods[m.Name()] = m
	}
	for _, t := range []types.Type{named, types.NewPointer(named)} {
		mset := types.NewMethodSet(t)
		for i := 0; i < mset.Len(); i++ {
			sel := mset.At(i)
			fn, ok := sel.Obj().(*types.Func)
			if !ok {
				continue
			}
			add(fn, sel.Index())
		}
	}
}

// parseTag mirrors the runtime parseTag in parse.go. It splits the
// route tag into [METHOD] /path [Title] and defaults to method=ALL when
// the first field is not a recognised HTTP verb.
func parseTag(route string) (method, urlPath, title string) {
	method = methodAll
	parts := strings.Fields(route)
	switch len(parts) {
	case 0:
		return method, "/", ""
	case 1:
		return method, parts[0], ""
	}
	if isHTTPMethod(strings.ToUpper(parts[0])) {
		return strings.ToUpper(parts[0]), parts[1], strings.Join(parts[2:], " ")
	}
	return method, parts[0], strings.Join(parts[1:], " ")
}

const methodAll = "ALL"

var validMethods = []string{
	"GET", "HEAD", "POST", "PUT", "PATCH",
	"DELETE", "CONNECT", "OPTIONS", "TRACE", methodAll,
}

func isHTTPMethod(s string) bool {
	for _, m := range validMethods {
		if s == m {
			return true
		}
	}
	return false
}

// typeString gives a short, diagnostic-friendly type rendering.
func typeString(t types.Type) string {
	if t == nil {
		return "<nil>"
	}
	return t.String()
}

// callFromPackage reports whether call is a call to the named function
// from structpagesPkg. It tolerates dot imports and renamed imports by
// resolving the callee through TypesInfo.
func callFromPackage(pkg *packages.Package, call *ast.CallExpr, fnName string) bool {
	if pkg.TypesInfo == nil {
		return false
	}
	fn := calleeFunc(pkg.TypesInfo, call)
	if fn == nil {
		return false
	}
	if fn.Pkg() == nil || fn.Pkg().Path() != structpagesPkg {
		return false
	}
	return fn.Name() == fnName
}

// calleeFunc resolves the callee of a CallExpr to its *types.Func,
// or returns nil if the call cannot be resolved (e.g., calls through
// interface values).
func calleeFunc(info *types.Info, call *ast.CallExpr) *types.Func {
	var ident *ast.Ident
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		ident = fun
	case *ast.SelectorExpr:
		ident = fun.Sel
	default:
		return nil
	}
	if ident == nil {
		return nil
	}
	use, ok := info.Uses[ident]
	if !ok {
		return nil
	}
	fn, _ := use.(*types.Func)
	return fn
}
