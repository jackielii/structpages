package structpages

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

// IDParams provides advanced configuration for ID generation.
// Use this when you need suffixes or raw IDs without the CSS selector prefix.
type IDParams struct {
	Method   any      // The method expression (required)
	Suffixes []string // Optional suffixes like "container", "input"
	RawID    bool     // If true, returns "user-list" instead of "#user-list"
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
//	// With suffixes for compound IDs
//	IDFor(ctx, IDParams{
//	    Method: p.UserModal,
//	    Suffixes: []string{"container"},
//	})
//	// → "#team-management-view-user-modal-container"
//
//	// Raw ID with suffixes
//	IDFor(ctx, IDParams{
//	    Method: p.GroupSearch,
//	    Suffixes: []string{"input", "field"},
//	    RawID: true,
//	})
//	// → "team-management-view-group-search-input-field"
//
// The function automatically prevents ID conflicts by prefixing with the page name:
//
//	type UserManagement struct{}
//	func (UserManagement) UserList() component { ... }
//	// → "#user-management-user-list"
//
//	type AdminManagement struct{}
//	func (AdminManagement) UserList() component { ... }
//	// → "#admin-management-user-list"
//
// Returns an error if parseContext is not found in the provided context.
// This ensures IDFor is only used within the intended scope (page handlers/templates).
func IDFor(ctx context.Context, v any) (string, error) {
	// Extract parseContext - REQUIRED
	pc := pcCtx.Value(ctx)
	if pc == nil {
		return "", errors.New("parseContext not found in context - IDFor must be called within a page handler or template")
	}

	// Handle IDParams pattern
	var methodExpr any
	var suffixes []string
	rawID := false

	if params, ok := v.(IDParams); ok {
		methodExpr = params.Method
		suffixes = params.Suffixes
		rawID = params.RawID
	} else {
		methodExpr = v
	}

	// Handle Ref type for dynamic method references
	if ref, ok := methodExpr.(Ref); ok {
		return idForRef(pc, string(ref), suffixes, rawID)
	}

	// Extract method and receiver info
	methodName := extractMethodName(methodExpr)
	if methodName == "" {
		return "", errors.New("failed to extract method name from expression")
	}

	receiverType := extractReceiverType(methodExpr)
	if receiverType == nil {
		return "", errors.New("failed to extract receiver type from method expression")
	}

	// Find page node - this gives us the page name
	pn, err := pc.findPageNodeByType(receiverType)
	if err != nil {
		return "", fmt.Errorf("cannot find page for method expression: %w", err)
	}

	// Build ID
	id := buildID(pn.Name, methodName, suffixes, rawID)
	return id, nil
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

// extractMethodName extracts the method name from a method expression using reflection.
// It handles method values and returns the short method name without package or receiver.
func extractMethodName(methodExpr any) string {
	// Get the function value
	v := reflect.ValueOf(methodExpr)
	if v.Kind() != reflect.Func {
		// If not a function, return empty string
		return ""
	}

	// Get the function's PC (program counter)
	pc := v.Pointer()
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return ""
	}

	// Get the full function name (e.g., "github.com/user/pkg.(*Type).Method")
	fullName := fn.Name()

	// Extract just the method name from the full path
	// Format: "package.(*Type).Method" or "package.Type.Method"
	lastDot := strings.LastIndex(fullName, ".")
	if lastDot == -1 {
		return fullName
	}

	methodName := fullName[lastDot+1:]

	// Remove any suffix like "-fm" that Go adds for method values
	if idx := strings.Index(methodName, "-fm"); idx != -1 {
		methodName = methodName[:idx]
	}

	return methodName
}

// extractReceiverType extracts the receiver type from a method expression.
// Returns the receiver type (as a non-pointer type for consistency) or nil if extraction fails.
func extractReceiverType(methodExpr any) reflect.Type {
	v := reflect.ValueOf(methodExpr)
	if v.Kind() != reflect.Func {
		return nil
	}

	// Get the function type
	funcType := v.Type()
	if funcType.NumIn() == 0 {
		return nil
	}

	// First parameter is the receiver
	receiverType := funcType.In(0)

	// Normalize to non-pointer type
	if receiverType.Kind() == reflect.Pointer {
		receiverType = receiverType.Elem()
	}

	return receiverType
}

// idForRef handles dynamic method references using the Ref type.
// It supports both qualified references (PageName.MethodName) and simple method names.
func idForRef(pc *parseContext, ref string, suffixes []string, rawID bool) (string, error) {
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
	return buildID(pn.Name, methodName, suffixes, rawID), nil
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

// buildID constructs the HTML ID string from page name, method name, and optional suffixes.
func buildID(pageName, methodName string, suffixes []string, rawID bool) string {
	id := camelToKebab(pageName) + "-" + camelToKebab(methodName)
	for _, suffix := range suffixes {
		id += "-" + camelToKebab(suffix)
	}
	if !rawID {
		id = "#" + id
	}
	return id
}
