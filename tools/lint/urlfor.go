package lint

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"
)

// checkURLFor validates a `structpages.URLFor(ctx, page, args...)`
// call. It performs:
//
//   - typed page lookup (bare typed value or `[]any{...}` chain)
//   - chain step ordering rules (no Ref/predicate past index 0; no
//     typed value after a string fragment)
//   - param key vs route-placeholder check when a literal
//     map[string]any or a variadic key/value list is supplied
func checkURLFor(ctx *checkCtx, call *ast.CallExpr) {
	if len(call.Args) < 2 {
		return
	}
	if len(ctx.roots) == 0 {
		return
	}
	pageArg := call.Args[1]
	args := call.Args[2:]

	node, frag := resolvePageArgAndFragment(ctx, pageArg)
	if node != nil {
		checkParamMap(ctx, node, frag, args)
		if !call.Ellipsis.IsValid() {
			checkParamPairs(ctx, node, frag, args)
		}
	}
}

// resolvePageArgAndFragment is like resolvePageArg but additionally
// returns the concatenated trailing string fragments of an []any
// chain. Those fragments contribute placeholders to the URL
// pattern (e.g., "?preset={preset}" supplies the {preset} key);
// checkParamMap needs them to avoid false-positive "unknown param"
// diagnostics.
func resolvePageArgAndFragment(ctx *checkCtx, expr ast.Expr) (*PageNode, string) {
	expr = unparen(expr)
	if comp, ok := asAnySliceLiteral(ctx, expr); ok {
		return resolveChainLiteralWithFragment(ctx, comp)
	}
	return resolvePageArg(ctx, expr), ""
}

// resolvePageArg classifies the page argument and resolves it to a
// PageNode. It returns nil on any unresolvable case (after emitting
// diagnostics) or when the argument is non-literal (silent skip).
func resolvePageArg(ctx *checkCtx, expr ast.Expr) *PageNode {
	// Unwrap parens.
	expr = unparen(expr)

	// []any{...} composite literal: chain + fragments
	if comp, ok := asAnySliceLiteral(ctx, expr); ok {
		return resolveChainLiteral(ctx, comp)
	}

	// Ref(...) — validate at the URLFor call-site with URL-context
	// semantics (strict anchoring). The Ref-conversion check
	// (checkRefConversion) only catches refs invalid in *every*
	// context, so context-specific failures (URL-only callers
	// feeding a depth-2 anchor) must be caught here.
	if call, ok := expr.(*ast.CallExpr); ok && isRefConversion(ctx.pass.TypesInfo, call) {
		if s, ok := stringConstantFromPass(ctx, call.Args[0]); ok {
			return resolveRefAtCallSite(ctx, call.Args[0].Pos(), s, false)
		}
		return nil
	}

	// String literal (or named string constant) at the page slot is
	// sugar for Ref(...): URLFor(ctx, "Admin.Settings"). Validate it
	// the same way Ref conversions are validated.
	if s, ok := stringConstantFromPass(ctx, expr); ok {
		return resolveRef(ctx, expr.Pos(), s, false)
	}

	// Bare typed value: composite literal, &composite, or named value.
	t := normalisedPageType(ctx.pass.TypesInfo, expr)
	if t == nil {
		return nil
	}
	return resolveByType(ctx, expr.Pos(), t)
}

// resolveByType looks up a named struct type in the page tree. It
// errors with a suggested chain form on ambiguity, mirroring the
// runtime "ambiguous: type X matches N nodes" message.
//
// The runtime resolves against the one tree serving the request, so
// ambiguity is judged within a tree: callers run under
// checkCtx.inSomeTree, which narrows ctx.roots to one root at a time.
// A page mounted once in each of two trees — a standalone preview
// binary plus the same tree embedded in an app (issue #22), or a test
// tree mounting a section at "/" — is unambiguous in each. The same
// path reached twice (a re-mounted sub-tree at the path it already
// lives at) collapses by FullRoute and stays unambiguous.
func resolveByType(ctx *checkCtx, pos token.Pos, named *types.Named) *PageNode {
	wantKey := typeKey(named)
	var matches []*PageNode
	for _, root := range ctx.roots {
		root.All(func(n *PageNode) bool {
			if typeKey(n.Type) == wantKey {
				matches = append(matches, n)
			}
			return true
		})
	}
	if len(matches) == 0 {
		ctx.report(pos, "urlfor",
			"URLFor: no page mounted for type %s", named.Obj().Name())
		return nil
	}
	if routes := distinctRoutes(matches); len(routes) > 1 {
		ctx.report(pos, "urlfor",
			"URLFor: type %s is ambiguous (mounted at %s); disambiguate with "+
				"[]any{Parent{}, %s{}} chain or Ref(\"Parent.Field\")",
			named.Obj().Name(), strings.Join(routes, ", "), named.Obj().Name())
		return nil
	}
	return matches[0]
}

// distinctRoutes returns the sorted, de-duplicated FullRoutes of nodes.
func distinctRoutes(nodes []*PageNode) []string {
	set := map[string]bool{}
	for _, n := range nodes {
		set[n.FullRoute] = true
	}
	routes := make([]string, 0, len(set))
	for r := range set {
		routes = append(routes, r)
	}
	sort.Strings(routes)
	return routes
}

// resolveChainLiteralWithFragment is the chain-form walker that
// returns the concatenated trailing string fragments alongside the
// leaf node. Wraps resolveChainLiteral; callers that don't care
// about the fragment can use resolveChainLiteral directly.
func resolveChainLiteralWithFragment(ctx *checkCtx, comp *ast.CompositeLit) (*PageNode, string) {
	node := resolveChainLiteral(ctx, comp)
	if node == nil {
		return nil, ""
	}
	var frag strings.Builder
	for _, e := range comp.Elts {
		s, ok := plainStringConstant(ctx, e)
		if !ok {
			continue
		}
		frag.WriteString(s)
	}
	return node, frag.String()
}

// resolveChainLiteral handles the `[]any{step0, step1, ..., "?fragment"}`
// shape. Steps before the first string element are chain steps; the
// rest are appended literals. As at runtime (`part.(string)`), only a
// plain string is a fragment: a Ref constant is a chain step.
func resolveChainLiteral(ctx *checkCtx, comp *ast.CompositeLit) *PageNode {
	chainEnd := len(comp.Elts)
	for i, e := range comp.Elts {
		if _, ok := plainStringConstant(ctx, e); ok {
			chainEnd = i
			break
		}
	}
	chain := comp.Elts[:chainEnd]
	fragments := comp.Elts[chainEnd:]

	if len(chain) == 0 {
		ctx.report(comp.Pos(), "urlfor",
			"URLFor: []any chain has no page step before string fragments")
		return nil
	}

	// Step 0: accepts typed value or Ref or predicate.
	node := resolveChainStep(ctx, chain[0], true)
	if node == nil {
		return nil
	}

	// Subsequent steps: typed value only, descended by child type.
	for i, step := range chain[1:] {
		s := unparen(step)
		if call, ok := s.(*ast.CallExpr); ok && isRefConversion(ctx.pass.TypesInfo, call) {
			ctx.report(s.Pos(), "urlfor",
				"URLFor: chain step %d is a Ref; Ref is only valid as the first chain step",
				i+1)
			return nil
		}
		t := normalisedPageType(ctx.pass.TypesInfo, s)
		if t == nil {
			return nil
		}
		next := descendByType(ctx, node, s.Pos(), t)
		if next == nil {
			return nil
		}
		node = next
	}

	// Phase 2: ensure no typed values appear after a string fragment.
	for i, frag := range fragments {
		if _, ok := plainStringConstant(ctx, frag); ok {
			continue
		}
		ctx.report(frag.Pos(), "urlfor",
			"URLFor: typed value at slice position %d follows a string fragment; "+
				"chain steps must all come before any string fragment",
			chainEnd+i)
		return nil
	}
	return node
}

func resolveChainStep(ctx *checkCtx, step ast.Expr, allowRef bool) *PageNode {
	s := unparen(step)
	// A Ref step is either a conversion, Ref("A.B"), or a Ref-typed
	// constant such as `const Compare = structpages.Ref("A.B")`. Its
	// string is validated where it is written (checkRefConversion), so
	// it resolves quietly here.
	refArg := s
	if call, ok := s.(*ast.CallExpr); ok && isRefConversion(ctx.pass.TypesInfo, call) {
		refArg = call.Args[0]
	} else if !isRefType(ctx.pass.TypesInfo.TypeOf(s)) {
		refArg = nil
	}
	if refArg != nil {
		if !allowRef {
			ctx.report(s.Pos(), "urlfor", "URLFor: Ref not allowed in this position")
			return nil
		}
		if str, ok := stringConstantFromPass(ctx, refArg); ok {
			save := ctx.silent
			ctx.silent = true
			node := resolveRef(ctx, refArg.Pos(), str, false)
			ctx.silent = save
			return node
		}
		return nil
	}
	t := normalisedPageType(ctx.pass.TypesInfo, s)
	if t == nil {
		return nil
	}
	return resolveByType(ctx, s.Pos(), t)
}

func descendByType(ctx *checkCtx, parent *PageNode, pos token.Pos, want *types.Named) *PageNode {
	wantKey := typeKey(want)
	var matches []*PageNode
	for _, c := range parent.Children {
		if typeKey(c.Type) == wantKey {
			matches = append(matches, c)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0]
	case 0:
		names := make([]string, len(parent.Children))
		for i, c := range parent.Children {
			names[i] = fmt.Sprintf("%s (%s)", c.Name, c.Type.Obj().Name())
		}
		ctx.report(pos, "urlfor",
			"URLFor chain: parent %s has no child of type %s; available: %s",
			parent.Name, want.Obj().Name(), strings.Join(names, ", "))
		return nil
	default:
		fields := make([]string, len(matches))
		for i, m := range matches {
			fields[i] = m.Name
		}
		sort.Strings(fields)
		ctx.report(pos, "urlfor",
			"URLFor chain: parent %s has multiple children of type %s: %s; use Ref(%q)",
			parent.Name, want.Obj().Name(), strings.Join(fields, ", "),
			parent.Name+"."+fields[0])
		return nil
	}
}

// normalisedPageType returns the *types.Named that backs expr, peeled
// of pointer indirection. Returns nil if expr is not a named struct
// (e.g. interface{}, an untyped value, a function, etc.).
func normalisedPageType(info *types.Info, expr ast.Expr) *types.Named {
	named, _ := resolveNamedStruct(info.TypeOf(expr))
	return named
}

// asAnySliceLiteral returns the composite literal if expr is an
// []any{...} literal — the canonical chain form. Other slice-of-any
// constructions are not flagged (they may be dynamic).
func asAnySliceLiteral(ctx *checkCtx, expr ast.Expr) (*ast.CompositeLit, bool) {
	comp, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil, false
	}
	t := ctx.pass.TypesInfo.TypeOf(comp)
	if t == nil {
		return nil, false
	}
	sl, ok := t.Underlying().(*types.Slice)
	if !ok {
		return nil, false
	}
	iface, ok := sl.Elem().Underlying().(*types.Interface)
	if !ok || !iface.Empty() {
		return nil, false
	}
	return comp, true
}

func unparen(expr ast.Expr) ast.Expr {
	for {
		p, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = p.X
	}
}
