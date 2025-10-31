package structpages

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"slices"
)

// ErrSkipPageRender is a sentinel error that can be returned from a Props method
// to indicate that the page rendering should be skipped. This is useful for
// implementing conditional rendering or redirects within page logic.
var ErrSkipPageRender = errors.New("skip page render")

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

// MiddlewareFunc is a function that wraps an http.Handler with additional functionality.
// It receives both the handler to wrap and the PageNode being handled, allowing middleware
// to access page metadata like route, title, and other properties.
type MiddlewareFunc func(http.Handler, *PageNode) http.Handler

// Mux represents any HTTP router that can register handlers using the Handle method.
// This interface is satisfied by http.ServeMux and must follow the same pattern support
// for route registration.
type Mux interface {
	Handle(pattern string, handler http.Handler)
}

// StructPages holds the parsed page tree context for URL generation.
// It is returned by Mount and provides URLFor and IDFor methods.
type StructPages struct {
	pc             *parseContext
	onError        func(http.ResponseWriter, *http.Request, error)
	middlewares    []MiddlewareFunc
	targetSelector TargetSelector
	warnEmptyRoute func(*PageNode)
	args           []any
}

// Option represents a configuration option for StructPages.
type Option func(*StructPages)

// Mount parses the page tree and registers all routes onto the provided mux.
// If mux is nil, routes are registered on http.DefaultServeMux.
// Returns a StructPages that provides URLFor and IDFor methods.
//
// Parameters:
//   - mux: Any router satisfying the Mux interface (e.g., http.ServeMux). If nil, uses http.DefaultServeMux.
//   - page: A struct instance with route-tagged fields
//   - route: The base route path for this page tree (e.g., "/" or "/admin")
//   - title: The title for the root page
//   - options: Optional configuration (WithErrorHandler, WithMiddlewares, etc.) and dependency injection args
//
// Example with custom mux:
//
//	mux := http.NewServeMux()
//	sp, err := structpages.Mount(mux, index{}, "/", "My App",
//	    structpages.WithErrorHandler(customHandler))
//	sp.URLFor(index.Page)
//	http.ListenAndServe(":8080", mux)
//
// Example with DefaultServeMux:
//
//	sp, err := structpages.Mount(nil, index{}, "/", "My App")
//	http.ListenAndServe(":8080", nil)
func Mount(mux Mux, page any, route, title string, options ...Option) (*StructPages, error) {
	if mux == nil {
		mux = http.DefaultServeMux
	}

	sp := &StructPages{
		onError: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		},
		targetSelector: HTMXRenderTarget,
	}

	for _, opt := range options {
		opt(sp)
	}

	// Parse page tree
	pc, err := parsePageTree(route, page, sp.args...)
	if err != nil {
		return nil, err
	}
	pc.root.Title = title
	sp.pc = pc

	// Register all pages
	middlewares := append([]MiddlewareFunc{withPcCtx(pc), extractURLParams}, sp.middlewares...)
	if err := sp.registerPageItem(mux, pc.root, middlewares); err != nil {
		return nil, err
	}

	return sp, nil
}

// URLFor returns the URL for a given page type. If args is provided, it'll replace
// the path segments. Supported format is similar to http.ServeMux.
//
// Unlike the context-based URLFor function, this method doesn't have access to
// pre-extracted URL parameters from the current request, so all required parameters
// must be provided as args.
//
// If multiple page type matches are found, the first one is returned.
// In such situation, use a func(*PageNode) bool as page argument to match a specific page.
//
// Additionally, you can pass []any to page to join multiple path segments together.
// Strings will be joined as is. Example:
//
//	sp.URLFor([]any{Page{}, "?foo={bar}"}, "bar", "baz")
//
// It also supports a func(*PageNode) bool as the Page argument to match a specific page.
// It can be useful when you have multiple pages with the same type but different routes.
func (sp *StructPages) URLFor(page any, args ...any) (string, error) {
	// Create a context with parseContext and call the context-based URLFor
	ctx := pcCtx.WithValue(context.Background(), sp.pc)
	return URLFor(ctx, page, args...)
}

// IDFor generates a consistent HTML ID or CSS selector for a component method.
// It works without context by using the structpages's parseContext directly.
//
// By default, returns a CSS selector (with "#" prefix) for use in HTMX targets.
// The ID includes the page name prefix to avoid conflicts across pages.
//
// Basic usage (returns CSS selector):
//
//	sp.IDFor(p.UserList)
//	// → "#team-management-view-user-list"
//
//	sp.IDFor(TeamManagementView{}.GroupMembers)
//	// → "#team-management-view-group-members"
//
// Advanced usage with IDParams:
//
//	// For id attribute (raw ID without "#")
//	sp.IDFor(IDParams{
//	    Method: p.UserList,
//	    RawID: true,
//	})
//	// → "team-management-view-user-list"
func (sp *StructPages) IDFor(v any) (string, error) {
	// Create a context with parseContext - no currentPage since this is outside request context
	ctx := pcCtx.WithValue(context.Background(), sp.pc)
	return idFor(ctx, sp.pc, v)
}

// WithArgs adds global dependency injection arguments that will be
// available to all page methods (Props, Middlewares, ServeHTTP etc.).
func WithArgs(args ...any) func(*StructPages) {
	return func(r *StructPages) {
		r.args = append(r.args, args...)
	}
}

// WithTargetSelector sets a custom TargetSelector function that determines
// which component to render based on the request. The default is HTMXRenderTarget,
// which handles HTMX partial requests automatically.
//
// The selector function receives the request and page node, and returns a RenderTarget
// that will be passed to Props. This allows custom logic for component selection.
//
// Example - Custom selector with A/B testing:
//
//	sp := Mount(http.NewServeMux(), index{}, "/", "My App",
//	    WithTargetSelector(func(r *http.Request, pn *PageNode) (RenderTarget, error) {
//	        if getABTestVariant(r) == "B" {
//	            // Use different component for variant B
//	            method := pn.Components["ContentB"]
//	            return newMethodRenderTarget("ContentB", method), nil
//	        }
//	        // Fallback to default HTMX behavior
//	        return HTMXRenderTarget(r, pn)
//	    }))
func WithTargetSelector(selector TargetSelector) func(*StructPages) {
	return func(r *StructPages) {
		r.targetSelector = selector
	}
}

// WithErrorHandler sets a custom error handler function that will be called when
// an error occurs during page rendering or request handling. If not set, a default
// handler returns a generic "Internal Server Error" response.
func WithErrorHandler(onError func(http.ResponseWriter, *http.Request, error)) func(*StructPages) {
	return func(r *StructPages) {
		r.onError = onError
	}
}

// WithMiddlewares adds global middleware functions that will be applied to all routes.
// Middleware is executed in the order provided, with the first middleware being the
// outermost handler. These global middlewares run before any page-specific middlewares.
func WithMiddlewares(middlewares ...MiddlewareFunc) func(*StructPages) {
	return func(r *StructPages) {
		r.middlewares = append(r.middlewares, middlewares...)
	}
}

// WithWarnEmptyRoute sets a custom warning function for pages that have neither
// a handler method nor children. These pages are automatically skipped during
// route registration. If warnFunc is nil, a default warning message is printed
// to stdout. Set warnFunc to a no-op function to suppress warnings entirely.
//
// Example usage:
//
//	// Use default warning (prints to stdout)
//	sp := structpages.Mount(
//		http.NewServeMux(), index{}, "/", "App",
//		structpages.WithWarnEmptyRoute(nil),
//	)
//
//	// Custom warning function
//	sp := structpages.Mount(
//		http.NewServeMux(), index{}, "/", "App",
//		structpages.WithWarnEmptyRoute(func(pn *PageNode) {
//			log.Printf("Skipping empty page: %s", pn.Name)
//		}),
//	)
//
//	// Suppress warnings entirely
//	sp := structpages.Mount(
//		http.NewServeMux(), index{}, "/", "App",
//		structpages.WithWarnEmptyRoute(func(*PageNode) {}),
//	)
func WithWarnEmptyRoute(warnFunc func(*PageNode)) func(*StructPages) {
	if warnFunc == nil {
		warnFunc = func(pn *PageNode) {
			fmt.Printf("⚠️  Warning: page route has no children and no handler, skipping route registration: %s\n", pn.Name)
		}
	}
	return func(r *StructPages) {
		r.warnEmptyRoute = warnFunc
	}
}

func (sp *StructPages) registerPageItem(mux Mux, page *PageNode, mw []MiddlewareFunc) error {
	if page.Route == "" {
		return fmt.Errorf("page item route is empty: %s", page.Name)
	}

	if page.Middlewares != nil {
		res, err := sp.pc.callMethod(page, page.Middlewares)
		if err != nil {
			return fmt.Errorf("error calling Middlewares method on %s: %w", page.Name, err)
		}
		if len(res) != 1 {
			return fmt.Errorf("middlewares method on %s did not return single result", page.Name)
		}
		mws, ok := res[0].Interface().([]MiddlewareFunc)
		if !ok {
			return fmt.Errorf("middlewares method on %s did not return []func(http.Handler, *PageNode) http.Handler", page.Name)
		}
		mw = append(mw, mws...)
	}
	if page.Children != nil {
		// nested pages has to be registered first to avoid conflicts with the parent route
		for _, child := range page.Children {
			if err := sp.registerPageItem(mux, child, mw); err != nil {
				return err
			}
		}
	}
	handler := sp.buildHandler(page)
	if handler == nil && len(page.Children) == 0 {
		if sp.warnEmptyRoute != nil {
			sp.warnEmptyRoute(page)
		}
		return nil
	} else if handler == nil {
		return nil
	}
	for _, middleware := range slices.Backward(mw) {
		handler = middleware(handler, page)
	}
	// If method is "ALL", register without method prefix (matches all methods)
	// Otherwise, register with "METHOD /path" format
	pattern := page.FullRoute()
	if page.Method != methodAll {
		pattern = page.Method + " " + pattern
	}
	mux.Handle(pattern, handler)
	return nil
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
		targetPage, err := sp.pc.findPageNodeForMethod(r.Context(), info)
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

func (sp *StructPages) buildHandler(page *PageNode) http.Handler {
	if h := sp.asHandler(page); h != nil {
		return h
	}
	if len(page.Components) == 0 {
		return nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Inject current page into context for IDFor to use with standalone functions
		ctx := currentPageCtx.WithValue(r.Context(), page)
		r = r.WithContext(ctx)

		// 1. Select which component to render using TargetSelector
		target, err := sp.targetSelector(r, page)
		if err != nil {
			sp.onError(w, r, fmt.Errorf("error selecting target for %s: %w", page.Name, err))
			return
		}

		// 2. Call Props with RenderTarget available for injection
		props, err := sp.execProps(page, r, w, target)
		if err != nil {
			// Check if it's a render component error
			if sp.handleRenderComponentError(w, r, err, page, props) {
				return
			}

			if errors.Is(err, ErrSkipPageRender) {
				return
			}
			sp.onError(w, r, fmt.Errorf("error running props for %s: %w", page.Name, err))
			return
		}

		// 3. Extract method from target and render with props
		// Type-assert to get the method
		if mrt, ok := target.(*methodRenderTarget); ok {
			// Validate method before calling
			if !mrt.method.Func.IsValid() {
				// Check if Props method exists - if so, this is a Props-only page
				if _, hasProps := page.Props["Props"]; hasProps {
					sp.onError(w, r, fmt.Errorf("page %s: no component found and Props did not use RenderComponent", page.Name))
				} else {
					sp.onError(w, r, fmt.Errorf("page %s does not have a Page component method", page.Name))
				}
				return
			}
			comp, err := sp.pc.callComponentMethod(page, &mrt.method, props...)
			if err != nil {
				sp.onError(w, r, fmt.Errorf("error calling component %s.%s: %w", page.Name, mrt.method.Name, err))
				return
			}
			sp.render(w, r, comp)
			return
		}

		// If we get here, target is not a method (could be function or custom)
		// Props should have called RenderComponent
		sp.onError(w, r, fmt.Errorf("page %s: target is not a method and Props did not use RenderComponent", page.Name))
	})
}

func (sp *StructPages) render(w http.ResponseWriter, r *http.Request, comp component) {
	buf := getBuffer()
	defer releaseBuffer(buf)
	if err := comp.Render(r.Context(), buf); err != nil {
		sp.onError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

type httpErrHandler interface {
	ServeHTTP(http.ResponseWriter, *http.Request) error
}

var (
	handlerType    = reflect.TypeOf((*http.Handler)(nil)).Elem()
	errHandlerType = reflect.TypeOf((*httpErrHandler)(nil)).Elem()
)

func (sp *StructPages) asHandler(pn *PageNode) http.Handler {
	v := pn.Value
	st, pt := v.Type(), v.Type()
	if st.Kind() == reflect.Pointer {
		st = st.Elem()
	} else {
		pt = reflect.PointerTo(st)
	}
	method, ok := st.MethodByName("ServeHTTP")
	if !ok || isPromotedMethod(&method) {
		method, ok = pt.MethodByName("ServeHTTP")
		if !ok || isPromotedMethod(&method) {
			return nil
		}
	}

	if v.Type().Implements(handlerType) {
		return v.Interface().(http.Handler)
	}
	if v.Type().Implements(errHandlerType) {
		h := v.Interface().(httpErrHandler)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// because we have to handle errors, and error handler could write header
			// potentially we want to clear the buffer writer
			bw := newBuffered(w)
			defer func() { _ = bw.close() }() // ignore error, no way to recover from it. maybe log it?
			if err := h.ServeHTTP(bw, r); err != nil {
				// Clear the buffer since we have an error
				bw.buf.Reset()
				// Check if it's a render component error
				if sp.handleRenderComponentError(bw, r, err, pn, nil) {
					return
				}
				// Write error directly to the buffered writer
				sp.onError(bw, r, err)
			}
		})
	}
	// extended ServeHTTP method with extra arguments
	if method.Type.NumIn() > 3 { // receiver, http.ResponseWriter, *http.Request
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var wv reflect.Value // ResponseWriter, will be buffered if handler returns error
			var bw *buffered
			if method.Type.NumOut() > 0 {
				// If the method returns any values (including just an error), we need to buffer
				bw = newBuffered(w)
				defer func() { _ = bw.close() }() // ignore error, no way to recover from it. maybe log it?
				wv = reflect.ValueOf(bw)
			} else {
				wv = reflect.ValueOf(w)
			}

			// Create RenderTarget for dependency injection using targetSelector
			var renderTarget RenderTarget
			if sp.targetSelector != nil {
				renderTarget, _ = sp.targetSelector(r, pn)
			}

			// Make RenderTarget available for dependency injection
			additionalArgs := []reflect.Value{wv, reflect.ValueOf(r), reflect.ValueOf(renderTarget)}

			results, err := sp.pc.callMethod(pn, &method, additionalArgs...)
			if err != nil {
				if bw != nil {
					bw.buf.Reset()
					sp.onError(bw, r, fmt.Errorf("error calling ServeHTTP method on %s: %w", pn.Name, err))
				} else {
					sp.onError(w, r, fmt.Errorf("error calling ServeHTTP method on %s: %w", pn.Name, err))
				}
				return
			}
			_, err = extractError(results)
			if err != nil {
				// bw is guaranteed to be non-nil here because:
				// - bw is nil only when method.Type.NumOut() == 0
				// - when NumOut() == 0, extractError always returns nil error
				// - therefore this branch is only reachable when bw != nil
				bw.buf.Reset()
				// Check if it's a render component error
				if sp.handleRenderComponentError(bw, r, err, pn, nil) {
					return
				}
				sp.onError(bw, r, err)
				return
			}
		})
	}

	return nil
}

func (sp *StructPages) execProps(pn *PageNode,
	r *http.Request, w http.ResponseWriter, renderTarget RenderTarget,
) ([]reflect.Value, error) {
	// Look for Props method
	propMethod, ok := pn.Props["Props"]
	if !ok {
		return nil, nil
	}

	if !propMethod.Func.IsValid() {
		return nil, fmt.Errorf("Props method for page %s has invalid Func", pn.Name)
	}

	// Make RenderTarget available for injection along with r and w
	// Note: only pass valid values to avoid zero reflect.Value issues
	args := []reflect.Value{reflect.ValueOf(r), reflect.ValueOf(w)}
	if renderTarget != nil {
		args = append(args, reflect.ValueOf(renderTarget))
	}
	props, err := sp.pc.callMethod(pn, &propMethod, args...)
	if err != nil {
		return nil, fmt.Errorf("error calling Props method %s.Props: %w", pn.Name, err)
	}
	return extractError(props)
}
