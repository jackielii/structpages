package structpages

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// IDParams provides advanced configuration for ID generation.
// Use this when you need raw IDs without the CSS selector prefix.
type IDParams struct {
	Method any  // The method expression (required)
	RawID  bool // If true, returns "user-list" instead of "#user-list"
}

// IDFor generates a consistent HTML ID or CSS selector for a component method.
// It requires a valid parseContext in the provided context and returns an error if not found.
//
// By default, returns a CSS selector (with "#" prefix) for use in HTMX targets.
// The ID includes the page name prefix to avoid conflicts across pages.
//
// Basic usage (returns CSS selector):
//
//	IDFor(ctx, p.UserList)
//	// → "#team-management-view-user-list"
//
//	IDFor(ctx, TeamManagementView{}.GroupMembers)
//	// → "#team-management-view-group-members"
//
// Advanced usage with IDParams:
//
//	// For id attribute (raw ID without "#")
//	IDFor(ctx, IDParams{
//	    Method: p.UserList,
//	    RawID: true,
//	})
//	// → "team-management-view-user-list"
//
// Returns an error if parseContext is not found in the provided context.
// This ensures IDFor is only used within the intended scope (page handlers/templates).
func IDFor(ctx context.Context, v any) (string, error) {
	// Extract parseContext - REQUIRED
	pc := pcCtx.Value(ctx)
	if pc == nil {
		return "", errors.New("parseContext not found in context - IDFor must be called within a page handler or template")
	}

	return idFor(ctx, pc, v)
}

// idFor generates the ID string based on the provided value (method expression or IDParams).
func idFor(ctx context.Context, pc *parseContext, v any) (string, error) {
	// Handle IDParams pattern
	var methodExpr any
	rawID := false

	if params, ok := v.(IDParams); ok {
		methodExpr = params.Method
		rawID = params.RawID
	} else {
		methodExpr = v
	}

	// Handle Ref type for dynamic method references
	if ref, ok := methodExpr.(Ref); ok {
		return idForRef(pc, string(ref), rawID)
	}

	// Extract all method info (handles methods and standalone functions)
	info, err := extractMethodInfo(methodExpr)
	if err != nil {
		return "", err
	}

	// Find the page node
	pn, err := pc.findPageNodeForMethod(ctx, info)
	if err != nil {
		return "", fmt.Errorf("cannot find page for method expression: %w", err)
	}

	// Build ID
	id := buildID(pn.Name, info.methodName, rawID)
	return id, nil
}

// findPageNodeForMethod finds a page node using the method info
func (p *parseContext) findPageNodeForMethod(ctx context.Context, info *methodInfo) (*PageNode, error) {
	// Handle standalone functions - use current page from context
	if info.isFunction {
		currentPage := currentPageCtx.Value(ctx)
		if currentPage == nil {
			// Fallback: no page prefix, return empty string as page name
			// This allows IDFor to work outside page context (e.g., tests)
			return &PageNode{Name: ""}, nil
		}
		return currentPage, nil
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
