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
	"maps"
	"path"
	"reflect"
	"sort"
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
	// reach is the set of packages compiled into any of binaries: the
	// code that can run while this root's tree serves a request. Nil
	// when binaries is nil. Populated only on root nodes.
	reach map[string]bool
	// prod marks a root reached by a real (non-test) main package.
	// Diagnostics prefer it, since a production tree is complete while
	// test trees often stub whole sections out. Root nodes only.
	prod bool
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

// BuildTree finds every structpages.Mount (or Parse) call across the supplied
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
	posRoot := map[string]string{}    // Mount-call position -> root key ("" when skipped)
	roots := map[string]*PageNode{}   // root key -> kept root
	mountIDs := map[string][]string{} // root key -> Mount-call package variant IDs

	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for _, m := range findMounts(pkg) {
			pos := pkg.Fset.Position(m.Call.Pos())
			key := fmt.Sprintf("%s:%d:%d", pos.Filename, pos.Line, pos.Column)
			// Record the Mount-call package variant for every call that
			// maps to a root — the per-variant repeats of one call and
			// other calls collapsed onto the same root alike — so the
			// root is attributed to every binary that compiles one in.
			if rk, ok := posRoot[key]; ok {
				if rk != "" {
					mountIDs[rk] = append(mountIDs[rk], pkg.ID)
				}
				continue
			}
			root, ds := buildRoot(pkg, m)
			diags = append(diags, ds...)
			if root == nil {
				posRoot[key] = ""
				continue
			}
			rk := typeKey(root.Type) + "|" + root.Route
			posRoot[key] = rk
			mountIDs[rk] = append(mountIDs[rk], pkg.ID)
			if _, ok := roots[rk]; ok {
				continue
			}
			roots[rk] = root
			tree.Roots = append(tree.Roots, root)
		}
	}

	closures := mainClosures(pkgs)
	prodMains := realMains(pkgs)
	for rk, root := range roots {
		root.binaries = binariesFor(mountIDs[rk], closures)
		for b := range root.binaries {
			if root.reach == nil {
				root.reach = map[string]bool{}
			}
			maps.Copy(root.reach, closures[b].pkgs)
			root.prod = root.prod || prodMains[b]
		}
	}
	return tree, diags
}

// rootsFor returns the roots whose tree can serve a request that runs
// code in pkgPath: those reached by a binary that compiles pkgPath in,
// plus roots with no binary attribution (library-only analysis), which
// apply everywhere. Production roots come first.
//
// At runtime URLFor, ID and Ref resolve against the one tree serving
// the request, so a call site is checked tree by tree rather than
// against every Mount in the module (see checkCtx.inSomeTree).
func (t *PageTree) rootsFor(pkgPath string) []*PageNode {
	if t == nil {
		return nil
	}
	np := normalizePkgPath(pkgPath)
	var out []*PageNode
	for _, r := range t.Roots {
		if r.binaries == nil || r.reach[np] {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].prod && !out[j].prod })
	return out
}

// realMains returns the normalised paths of the non-test main packages
// in pkgs. A synthesised test main ("foo.test") normalises onto its
// package path, so it is recognised by its un-normalised path.
func realMains(pkgs []*packages.Package) map[string]bool {
	out := map[string]bool{}
	for _, p := range pkgs {
		if p != nil && p.Name == "main" && !strings.HasSuffix(p.PkgPath, ".test") {
			out[normalizePkgPath(p.PkgPath)] = true
		}
	}
	return out
}

// binary is what one main package compiles in.
type binary struct {
	// ids are the package variant IDs ("foo", "foo [foo.test]"). A
	// Mount call belongs to the binary iff its variant is here: a call
	// in foo's _test.go file exists only in "foo [foo.test]", not in
	// the plain "foo" every other binary imports.
	ids map[string]bool
	// pkgs are the normalised paths of ids: the code that can run in
	// the binary, keyed the way an analysis pass names its package.
	pkgs map[string]bool
}

// mainClosures maps each main package's normalised import path to what
// it compiles in. A main package surfacing under several variants (its
// test binary) is unioned under the one path.
func mainClosures(pkgs []*packages.Package) map[string]*binary {
	closures := map[string]*binary{}
	for _, p := range pkgs {
		if p == nil || p.Name != "main" {
			continue
		}
		mainPath := normalizePkgPath(p.PkgPath)
		bin := closures[mainPath]
		if bin == nil {
			bin = &binary{ids: map[string]bool{}, pkgs: map[string]bool{}}
			closures[mainPath] = bin
		}
		collectImports(p, bin)
	}
	return closures
}

// collectImports records p and every package variant it transitively
// imports into bin.
func collectImports(p *packages.Package, bin *binary) {
	if bin.ids[p.ID] {
		return
	}
	bin.ids[p.ID] = true
	bin.pkgs[normalizePkgPath(p.PkgPath)] = true
	for _, imp := range p.Imports {
		collectImports(imp, bin)
	}
}

// binariesFor returns the set of main-package paths that compile in any
// of the given Mount-call package variants. Returns nil when no main
// package does (library-only analysis); such roots apply to every
// package (see PageTree.rootsFor).
func binariesFor(mountIDs []string, closures map[string]*binary) map[string]bool {
	bins := map[string]bool{}
	for _, id := range mountIDs {
		for mainPath, bin := range closures {
			if bin.ids[id] {
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
	argType := pkg.TypesInfo.TypeOf(m.PageArg)
	pageType, ok := resolveNamedStruct(argType)
	if !ok {
		// Two shapes aren't misuses, so they are skipped quietly: an
		// interface-typed argument is a helper forwarding its caller's
		// tree (`func mount(pages any)`), which isn't knowable here, and
		// an anonymous struct (`Parse(struct{}{}, …)`) is test
		// scaffolding no call site can name.
		if argType != nil && (types.IsInterface(argType) || isAnonymousStruct(argType)) {
			return nil, nil
		}
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

// isAnonymousStruct reports whether t is an unnamed struct type or a
// pointer to one.
func isAnonymousStruct(t types.Type) bool {
	t = types.Unalias(t)
	if ptr, ok := t.(*types.Pointer); ok {
		t = types.Unalias(ptr.Elem())
	}
	_, ok := t.(*types.Struct)
	return ok
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

// resolveNamedStruct peels aliases and a single layer of pointer
// indirection and returns the underlying *types.Named iff it backs a
// struct type. A field declared with an alias (`type API = api.Root`)
// has a *types.Alias type, which the runtime sees straight through.
func resolveNamedStruct(t types.Type) (*types.Named, bool) {
	if t == nil {
		return nil, false
	}
	t = types.Unalias(t)
	if ptr, ok := t.(*types.Pointer); ok {
		t = types.Unalias(ptr.Elem())
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
