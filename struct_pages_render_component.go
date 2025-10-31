package structpages

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
)

// renderOp represents what to render and how to get it.
type renderOp struct {
	component component     // Direct component (just render it)
	callable  reflect.Value // Function/method to call (returns component)
	args      []reflect.Value

	// For methodRenderTarget from Props:
	method *reflect.Method // The method to call on page
}

// errRenderComponent is an internal error type that carries a renderOp.
type errRenderComponent struct {
	op *renderOp
}

func (e *errRenderComponent) Error() string {
	return "should render component from target"
}

// RenderComponent creates an error that instructs the framework to render
// a specific component instead of the default component.
//
// It supports multiple patterns:
//
// 1. Direct component:
//
//	comp := MyComponent("data")
//	return RenderComponent(comp)
//
// 2. Custom RenderTarget with Component() method (for custom TargetSelector implementations):
//
//	type customTarget struct { data string }
//	func (ct customTarget) Is(method any) bool { ... }
//	func (ct customTarget) Component() component { return MyComponent(ct.data) }
//	// Custom TargetSelector returns customTarget
//	// Props can then: return Props{}, RenderComponent(target)
//
// 3. Same-page component (with target from Props):
//
//	func (p DashboardPage) Props(r *http.Request, target RenderTarget) (DashboardProps, error) {
//		if target.Is(UserStatsWidget) {
//			stats := loadUserStats()
//			return DashboardProps{}, RenderComponent(target, stats)
//		}
//	}
//
// 4. Cross-page component (with method expression):
//
//	func (p MyPage) Props(r *http.Request) (Props, error) {
//		return Props{}, RenderComponent(OtherPage.ErrorComponent, "error message")
//	}
func RenderComponent(targetOrMethod any, args ...any) error {
	op, err := resolveRenderOp(targetOrMethod, args)
	if err != nil {
		return err
	}
	return &errRenderComponent{op: op}
}

// componentGetter is an optional interface for custom RenderTarget implementations.
// When a custom TargetSelector returns a RenderTarget that implements this interface,
// RenderComponent can call Component() to get the component directly without args.
type componentGetter interface {
	Component() component
}

// resolveRenderOp converts the input into a renderOp.
func resolveRenderOp(target any, args []any) (*renderOp, error) {
	// Case 1: Direct component - just render it
	if comp, ok := target.(component); ok {
		if len(args) > 0 {
			return nil, fmt.Errorf("RenderComponent: component instance cannot have args")
		}
		return &renderOp{component: comp}, nil
	}

	// Case 2: Type with Component() method - call it to get component
	if cg, ok := target.(componentGetter); ok {
		if len(args) > 0 {
			return nil, fmt.Errorf("RenderComponent: componentGetter cannot have args")
		}
		comp := cg.Component()
		return &renderOp{component: comp}, nil
	}

	// Convert args to reflect.Values once
	reflectArgs := make([]reflect.Value, len(args))
	for i, arg := range args {
		reflectArgs[i] = reflect.ValueOf(arg)
	}

	// Case 3: RenderTarget (from Props) - method or function
	if rt, ok := target.(RenderTarget); ok {
		op, err := renderOpFromTarget(rt, reflectArgs)
		if err != nil {
			return nil, err
		}
		return op, nil
	}

	// Case 4: Method/function expression or any callable
	val := reflect.ValueOf(target)

	// Must be a function
	if val.Kind() != reflect.Func {
		return nil, fmt.Errorf("RenderComponent: target must be a component, RenderTarget, or function, got %T", target)
	}

	// It's a callable (method expression, named function, or anonymous function)
	return &renderOp{
		callable: val,
		args:     reflectArgs,
	}, nil
}

// renderOpFromTarget creates a renderOp from a RenderTarget.
func renderOpFromTarget(rt RenderTarget, args []reflect.Value) (*renderOp, error) {
	switch target := rt.(type) {
	case *methodRenderTarget:
		// Method target - store method info, will need page instance later
		return &renderOp{
			method: &target.method,
			args:   args,
		}, nil

	case *functionRenderTarget:
		// Function target - funcValue was stored by Is()
		if !target.funcValue.IsValid() {
			return nil, fmt.Errorf("function target has no funcValue (did you call target.Is() first?)")
		}
		return &renderOp{
			callable: target.funcValue,
			args:     args,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported RenderTarget type: %T", rt)
	}
}

// executeRenderOp executes a renderOp and returns the component to render.
func (sp *StructPages) executeRenderOp(op *renderOp, page *PageNode) (component, error) {
	// Case 1: Direct component
	if op.component != nil {
		return op.component, nil
	}

	// Case 2: Method from RenderTarget (has method but no callable)
	if op.method != nil {
		if page == nil {
			return nil, fmt.Errorf("cannot execute method without page context")
		}
		return sp.pc.callComponentMethod(page, op.method, op.args...)
	}

	// Case 3: Callable (function or method expression)
	if !op.callable.IsValid() {
		return nil, fmt.Errorf("renderOp has no component, method, or callable")
	}

	// Validate argument count before calling to prevent panic
	funcType := op.callable.Type()
	if funcType.NumIn() != len(op.args) {
		return nil, fmt.Errorf("callable expects %d arguments but got %d", funcType.NumIn(), len(op.args))
	}

	results := op.callable.Call(op.args)
	if len(results) != 1 {
		return nil, fmt.Errorf("component callable must return single value, got %d", len(results))
	}

	comp, ok := results[0].Interface().(component)
	if !ok {
		return nil, fmt.Errorf("component callable must return component, got %T", results[0].Interface())
	}

	return comp, nil
}

// handleRenderComponentError checks if the error is an errRenderComponent and handles it.
// Returns true if it handled the error, false otherwise.
func (sp *StructPages) handleRenderComponentError(
	w http.ResponseWriter, r *http.Request, err error, page *PageNode,
) bool {
	var renderErr *errRenderComponent
	if !errors.As(err, &renderErr) {
		return false
	}

	op := renderErr.op

	// For method expressions (not from RenderTarget), we need to resolve the page
	if op.callable.IsValid() && op.method == nil {
		info, extractErr := extractMethodInfo(op.callable.Interface())

		// Handle extraction errors - these indicate the callable is not what we expected
		if extractErr != nil {
			// If it's a callable but we can't extract method info, it might be a lambda or complex function
			// Try to call it directly, but only if it has the right number of args
			funcType := op.callable.Type()
			if funcType.NumIn() != len(op.args) {
				sp.onError(w, r, fmt.Errorf("callable expects %d arguments but got %d (extraction error: %w)",
					funcType.NumIn(), len(op.args), extractErr))
				return true
			}
			// Fall through to execute as-is
		} else if !info.isFunction {
			// It's a method expression - find the page and convert to method call
			targetPage, findErr := sp.pc.findPageNodeForMethod(info)
			if findErr != nil {
				sp.onError(w, r, fmt.Errorf("cannot find page for method expression: %w", findErr))
				return true
			}

			// Find the component method on the page
			method, ok := targetPage.Components[info.methodName]
			if !ok {
				sp.onError(w, r, fmt.Errorf("component %s not found in page %s", info.methodName, targetPage.Name))
				return true
			}

			// Convert to a method-based renderOp
			op.method = &method
			op.callable = reflect.Value{} // Clear callable
			page = targetPage
		}
		// else: it's a standalone function, execute as-is at line 221
	}

	// Execute the renderOp
	comp, execErr := sp.executeRenderOp(op, page)
	if execErr != nil {
		sp.onError(w, r, fmt.Errorf("error executing render operation: %w", execErr))
		return true
	}

	// Render the component
	sp.render(w, r, comp)
	return true
}
