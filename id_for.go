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

// resolvePageForMethod picks the PageNode whose Name supplies the
// id prefix. It prefers the currentPage when its receiver type
// matches the method expression — that way a page rendering its
// own template gets the id for *its* mount, not whichever match
// tree-walk encounters first. Falls back to the global lookup
// when there is no current page or its type doesn't match.
func resolvePageForMethod(pc *parseContext, currentPage *PageNode, info *methodInfo) (*PageNode, error) {
	if currentPage != nil && pageNodeMatchesMethod(currentPage, info) {
		return currentPage, nil
	}
	return pc.findPageNodeForMethod(info)
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
