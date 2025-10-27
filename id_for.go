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

	// Build ID with page name prefix for conflict prevention
	id := camelToKebab(pn.Name) + "-" + camelToKebab(methodName)

	// Append suffixes if provided
	for _, suffix := range suffixes {
		id += "-" + camelToKebab(suffix)
	}

	// Prepend "#" unless RawID requested
	if !rawID {
		id = "#" + id
	}

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
