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

// RenderTarget contains information about which component will be rendered.
// It's available to Props methods via dependency injection, allowing Props to
// load only the data needed for the target component.
//
// Example usage in Props:
//
//	func (p index) Props(r *http.Request, target *structpages.RenderTarget) ([]Todo, error) {
//	    switch {
//	    case target.Is(index.TodoList):
//	        return getTodos(), nil
//	    case target.Is(index.Page):
//	        return getAllData(), nil
//	    default:
//	        return getAllData(), nil
//	    }
//	}
type RenderTarget struct {
	selectedMethod reflect.Method // The actual component method that was selected
}

// Is returns true if the given method expression matches the target component.
// Uses method expressions for compile-time safety and IDE refactoring support.
//
// Example:
//
//	if target.Is(index.TodoList) {
//	    // TodoList component will be rendered
//	}
func (rt *RenderTarget) Is(method any) bool {
	// If no component is selected yet (Props-only pattern), return false
	if rt == nil || !rt.selectedMethod.Func.IsValid() || rt.selectedMethod.Type == nil {
		return false
	}

	methodName := extractMethodName(method)
	if methodName == "" {
		return false
	}

	// Compare both method name and receiver type for more robust matching
	receiverType := extractReceiverType(method)
	if receiverType == nil {
		return false
	}

	selectedReceiverType := rt.selectedMethod.Type.In(0)
	if selectedReceiverType.Kind() == reflect.Pointer {
		selectedReceiverType = selectedReceiverType.Elem()
	}

	return rt.selectedMethod.Name == methodName &&
		selectedReceiverType == receiverType
}

// errRenderComponent is an internal error type that specifies which component
// to render and optionally provides replacement arguments for that component.
type errRenderComponent struct {
	method any   // Method expression (e.g., p.UserList)
	args   []any // Optional replacement arguments
}

func (e *errRenderComponent) Error() string {
	return "should render component from method expression"
}

// RenderComponent creates an error that instructs the framework to render
// a specific component method instead of the default component.
// If args are provided, they completely replace the Props return values for the component.
// If no args are provided, the Props return values are used with the specified component.
//
// Uses method expressions for compile-time safety and IDE refactoring support.
//
// Example:
//
//	func (p MyPage) Props(r *http.Request) (string, error) {
//	    if r.URL.Query().Get("partial") == "true" {
//	        // PartialView will receive only these args
//	        return "", RenderComponent(p.PartialView, "custom data")
//	    }
//	    return "default data", nil
//	}
func RenderComponent(method any, args ...any) error {
	return &errRenderComponent{method: method, args: args}
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
	pc                       *parseContext
	onError                  func(http.ResponseWriter, *http.Request, error)
	middlewares              []MiddlewareFunc
	defaultComponentSelector func(r *http.Request, pn *PageNode) (string, error)
	warnEmptyRoute           func(*PageNode)
	args                     []any
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
		defaultComponentSelector: HTMXPageConfig,
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
//
//	// With suffixes for compound IDs
//	sp.IDFor(IDParams{
//	    Method: p.UserModal,
//	    Suffixes: []string{"container"},
//	})
//	// → "#team-management-view-user-modal-container"
func (sp *StructPages) IDFor(v any) (string, error) {
	// Create a context with parseContext and call the context-based IDFor
	ctx := pcCtx.WithValue(context.Background(), sp.pc)
	return IDFor(ctx, v)
}

// WithArgs adds global dependency injection arguments that will be
// available to all page methods (Props, Middlewares, ServeHTTP etc.).
func WithArgs(args ...any) func(*StructPages) {
	return func(r *StructPages) {
		r.args = append(r.args, args...)
	}
}

// WithDefaultComponentSelector sets a global component selector function that determines
// which component to render when RenderComponent is not explicitly called in Props.
// This is useful for implementing common patterns like HTMX partial rendering across all pages.
//
// The selector function receives the request and page node, and returns the component method name.
// For example, returning "Content" will render the Content() method instead of Page().
//
// Example - HTMX boost pattern:
//
//	sp := Mount(http.NewServeMux(), index{}, "/", "My App",
//	    WithDefaultComponentSelector(func(r *http.Request, pn *PageNode) (string, error) {
//	        if r.Header.Get("HX-Request") == "true" {
//	            return "Content", nil  // Skip layout, render just content
//	        }
//	        return "Page", nil  // Full page with layout
//	    }))
func WithDefaultComponentSelector(selector func(r *http.Request, pn *PageNode) (string, error)) func(*StructPages) {
	return func(r *StructPages) {
		r.defaultComponentSelector = selector
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

	// If args are provided, use them as component args; otherwise use provided props
	componentArgs := props
	if len(renderErr.args) > 0 {
		// Convert args to reflect.Values
		componentArgs = make([]reflect.Value, len(renderErr.args))
		for i, arg := range renderErr.args {
			componentArgs[i] = reflect.ValueOf(arg)
		}
	}

	// Extract method info from method expression
	methodName := extractMethodName(renderErr.method)
	if methodName == "" {
		sp.onError(w, r, fmt.Errorf("failed to extract method name from method expression"))
		return true
	}

	receiverType := extractReceiverType(renderErr.method)
	if receiverType == nil {
		sp.onError(w, r, fmt.Errorf("failed to extract receiver type from method expression"))
		return true
	}

	// Find the page that owns this component
	targetPage, err := sp.pc.findPageNodeByType(receiverType)
	if err != nil {
		sp.onError(w, r, fmt.Errorf("cannot find page for method expression: %w", err))
		return true
	}

	// Look up the component method
	compMethod, ok := targetPage.Components[methodName]
	if !ok {
		sp.onError(w, r, fmt.Errorf("component %s not found in page %s", methodName, targetPage.Name))
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
}

func (sp *StructPages) buildHandler(page *PageNode) http.Handler {
	if h := sp.asHandler(page); h != nil {
		return h
	}
	if len(page.Components) == 0 {
		return nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Determine which component to render (before calling Props)
		compMethod, err := sp.findComponent(page, r)
		hasProps := len(page.Props) > 0

		// Handle error finding component
		if err != nil {
			if !hasProps {
				// No component and no Props to select one
				sp.onError(w, r, fmt.Errorf("error finding component for %s: %w", page.Name, err))
				return
			}
			// Allow Props to select component via RenderComponent
			compMethod = reflect.Method{} // Empty method - Props must use RenderComponent
		} else if !compMethod.Func.IsValid() {
			// Component found but Func is invalid
			if !hasProps {
				sp.onError(w, r, fmt.Errorf("page %s does not have a Page component method", page.Name))
				return
			}
			// Allow Props to select component via RenderComponent
			compMethod = reflect.Method{} // Empty method - Props must use RenderComponent
		}

		// 2. Create RenderTarget with the selected component info
		renderTarget := &RenderTarget{
			selectedMethod: compMethod,
		}

		// 3. Call Props with RenderTarget available for injection
		props, err := sp.execProps(page, r, w, renderTarget)
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

		// 4. If we still don't have a valid component, Props must have forgotten to use RenderComponent
		if !compMethod.Func.IsValid() {
			sp.onError(w, r, fmt.Errorf("page %s: no component found and Props did not use RenderComponent", page.Name))
			return
		}

		// 5. Render the selected component with props
		comp, err := sp.pc.callComponentMethod(page, &compMethod, props...)
		if err != nil {
			sp.onError(w, r, fmt.Errorf("error calling component %s.%s: %w", page.Name, compMethod.Name, err))
			return
		}
		sp.render(w, r, comp)
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
	errorType      = reflect.TypeOf((*error)(nil)).Elem()
	handlerType    = reflect.TypeOf((*http.Handler)(nil)).Elem()
	errHandlerType = reflect.TypeOf((*httpErrHandler)(nil)).Elem()
)

func extractError(args []reflect.Value) ([]reflect.Value, error) {
	if len(args) >= 1 && args[len(args)-1].Type().AssignableTo(errorType) {
		i := args[len(args)-1].Interface()
		args = args[:len(args)-1]
		if i == nil {
			return args, nil
		}
		return args, i.(error)
	}
	return args, nil
}

func formatMethod(method *reflect.Method) string {
	if method == nil || !method.Func.IsValid() {
		return "<nil>"
	}
	receiver := method.Type.In(0)
	if receiver.Kind() == reflect.Pointer {
		receiver = receiver.Elem()
	}
	return fmt.Sprintf("%s.%s", receiver.String(), method.Name)
}

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

			// Create RenderTarget for dependency injection
			// If components exist and one is selected, populate it; otherwise leave empty
			renderTarget := &RenderTarget{}
			if len(pn.Components) > 0 {
				if compMethod, err := sp.findComponent(pn, r); err == nil && compMethod.Func.IsValid() {
					renderTarget.selectedMethod = compMethod
				}
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

func (sp *StructPages) findComponent(pn *PageNode, r *http.Request) (reflect.Method, error) {
	// Use default component selector if configured (e.g., for HTMX boost pattern)
	if sp.defaultComponentSelector != nil {
		name, err := sp.defaultComponentSelector(r, pn)
		if err != nil {
			return reflect.Method{}, fmt.Errorf("error calling default component selector for %s: %w", pn.Name, err)
		}
		comp, ok := pn.Components[name]
		if !ok {
			return reflect.Method{}, fmt.Errorf(
				"default component selector for %s returned unknown component name: %s",
				pn.Name, name)
		}
		return comp, nil
	}

	// Default to "Page" component
	page, ok := pn.Components["Page"]
	if !ok {
		return reflect.Method{}, fmt.Errorf("no Page component found for %s", pn.Name)
	}
	return page, nil
}

func (sp *StructPages) execProps(pn *PageNode,
	r *http.Request, w http.ResponseWriter, renderTarget *RenderTarget,
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
	props, err := sp.pc.callMethod(
		pn, &propMethod,
		reflect.ValueOf(r), reflect.ValueOf(w), reflect.ValueOf(renderTarget))
	if err != nil {
		return nil, fmt.Errorf("error calling Props method %s.Props: %w", pn.Name, err)
	}
	return extractError(props)
}
