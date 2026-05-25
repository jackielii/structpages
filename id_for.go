package structpages

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// ID generates a raw HTML ID for a component method (without "#" prefix).
// Use this for HTML id attributes.
//
// Parameters:
//   - ctx: Context containing parseContext (required for method expressions and Ref)
//   - v: One of:
//   - Method expression (p.UserList) - generates ID from page and method name
//   - Ref type (structpages.Ref("PageName.MethodName")) - looks up page/method dynamically
//   - Plain string ("my-custom-id") - returned as-is
//
// Example:
//
//	<div id={ structpages.ID(ctx, p.UserList) }>
//	// → <div id="team-management-view-user-list">
//
//	<div id={ structpages.ID(ctx, UserStatsWidget) }>
//	// → <div id="user-stats-widget"> (no page prefix for standalone functions)
//
//	<div id={ structpages.ID(ctx, "my-custom-id") }>
//	// → <div id="my-custom-id">
//
// Returns an error if parseContext is not found in the provided context.
func ID(ctx context.Context, v any) (string, error) {
	// Extract parseContext - REQUIRED
	pc := pcCtx.Value(ctx)
	if pc == nil {
		return "", errors.New("parseContext not found in context - ID must be called within a page handler or template")
	}

	return idFor(pc, currentPageCtx.Value(ctx), v, true)
}

// IDTarget generates a CSS selector (with "#" prefix) for a component method.
// Use this for HTMX hx-target attributes.
//
// Parameters:
//
//   - ctx: Context containing parseContext (required for method expressions and Ref)
//   - v: One of:
//   - Method expression (p.UserList) - generates selector from page and method name
//   - Ref type (structpages.Ref("PageName.MethodName")) - looks up page/method dynamically
//   - string ("body" or "#my-custom-id") - returned as-is
//
// Example:
//
//	<button hx-target={ structpages.IDTarget(ctx, p.UserList) }>
//	// → <button hx-target="#team-management-view-user-list">
//
//	<button hx-target={ structpages.IDTarget(ctx, UserStatsWidget) }>
//	// → <button hx-target="#user-stats-widget"> (no page prefix for standalone functions)
//
//	<button hx-target={ structpages.IDTarget(ctx, "body") }>
//	// → <button hx-target="body">
//
// Returns an error if parseContext is not found in the provided context.
func IDTarget(ctx context.Context, v any) (string, error) {
	// Extract parseContext - REQUIRED
	pc := pcCtx.Value(ctx)
	if pc == nil {
		return "", errors.New("parseContext not found in context - IDTarget must be called within a page handler or template")
	}

	return idFor(pc, currentPageCtx.Value(ctx), v, false)
}

// idFor generates the ID string based on the provided value
// (method expression, Ref, plain string, or standalone function).
//
// When currentPage is non-nil and the method expression's receiver
// type matches the current page's type, the id is derived from the
// current page's field Name. This guarantees self-render produces
// the right id when the same struct type is mounted under multiple
// parents with different field names (topologies C and D from the
// design discussion). When currentPage is nil or the receiver type
// doesn't match, the resolver falls back to a global tree lookup —
// matches the existing behavior used by sp.ID / sp.IDTarget and
// cross-page renders.
func idFor(pc *parseContext, currentPage *PageNode, v any, rawID bool) (string, error) {
	methodExpr := v

	// Handle Ref type for dynamic method references
	if ref, ok := methodExpr.(Ref); ok {
		return idForRef(pc, string(ref), rawID)
	}

	// Handle []any chain form: typed chain steps + trailing method spec.
	// Parallels URLFor's []any{Parent{}, Leaf{}, "?frag"} composition,
	// but the trailing element is a method name string or a method
	// expression whose receiver type is the chain leaf.
	if parts, ok := methodExpr.([]any); ok {
		return idForChain(pc, currentPage, parts, rawID)
	}

	// Handle plain string as literal ID - return as-is
	if str, ok := methodExpr.(string); ok {
		return str, nil
	}

	// Validate that we have a method expression (function) before extracting method info
	rv := reflect.ValueOf(methodExpr)
	if rv.Kind() != reflect.Func {
		return "", fmt.Errorf("unsupported type %T: expected method expression, Ref, or string", methodExpr)
	}

	// Extract all method info (handles methods and standalone functions)
	info, err := extractMethodInfo(methodExpr)
	if err != nil {
		return "", err
	}

	// Find the page node (for methods only - functions don't need one)
	var pageName string
	if !info.isFunction {
		pn, err := resolvePageForMethod(pc, currentPage, info)
		if err != nil {
			return "", fmt.Errorf("cannot find page for method expression: %w", err)
		}
		pageName = pn.Name
	}
	// Standalone functions have no page prefix (they're shared components)

	// Build ID
	id := buildID(pageName, info.methodName, rawID)
	return id, nil
}

// idForChain resolves the []any composition form for ID/IDTarget.
// The trailing element is the method spec:
//
//   - string — used as the method name on the chain leaf's type.
//   - method expression / method value — its receiver type IS the
//     chain leaf, and its name supplies the method. If the prior
//     chain step's type matches the receiver type, the method
//     expression's implicit leaf is collapsed (no duplicate descend).
//
// Leading typed values form the chain steps, resolved by the same
// rules URLFor's chain form uses (first step by type lookup,
// subsequent steps by child-type descent).
func idForChain(pc *parseContext, currentPage *PageNode, parts []any, rawID bool) (string, error) {
	if len(parts) == 0 {
		return "", errors.New("ID: empty []any chain")
	}
	last := parts[len(parts)-1]
	chainSteps := parts[:len(parts)-1]

	var methodName string
	var fromMethodExpr *methodInfo

	switch t := last.(type) {
	case string:
		if t == "" {
			return "", errors.New("ID: empty method name in []any chain")
		}
		methodName = t
	case nil:
		return "", errors.New("ID: nil trailing element in []any chain")
	default:
		rv := reflect.ValueOf(last)
		if rv.Kind() != reflect.Func {
			return "", fmt.Errorf(
				"ID: trailing element of []any chain must be a method name string or method expression, got %T",
				last)
		}
		info, err := extractMethodInfo(last)
		if err != nil {
			return "", fmt.Errorf("ID: invalid method expression in chain: %w", err)
		}
		if info.isFunction {
			return "", errors.New("ID: trailing element is a standalone function; " +
				"chain form expects a method expression bound to a page type")
		}
		fromMethodExpr = info
		methodName = info.methodName
	}

	// When the trailing element is a method expression, its receiver
	// type is an implicit final chain step. Append it unless the
	// caller already wrote the type explicitly (e.g.,
	// []any{Parent{}, Leaf{}, leafPage.Method} where Leaf{} and
	// leafPage agree — collapse to a single descend rather than
	// double-stepping).
	if fromMethodExpr != nil {
		recv := fromMethodExpr.receiverType
		recvElem := recv
		if recvElem.Kind() == reflect.Pointer {
			recvElem = recvElem.Elem()
		}
		duplicate := false
		if n := len(chainSteps); n > 0 {
			lastStepType := reflect.TypeOf(chainSteps[n-1])
			if pointerType(lastStepType) == pointerType(recv) {
				duplicate = true
			}
		}
		if !duplicate {
			chainSteps = append(chainSteps, reflect.New(recvElem).Elem().Interface())
		}
	}

	if len(chainSteps) == 0 {
		return "", errors.New("ID: []any chain has no page context; " +
			"provide at least one typed chain step or use a method expression")
	}

	leaf, err := pc.resolveChain(chainSteps)
	if err != nil {
		return "", fmt.Errorf("ID: %w", err)
	}

	// Self-render override: if the resolved leaf type matches the
	// current page's type, prefer the current page's field name.
	// This mirrors the bare method-expression behavior from idFor.
	if currentPage != nil && pointerType(currentPage.Value.Type()) == pointerType(leaf.Value.Type()) {
		leaf = currentPage
	}

	leafType := pointerType(leaf.Value.Type())
	if _, ok := leafType.MethodByName(methodName); !ok {
		return "", fmt.Errorf("ID: method %q not found on chain leaf type %s",
			methodName, leafType.Elem().Name())
	}

	return buildID(leaf.Name, methodName, rawID), nil
}

// resolvePageForMethod picks the PageNode whose Name supplies the
// id prefix. It prefers the currentPage when its receiver type
// matches the method expression — that way a page rendering its
// own template gets the id for *its* mount, not whichever match
// tree-walk encounters first.
//
// For cross-page calls (no current page, or its type doesn't
// match), the resolver collects every mount of the receiver type.
// Identical mount field names produce identical ids and are
// silently collapsed (e.g. an entryPage mounted under three
// section roots all named "EntryDetail" — the user explicitly
// chose this shape). Divergent field names produce different ids;
// the resolver refuses to silently pick one and surfaces a
// disambiguation error instead.
func resolvePageForMethod(pc *parseContext, currentPage *PageNode, info *methodInfo) (*PageNode, error) {
	if currentPage != nil && pageNodeMatchesMethod(currentPage, info) {
		return currentPage, nil
	}
	matches := pc.collectPageNodesForMethod(info)
	switch len(matches) {
	case 0:
		if info.isBound {
			return nil, fmt.Errorf("no page node found with type name %q", info.receiverTypeName)
		}
		return nil, fmt.Errorf("no page node found for type %s", pointerType(info.receiverType).String())
	case 1:
		return matches[0], nil
	}
	first := matches[0]
	allSameName := true
	for _, m := range matches[1:] {
		if m.Name != first.Name {
			allSameName = false
			break
		}
	}
	if allSameName {
		return first, nil
	}
	// Build a useful error: list each distinct (mount, id) pair.
	type opt struct {
		name, route, id string
	}
	seen := map[string]bool{}
	var opts []opt
	for _, m := range matches {
		if seen[m.Name] {
			continue
		}
		seen[m.Name] = true
		opts = append(opts, opt{
			name:  m.Name,
			route: m.FullRoute(),
			id:    buildID(m.Name, info.methodName, true),
		})
	}
	descs := make([]string, len(opts))
	for i, o := range opts {
		descs[i] = fmt.Sprintf("%s at %s → %q", o.name, o.route, o.id)
	}
	return nil, fmt.Errorf(
		"ID: type %s is mounted under multiple fields producing different ids: %s; "+
			"disambiguate with the []any chain form, a Ref, or move the slot to a "+
			"standalone function",
		first.Value.Type().String(), strings.Join(descs, "; "))
}

// collectPageNodesForMethod returns every PageNode whose value type
// matches info's receiver. For bound method values (isBound), it
// additionally verifies the method exists on the type — matching
// the historical behavior of findPageNodeByTypeName. For unbound
// method expressions, the caller has already vouched for the
// method by writing the expression, so we trust it.
func (p *parseContext) collectPageNodesForMethod(info *methodInfo) []*PageNode {
	var out []*PageNode
	for node := range p.root.All() {
		nodeType := node.Value.Type()
		if info.isBound {
			nodeTypeName := nodeType.Name()
			if nodeType.Kind() == reflect.Pointer {
				nodeTypeName = nodeType.Elem().Name()
			}
			if nodeTypeName != info.receiverTypeName {
				continue
			}
			if _, found := nodeType.MethodByName(info.methodName); !found {
				continue
			}
		} else if pointerType(nodeType) != pointerType(info.receiverType) {
			continue
		}
		out = append(out, node)
	}
	return out
}

// pageNodeMatchesMethod reports whether pn's value type is the
// receiver type for info (either by reflect.Type or by type name
// for bound method values).
func pageNodeMatchesMethod(pn *PageNode, info *methodInfo) bool {
	if pn == nil {
		return false
	}
	nodeType := pn.Value.Type()
	if info.isBound {
		nodeTypeName := nodeType.Name()
		if nodeType.Kind() == reflect.Pointer {
			nodeTypeName = nodeType.Elem().Name()
		}
		if nodeTypeName != info.receiverTypeName {
			return false
		}
	} else if pointerType(nodeType) != pointerType(info.receiverType) {
		return false
	}
	// Confirm the method exists on this type — guards against
	// matching a same-named type that doesn't have the method.
	_, found := pointerType(nodeType).MethodByName(info.methodName)
	return found
}

// findPageNodeForMethod finds a page node using the method info.
// Only call this for methods, not for standalone functions.
func (p *parseContext) findPageNodeForMethod(info *methodInfo) (*PageNode, error) {
	if info.isFunction {
		panic("findPageNodeForMethod called with standalone function")
	}

	if info.isBound {
		// Find by type name string
		return p.findPageNodeByTypeName(info.receiverTypeName, info.methodName)
	}
	// Find by reflect.Type
	return p.findPageNodeByType(info.receiverType)
}

// findPageNodeByTypeName finds a PageNode by matching its type name.
// Also verifies that the method exists on the page.
func (p *parseContext) findPageNodeByTypeName(typeName, methodName string) (*PageNode, error) {
	for node := range p.root.All() {
		nodeType := node.Value.Type()
		nodeTypeName := nodeType.Name()
		if nodeType.Kind() == reflect.Pointer {
			nodeTypeName = nodeType.Elem().Name()
		}
		if nodeTypeName == typeName {
			// Verify the method exists
			if _, found := nodeType.MethodByName(methodName); !found {
				return nil, fmt.Errorf("method %q not found on page %q", methodName, node.Name)
			}
			return node, nil
		}
	}
	return nil, fmt.Errorf("no page node found with type name %q", typeName)
}

// findPageNodeByType finds a PageNode by its receiver type.
func (p *parseContext) findPageNodeByType(receiverType reflect.Type) (*PageNode, error) {
	// Normalize to pointer type for comparison
	targetType := pointerType(receiverType)

	for node := range p.root.All() {
		nodeType := pointerType(node.Value.Type())
		if targetType == nodeType {
			return node, nil
		}
	}
	return nil, fmt.Errorf("no page node found for type %s", targetType.String())
}

// idForRef handles dynamic method references using the Ref type.
// It supports both qualified references (PageName.MethodName) and simple method names.
func idForRef(pc *parseContext, ref string, rawID bool) (string, error) {
	var pageName, methodName string

	// Check if qualified reference (PageName.MethodName)
	if idx := strings.Index(ref, "."); idx != -1 {
		pageName = ref[:idx]
		methodName = ref[idx+1:]
	} else {
		// Simple method name - find which page(s) have it
		methodName = ref
		matches := findPagesWithMethod(pc, methodName)

		if len(matches) == 0 {
			return "", fmt.Errorf("method %q not found on any page", methodName)
		}
		if len(matches) > 1 {
			names := make([]string, len(matches))
			for i, m := range matches {
				names[i] = m.Name
			}
			return "", fmt.Errorf("method %q found on multiple pages: %s. Use qualified name like %q",
				methodName, strings.Join(names, ", "), names[0]+"."+methodName)
		}
		pageName = matches[0].Name
	}

	// Find the page
	var pn *PageNode
	for node := range pc.root.All() {
		if node.Name == pageName {
			pn = node
			break
		}
	}
	if pn == nil {
		return "", fmt.Errorf("no page found with name %q", pageName)
	}

	// Verify method exists on the page
	pageType := pn.Value.Type()
	if _, found := pageType.MethodByName(methodName); !found {
		return "", fmt.Errorf("method %q not found on page %q", methodName, pageName)
	}

	// Build ID
	return buildID(pn.Name, methodName, rawID), nil
}

// findPagesWithMethod finds all pages that have a method with the given name.
func findPagesWithMethod(pc *parseContext, methodName string) []*PageNode {
	var matches []*PageNode
	for node := range pc.root.All() {
		if _, found := node.Value.Type().MethodByName(methodName); found {
			matches = append(matches, node)
		}
	}
	return matches
}

// buildID constructs the HTML ID string from page name and method name.
func buildID(pageName, methodName string, rawID bool) string {
	var id string
	if pageName != "" {
		id = camelToKebab(pageName) + "-" + camelToKebab(methodName)
	} else {
		// No page name (e.g., standalone function outside page context)
		id = camelToKebab(methodName)
	}
	if !rawID {
		id = "#" + id
	}
	return id
}
