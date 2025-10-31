package structpages

import (
	"net/http"
	"reflect"
	"strings"
)

// Ref represents a dynamic reference to a page or method by name.
// Use it when static type references aren't available (e.g., configuration-driven
// menus, generic components, or code generation scenarios).
//
// For URLFor, the string can be:
//   - Page name: Ref("UserManagement")
//   - Route path: Ref("/user/management") - must start with /
//
// For IDFor, the string can be:
//   - Qualified method: Ref("PageName.MethodName")
//   - Simple method: Ref("MethodName") - must be unambiguous across all pages
//
// Both URLFor and IDFor return descriptive errors if the reference is invalid,
// providing runtime safety for dynamic references.
//
// Example usage:
//
//	// Dynamic menu from configuration
//	menuItems := []struct{ Page Ref; Label string }{
//	    {Ref("HomePage"), "Home"},
//	    {Ref("UserManagement"), "Users"},
//	}
//	for _, item := range menuItems {
//	    url, err := URLFor(ctx, item.Page)
//	    // Handle error if page doesn't exist
//	}
//
//	// Dynamic component reference
//	targetID, err := IDFor(ctx, Ref("UserManagement.UserList"))
type Ref string

// RenderTarget represents a selected component that will be rendered.
// It's available to Props methods via dependency injection, allowing Props to
// load only the data needed for the target component.
//
// RenderTarget is produced by a TargetSelector function (e.g., HTMXRenderTarget).
// The selector determines which component to render based on the request,
// and the resulting RenderTarget is passed to Props.
//
// Example usage in Props:
//
//	func (p DashboardPage) Props(r *http.Request, target RenderTarget) (DashboardProps, error) {
//	    switch {
//	    case target.Is(UserStatsWidget):
//	        stats := loadUserStats()
//	        return DashboardProps{}, RenderComponent(target, stats)
//	    case target.Is(p.Page):
//	        return DashboardProps{Stats: loadAll()}, nil
//	    }
//	}
type RenderTarget interface {
	// Is checks if this target matches the given method or function reference.
	// Works with both page methods and standalone functions.
	// Uses method/function expressions for compile-time safety.
	//
	// For function components, Is() has a side effect: it stores the function
	// value when a match is found, enabling lazy evaluation of the hxTarget.
	Is(method any) bool
}

// TargetSelector determines which component to render for a request.
// It returns a RenderTarget that will be passed to Props.
//
// The default selector is HTMXRenderTarget, which handles HTMX partial requests.
// Custom selectors can be provided via WithTargetSelector option.
type TargetSelector func(r *http.Request, pn *PageNode) (RenderTarget, error)

// methodRenderTarget is the concrete implementation for method components.
type methodRenderTarget struct {
	name   string
	method reflect.Method
}

func (mrt *methodRenderTarget) Is(method any) bool {
	// Check if this methodRenderTarget has a valid method
	// (it might be a zero method for Props-only pages)
	if mrt.method.Type == nil {
		return false
	}

	info, err := extractMethodInfo(method)
	if err != nil {
		return false
	}

	// For standalone functions, don't match against method targets
	if info.isFunction {
		return false
	}

	// For methods, compare both name and receiver type
	var selectedReceiverType reflect.Type
	if mrt.method.Type.NumIn() > 0 {
		selectedReceiverType = mrt.method.Type.In(0)
		if selectedReceiverType.Kind() == reflect.Pointer {
			selectedReceiverType = selectedReceiverType.Elem()
		}
	}

	// For bound methods, compare by receiver type name
	if info.isBound {
		if selectedReceiverType == nil {
			return false
		}
		return mrt.method.Name == info.methodName &&
			selectedReceiverType.Name() == info.receiverTypeName
	}

	// For unbound methods, compare by receiver type
	actualType := info.receiverType
	if actualType.Kind() == reflect.Pointer {
		actualType = actualType.Elem()
	}

	return mrt.method.Name == info.methodName &&
		selectedReceiverType == actualType
}

// functionRenderTarget is the concrete implementation for function components.
type functionRenderTarget struct {
	hxTarget  string        // Raw HX-Target from request (e.g., "dashboard-page-user-stats-widget")
	pageName  string        // Page name for normalization (e.g., "DashboardPage")
	funcValue reflect.Value // Stored when Is() finds a match (lazy evaluation)
}

func (frt *functionRenderTarget) Is(method any) bool {
	info, err := extractMethodInfo(method)
	if err != nil {
		return false
	}

	// Only match against functions
	if !info.isFunction {
		return false
	}

	// Lazy evaluation: convert function name to kebab-case
	funcKebab := camelToKebab(info.methodName)

	// Normalize hxTarget (strip # and page prefix)
	normalized := strings.TrimPrefix(frt.hxTarget, "#")
	pagePrefix := camelToKebab(frt.pageName) + "-"
	normalized = strings.TrimPrefix(normalized, pagePrefix)

	// Check if it matches
	if normalized == funcKebab || strings.HasSuffix(normalized, "-"+funcKebab) {
		// Store funcValue for later rendering
		frt.funcValue = reflect.ValueOf(method)
		return true
	}

	return false
}

// newMethodRenderTarget creates a RenderTarget for a method component.
func newMethodRenderTarget(name string, method *reflect.Method) RenderTarget {
	return &methodRenderTarget{
		name:   name,
		method: *method,
	}
}

// newFunctionRenderTarget creates a RenderTarget for a function component.
// The hxTarget is stored as-is for lazy evaluation in Is().
func newFunctionRenderTarget(hxTarget, pageName string) RenderTarget {
	return &functionRenderTarget{
		hxTarget: hxTarget,
		pageName: pageName,
		// funcValue filled in later by Is()
	}
}
