package lint

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"
)

// checkIDOrIDTarget validates `structpages.ID(ctx, v)` and
// `structpages.IDTarget(ctx, v)`. Three cases:
//
//   - Method expression (`p.UserList`): the receiver type must be
//     mounted as a page.
//   - Ref string: must resolve as a page in the tree; if the ref is
//     qualified (Page.Method), the trailing segment must be a method
//     on that page; if it's simple, it must be a method on exactly
//     one page.
//   - Plain string literal: no check (literal id passes through).
//   - Standalone function: no check (allowed by runtime).
func checkIDOrIDTarget(ctx *checkCtx, call *ast.CallExpr, fnName string) {
	if len(call.Args) != 2 {
		return
	}
	if ctx.tree == nil || len(ctx.tree.Roots) == 0 {
		return
	}
	arg := unparen(call.Args[1])

	// String literal? No check.
	if _, ok := stringConstantFromPass(ctx, arg); ok {
		// But only if the argument's static type is plain string. If
		// it's a Ref (also a string under the hood), fall through.
		if t := ctx.pass.TypesInfo.TypeOf(arg); t != nil {
			if b, ok := t.Underlying().(*types.Basic); ok && b.Kind() == types.String && !isRefType(t) {
				return
			}
		}
	}

	// Ref(...) — validate as a method reference.
	if c, ok := arg.(*ast.CallExpr); ok && isRefConversion(ctx.pass.TypesInfo, c) {
		checkIDRef(ctx, c, fnName)
		return
	}

	// []any{...} chain form — leading typed values + trailing
	// string method name or method expression. Mirrors URLFor's
	// chain check shape.
	if comp, ok := asAnySliceLiteral(ctx, arg); ok {
		checkIDChain(ctx, comp, fnName)
		return
	}

	// Method expression or standalone function.
	checkIDFuncArg(ctx, arg, fnName)
}

// checkIDChain validates a []any composition argument to ID /
// IDTarget. Two terminal shapes:
//
//   - trailing string: method name to look up on the chain leaf
//   - trailing method expression: receiver type IS the leaf (or
//     matches the prior chain step's type — duplicate collapse)
func checkIDChain(ctx *checkCtx, comp *ast.CompositeLit, fnName string) {
	if len(comp.Elts) == 0 {
		ctx.report(comp.Pos(), "idfor",
			"%s: empty []any chain", fnName)
		return
	}
	elts := comp.Elts
	last := elts[len(elts)-1]
	chainSteps := elts[:len(elts)-1]

	// Determine method spec.
	var methodName string
	var methodPos token.Pos
	if s, ok := stringConstantFromPass(ctx, last); ok {
		if t := ctx.pass.TypesInfo.TypeOf(last); t != nil {
			if b, ok := t.Underlying().(*types.Basic); ok && b.Kind() == types.String && !isRefType(t) {
				methodName = s
				methodPos = last.Pos()
			} else {
				ctx.report(last.Pos(), "idfor",
					"%s: trailing []any element must be a plain method-name string or a method expression", fnName)
				return
			}
		}
	} else if sel, ok := lastAsMethodExpr(ctx, last); ok {
		methodName = sel.fnName
		methodPos = last.Pos()
		// Append the method expression's receiver type as an
		// implicit chain step unless the prior step already
		// names the same type (duplicate collapse).
		dup := false
		if n := len(chainSteps); n > 0 {
			if t := normalisedPageType(ctx.pass.TypesInfo, chainSteps[n-1]); t != nil {
				if typeKey(t) == typeKey(sel.recvType) {
					dup = true
				}
			}
		}
		if !dup {
			chainSteps = append(chainSteps, last)
		}
	} else {
		ctx.report(last.Pos(), "idfor",
			"%s: trailing []any element must be a method-name string or a method expression", fnName)
		return
	}

	if len(chainSteps) == 0 {
		ctx.report(comp.Pos(), "idfor",
			"%s: []any chain has no page context", fnName)
		return
	}

	leaf := resolveChainLiteralSteps(ctx, chainSteps)
	if leaf == nil {
		return
	}
	if _, ok := leaf.Methods[methodName]; !ok {
		methods := mapMethodNames(leaf.Methods)
		ctx.report(methodPos, "idfor",
			"%s: method %q not found on chain leaf %q; available: %s",
			fnName, methodName, leaf.Name, strings.Join(methods, ", "))
	}
}

// methodExprInfo bundles a method expression's static info pulled
// from go/types so callers can avoid re-walking the AST.
type methodExprInfo struct {
	fnName   string
	recvType *types.Named
}

// lastAsMethodExpr reports whether expr is a selector that names a
// method on a struct receiver, and returns the receiver type + the
// method name.
func lastAsMethodExpr(ctx *checkCtx, expr ast.Expr) (methodExprInfo, bool) {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return methodExprInfo{}, false
	}
	use, ok := ctx.pass.TypesInfo.Uses[sel.Sel]
	if !ok {
		return methodExprInfo{}, false
	}
	fn, ok := use.(*types.Func)
	if !ok {
		return methodExprInfo{}, false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return methodExprInfo{}, false
	}
	named, ok := resolveNamedStruct(sig.Recv().Type())
	if !ok {
		return methodExprInfo{}, false
	}
	return methodExprInfo{fnName: fn.Name(), recvType: named}, true
}

// resolveChainLiteralSteps walks the chain steps (mirrors
// resolveChainLiteral in urlfor.go but doesn't accept string
// fragments — strings are method specs for ID, validated by the
// caller). On error, reports and returns nil.
func resolveChainLiteralSteps(ctx *checkCtx, steps []ast.Expr) *PageNode {
	if len(steps) == 0 {
		return nil
	}
	node := resolveChainStep(ctx, steps[0], true)
	if node == nil {
		return nil
	}
	for i, step := range steps[1:] {
		s := unparen(step)
		if call, ok := s.(*ast.CallExpr); ok && isRefConversion(ctx.pass.TypesInfo, call) {
			ctx.report(s.Pos(), "idfor",
				"ID/IDTarget: chain step %d is a Ref; Ref is only valid as the first chain step",
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
	return node
}

// checkIDRef validates a Ref passed to ID/IDTarget. Supported shapes:
//   - "Page.Method" — page must exist and method must be on it
//   - "Method" — must be on exactly one mounted page
func checkIDRef(ctx *checkCtx, call *ast.CallExpr, fnName string) {
	if len(call.Args) != 1 {
		return
	}
	ref, ok := stringConstantFromPass(ctx, call.Args[0])
	if !ok {
		return
	}
	pos := call.Args[0].Pos()
	if ref == "" {
		ctx.report(pos, "idfor", "%s: empty Ref", fnName)
		return
	}
	if strings.Contains(ref, ".") {
		idx := strings.LastIndex(ref, ".")
		pagePath := ref[:idx]
		method := ref[idx+1:]
		if pagePath == "" || method == "" {
			ctx.report(pos, "idfor", "%s: malformed qualified Ref %q", fnName, ref)
			return
		}
		node := resolveRef(ctx, pos, ref, true)
		if node == nil {
			return
		}
		if _, ok := node.Methods[method]; !ok {
			methods := mapMethodNames(node.Methods)
			ctx.report(pos, "idfor",
				"%s: Ref %q: method %q not found on page %q; available: %s",
				fnName, ref, method, node.Name, strings.Join(methods, ", "))
		}
		return
	}
	// Simple method name — must be on exactly one mounted page.
	var matches []*PageNode
	for _, root := range ctx.tree.Roots {
		root.All(func(n *PageNode) bool {
			if _, ok := n.Methods[ref]; ok {
				matches = append(matches, n)
			}
			return true
		})
	}
	switch len(matches) {
	case 1:
		return
	case 0:
		ctx.report(pos, "idfor",
			"%s: Ref %q: method not found on any mounted page", fnName, ref)
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.Name
		}
		sort.Strings(names)
		ctx.report(pos, "idfor",
			"%s: Ref %q: method found on multiple pages: %s; use qualified form like %q",
			fnName, ref, strings.Join(names, ", "), names[0]+"."+ref)
	}
}

// checkIDFuncArg validates a method-expression (or function) argument
// to ID/IDTarget. For a method expression p.Method, the receiver type
// must be mounted as a page. Standalone functions are accepted
// without check.
func checkIDFuncArg(ctx *checkCtx, expr ast.Expr, fnName string) {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return // Not a method expression: standalone function or variable.
	}
	use, ok := ctx.pass.TypesInfo.Uses[sel.Sel]
	if !ok {
		return
	}
	fn, ok := use.(*types.Func)
	if !ok {
		return
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return
	}
	recv := sig.Recv().Type()
	named, ok := resolveNamedStruct(recv)
	if !ok {
		return
	}
	if !ctx.isMounted(named) {
		ctx.report(expr.Pos(), "idfor",
			"%s: method expression %s.%s: receiver type %s is not mounted as a page",
			fnName, named.Obj().Name(), fn.Name(), named.Obj().Name())
	}
}

// isMounted reports whether named appears as the Type of any node in
// the tree.
func (ctx *checkCtx) isMounted(named *types.Named) bool {
	key := typeKey(named)
	for _, root := range ctx.tree.Roots {
		found := false
		root.All(func(n *PageNode) bool {
			if typeKey(n.Type) == key {
				found = true
				return false
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}

// isRefType reports whether t is structpages.Ref (a defined string
// type), as opposed to a plain string.
func isRefType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	return obj.Pkg().Path() == structpagesPkg && obj.Name() == "Ref"
}

func mapMethodNames(m map[string]*types.Func) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
