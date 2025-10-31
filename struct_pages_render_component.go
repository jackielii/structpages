package structpages

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
)

// errRenderComponent is an internal error type that specifies which component
// to render and provides replacement arguments for that component.
type errRenderComponent struct {
	target RenderTarget // The target component to render
	args   []any        // Replacement arguments for the component
}

func (e *errRenderComponent) Error() string {
	return "should render component from target"
}

// RenderComponent creates an error that instructs the framework to render
// a specific component instead of the default component.
//
// It supports two patterns:
//
//  1. Same-page component (with target from Props):
//     func (p DashboardPage) Props(r *http.Request, target RenderTarget) (DashboardProps, error) {
//     if target.Is(UserStatsWidget) {
//     stats := loadUserStats()
//     return DashboardProps{}, RenderComponent(target, stats)
//     }
//     }
//
//  2. Cross-page component (with method expression):
//     func (p MyPage) Props(r *http.Request) (Props, error) {
//     return Props{}, RenderComponent(OtherPage.ErrorComponent, "error message")
//     }
func RenderComponent(targetOrMethod any, args ...any) error {
	// Check if first arg is a RenderTarget (same-page component)
	if rt, ok := targetOrMethod.(RenderTarget); ok {
		return &errRenderComponent{target: rt, args: args}
	}

	// Otherwise, it's a method expression (cross-page component)
	// Create a wrapper target that will be handled by extracting the method
	return &errRenderComponent{target: &crossPageTarget{method: targetOrMethod}, args: args}
}

// crossPageTarget is a special RenderTarget for cross-page component rendering
type crossPageTarget struct {
	method any
}

func (cpt *crossPageTarget) Is(method any) bool {
	// Cross-page targets don't support Is() - they're used directly
	return false
}

// handleRenderComponentError checks if the error is an errRenderComponent and handles it.
// Returns true if it handled the error, false otherwise.
func (sp *StructPages) handleRenderComponentError(
	w http.ResponseWriter, r *http.Request, err error, page *PageNode, props []reflect.Value,
) bool {
	var renderErr *errRenderComponent
	if !errors.As(err, &renderErr) {
		return false
	}

	// Convert args to reflect.Values
	componentArgs := make([]reflect.Value, len(renderErr.args))
	for i, arg := range renderErr.args {
		componentArgs[i] = reflect.ValueOf(arg)
	}

	// Type-assert target to determine how to render
	switch target := renderErr.target.(type) {
	case *functionRenderTarget:
		// Function component - funcValue was stored by Is()
		if !target.funcValue.IsValid() {
			sp.onError(w, r, fmt.Errorf("RenderComponent: function target has no funcValue (did you call target.Is() first?)"))
			return true
		}

		results := target.funcValue.Call(componentArgs)
		if len(results) != 1 {
			sp.onError(w, r, fmt.Errorf("RenderComponent: function must return single component"))
			return true
		}

		comp, ok := results[0].Interface().(component)
		if !ok {
			sp.onError(w, r, fmt.Errorf("RenderComponent: function must return component"))
			return true
		}

		sp.render(w, r, comp)
		return true

	case *methodRenderTarget:
		// Method component - method is already in target
		comp, compErr := sp.pc.callComponentMethod(page, &target.method, componentArgs...)
		if compErr != nil {
			sp.onError(w, r, fmt.Errorf("error calling component %s.%s: %w", page.Name, target.method.Name, compErr))
			return true
		}

		sp.render(w, r, comp)
		return true

	case *crossPageTarget:
		// Cross-page component - extract method info and render
		info, err := extractMethodInfo(target.method)
		if err != nil {
			sp.onError(w, r, fmt.Errorf("failed to extract method info: %w", err))
			return true
		}

		// Handle standalone functions
		if info.isFunction {
			fnValue := reflect.ValueOf(target.method)
			if fnValue.Kind() != reflect.Func {
				sp.onError(w, r, fmt.Errorf("RenderComponent: not a function"))
				return true
			}

			results := fnValue.Call(componentArgs)
			if len(results) != 1 {
				sp.onError(w, r, fmt.Errorf("RenderComponent: function must return single component"))
				return true
			}

			comp, ok := results[0].Interface().(component)
			if !ok {
				sp.onError(w, r, fmt.Errorf("RenderComponent: function must return component"))
				return true
			}

			sp.render(w, r, comp)
			return true
		}

		// Handle methods - find the page that owns this component
		targetPage, err := sp.pc.findPageNodeForMethod(info)
		if err != nil {
			sp.onError(w, r, fmt.Errorf("cannot find page for method expression: %w", err))
			return true
		}

		// Look up the component method
		compMethod, ok := targetPage.Components[info.methodName]
		if !ok {
			sp.onError(w, r, fmt.Errorf("component %s not found in page %s", info.methodName, targetPage.Name))
			return true
		}

		// Execute the component with the args
		comp, compErr := sp.pc.callComponentMethod(targetPage, &compMethod, componentArgs...)
		if compErr != nil {
			sp.onError(w, r, fmt.Errorf("error calling component %s.%s: %w", targetPage.Name, compMethod.Name, compErr))
			return true
		}

		sp.render(w, r, comp)
		return true

	default:
		// Custom RenderTarget - can't render
		sp.onError(w, r, fmt.Errorf("RenderComponent: cannot render custom RenderTarget type: %T", renderErr.target))
		return true
	}
}
