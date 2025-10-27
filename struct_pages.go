package structpages

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
)

// ErrSkipPageRender is a sentinel error that can be returned from a Props method
// to indicate that the page rendering should be skipped. This is useful for
// implementing conditional rendering or redirects within page logic.
var ErrSkipPageRender = errors.New("skip page render")

// ComponentSelection contains information about which component was selected
// for rendering. It's available to Props methods via dependency injection.
//
// Example usage in Props:
//
//	func (p index) Props(r *http.Request, sel *structpages.ComponentSelection) ([]Todo, error) {
//	    switch {
//	    case sel.Selected(index.TodoList):
//	        return getTodos(), nil
//	    case sel.Selected(index.Page):
//	        return getAllData(), nil
//	    default:
//	        return getAllData(), nil
//	    }
//	}
type ComponentSelection struct {
	selectedMethod reflect.Method // The actual component method that was selected
}

// Selected returns true if the given method expression matches the selected component.
// Uses method expressions for compile-time safety and IDE refactoring support.
//
// Example:
//
//	if sel.Selected(index.TodoList) {
//	    // TodoList component is being rendered
//	}
func (cs *ComponentSelection) Selected(method any) bool {
	methodName := extractMethodName(method)
	if methodName == "" {
		return false
	}

	// Compare both method name and receiver type for more robust matching
	receiverType := extractReceiverType(method)
	if receiverType == nil {
		return false
	}

	selectedReceiverType := cs.selectedMethod.Type.In(0)
	if selectedReceiverType.Kind() == reflect.Pointer {
		selectedReceiverType = selectedReceiverType.Elem()
	}

	return cs.selectedMethod.Name == methodName &&
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
}

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
func Mount(mux Mux, page any, route, title string, options ...any) (*StructPages, error) {
	if mux == nil {
		mux = http.DefaultServeMux
	}

	sp := &StructPages{
		onError: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		},
		defaultComponentSelector: HTMXPageConfig,
	}

	// Separate options from dependency injection args
	var args []any
	for _, opt := range options {
		if fn, ok := opt.(func(*StructPages)); ok {
			fn(sp)
		} else {
			args = append(args, opt)
		}
	}

	// Parse page tree
	pc, err := parsePageTree(route, page, args...)
	if err != nil {
		return nil, err
	}
	pc.root.Title = title
	sp.pc = pc

	// Register all pages
	middlewares := append([]MiddlewareFunc{withPcCtx(pc), extractURLParams}, sp.middlewares...)
	if err := sp.registerPageItem(mux, pc, pc.root, middlewares); err != nil {
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
//	router.URLFor([]any{Page{}, "?foo={bar}"}, "bar", "baz")
//
// It also supports a func(*PageNode) bool as the Page argument to match a specific page.
// It can be useful when you have multiple pages with the same type but different routes.
func (r *StructPages) URLFor(page any, args ...any) (string, error) {
	var pattern string
	parts, ok := page.([]any)
	if !ok {
		parts = []any{page}
	}
	for _, page := range parts {
		if s, ok := page.(string); ok {
			pattern += s
		} else {
			p, err := r.pc.urlFor(page)
			if err != nil {
				return "", err
			}
			pattern += p
		}
	}
	path, err := formatPathSegments(context.Background(), pattern, args...)
	if err != nil {
		return "", fmt.Errorf("urlfor: %w", err)
	}
	return strings.Replace(path, "{$}", "", 1), nil
}

// IDFor generates a consistent HTML ID or CSS selector for a component method.
// It works without context by using the router's parseContext directly.
//
// By default, returns a CSS selector (with "#" prefix) for use in HTMX targets.
// The ID includes the page name prefix to avoid conflicts across pages.
//
// Basic usage (returns CSS selector):
//
//	router.IDFor(p.UserList)
//	// → "#team-management-view-user-list"
//
//	router.IDFor(TeamManagementView{}.GroupMembers)
//	// → "#team-management-view-group-members"
//
// Advanced usage with IDParams:
//
//	// For id attribute (raw ID without "#")
//	router.IDFor(IDParams{
//	    Method: p.UserList,
//	    RawID: true,
//	})
//	// → "team-management-view-user-list"
//
//	// With suffixes for compound IDs
//	router.IDFor(IDParams{
//	    Method: p.UserModal,
//	    Suffixes: []string{"container"},
//	})
//	// → "#team-management-view-user-modal-container"
func (r *StructPages) IDFor(v any) (string, error) {
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
	pn, err := r.pc.findPageNodeByType(receiverType)
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

// WithDefaultComponentSelector sets a global component selector function that determines
// which component to render when RenderComponent is not explicitly called in Props.
// This is useful for implementing common patterns like HTMX partial rendering across all pages.
//
// The selector function receives the request and page node, and returns the component method name.
// For example, returning "Content" will render the Content() method instead of Page().
//
// Example - HTMX boost pattern:
//
//	router := Mount(http.NewServeMux(), index{}, "/", "My App",
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
//	router := structpages.Mount(
//		http.NewServeMux(), index{}, "/", "App",
//		structpages.WithWarnEmptyRoute(nil),
//	)
//
//	// Custom warning function
//	router := structpages.Mount(
//		http.NewServeMux(), index{}, "/", "App",
//		structpages.WithWarnEmptyRoute(func(pn *PageNode) {
//			log.Printf("Skipping empty page: %s", pn.Name)
//		}),
//	)
//
//	// Suppress warnings entirely
//	router := structpages.Mount(
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

func (sp *StructPages) registerPageItem(mux Mux, pc *parseContext, page *PageNode, mw []MiddlewareFunc) error {
	if page.Route == "" {
		return fmt.Errorf("page item route is empty: %s", page.Name)
	}

	if page.Middlewares != nil {
		res, err := pc.callMethod(page, page.Middlewares)
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
			if err := sp.registerPageItem(mux, pc, child, mw); err != nil {
				return err
			}
		}
	}
	handler := sp.buildHandler(page, pc)
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
func (router *StructPages) handleRenderComponentError(
	w http.ResponseWriter, r *http.Request, err error, pc *parseContext, page *PageNode, props []reflect.Value,
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
		router.onError(w, r, fmt.Errorf("failed to extract method name from method expression"))
		return true
	}

	receiverType := extractReceiverType(renderErr.method)
	if receiverType == nil {
		router.onError(w, r, fmt.Errorf("failed to extract receiver type from method expression"))
		return true
	}

	// Find the page that owns this component
	targetPage, err := pc.findPageNodeByType(receiverType)
	if err != nil {
		router.onError(w, r, fmt.Errorf("cannot find page for method expression: %w", err))
		return true
	}

	// Look up the component method
	compMethod, ok := targetPage.Components[methodName]
	if !ok {
		router.onError(w, r, fmt.Errorf("component %s not found in page %s", methodName, targetPage.Name))
		return true
	}

	// Execute the component with the args
	comp, compErr := pc.callComponentMethod(targetPage, &compMethod, componentArgs...)
	if compErr != nil {
		router.onError(w, r, fmt.Errorf("error calling component %s.%s: %w", targetPage.Name, compMethod.Name, compErr))
		return true
	}

	router.render(w, r, comp)
	return true
}

func (router *StructPages) buildHandler(page *PageNode, pc *parseContext) http.Handler {
	if h := router.asHandler(pc, page); h != nil {
		return h
	}
	if len(page.Components) == 0 {
		return nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Determine which component to render (before calling Props)
		compMethod, err := router.findComponent(pc, page, r)
		if err != nil {
			router.onError(w, r, fmt.Errorf("error finding component for %s: %w", page.Name, err))
			return
		}

		if !compMethod.Func.IsValid() {
			router.onError(w, r, fmt.Errorf("page %s does not have a Page component method", page.Name))
			return
		}

		// 2. Create ComponentSelection with the selected component info
		componentSelection := &ComponentSelection{
			selectedMethod: compMethod,
		}

		// 3. Call Props with ComponentSelection available for injection
		props, err := router.execProps(pc, page, r, w, componentSelection)
		if err != nil {
			// Check if it's a render component error
			if router.handleRenderComponentError(w, r, err, pc, page, props) {
				return
			}

			if errors.Is(err, ErrSkipPageRender) {
				return
			}
			router.onError(w, r, fmt.Errorf("error running props for %s: %w", page.Name, err))
			return
		}

		// 4. Render the selected component with props
		comp, err := pc.callComponentMethod(page, &compMethod, props...)
		if err != nil {
			router.onError(w, r, fmt.Errorf("error calling component %s.%s: %w", page.Name, compMethod.Name, err))
			return
		}
		router.render(w, r, comp)
	})
}

func (router *StructPages) render(w http.ResponseWriter, r *http.Request, comp component) {
	buf := getBuffer()
	defer releaseBuffer(buf)
	if err := comp.Render(r.Context(), buf); err != nil {
		router.onError(w, r, err)
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

func (router *StructPages) asHandler(pc *parseContext, pn *PageNode) http.Handler {
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
				if router.handleRenderComponentError(bw, r, err, pc, pn, nil) {
					return
				}
				// Write error directly to the buffered writer
				router.onError(bw, r, err)
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
			results, err := pc.callMethod(pn, &method, wv, reflect.ValueOf(r))
			if err != nil {
				if bw != nil {
					bw.buf.Reset()
					router.onError(bw, r, fmt.Errorf("error calling ServeHTTP method on %s: %w", pn.Name, err))
				} else {
					router.onError(w, r, fmt.Errorf("error calling ServeHTTP method on %s: %w", pn.Name, err))
				}
				return
			}
			_, err = extractError(results)
			if err != nil {
				if bw != nil {
					bw.buf.Reset()
					// Check if it's a render component error
					if router.handleRenderComponentError(bw, r, err, pc, pn, nil) {
						return
					}
					router.onError(bw, r, err)
				} else {
					// if bw is nil it means we didn't need to buffer, i.e. the handler doesn't return error
					router.onError(w, r, err)
				}
				return
			}
		})
	}

	return nil
}

func (router *StructPages) findComponent(pc *parseContext, pn *PageNode, r *http.Request) (reflect.Method, error) {
	// Use default component selector if configured (e.g., for HTMX boost pattern)
	if router.defaultComponentSelector != nil {
		name, err := router.defaultComponentSelector(r, pn)
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

func (router *StructPages) execProps(pc *parseContext, pn *PageNode,
	r *http.Request, w http.ResponseWriter, componentSelection *ComponentSelection,
) ([]reflect.Value, error) {
	// Look for Props method
	propMethod, ok := pn.Props["Props"]
	if !ok {
		return nil, nil
	}

	if propMethod.Func.IsValid() {
		// Make ComponentSelection available for injection along with r and w
		props, err := pc.callMethod(
			pn, &propMethod,
			reflect.ValueOf(r), reflect.ValueOf(w), reflect.ValueOf(componentSelection))
		if err != nil {
			return nil, fmt.Errorf("error calling Props method %s.Props: %w", pn.Name, err)
		}
		return extractError(props)
	}
	return nil, nil
}
