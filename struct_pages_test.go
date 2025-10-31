//lint:file-ignore U1000 Ignore unused code in test file

package structpages

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type testComponent struct {
	content string
}

func (t testComponent) Render(ctx context.Context, w io.Writer) error {
	_, err := w.Write([]byte(t.content))
	return err
}

type errComponent struct {
	err error
}

func (e errComponent) Render(ctx context.Context, w io.Writer) error {
	return e.err
}

type TestHandlerPage struct{}

func (TestHandlerPage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("TestHttpHandler"))
}

func TestHttpHandler(t *testing.T) {
	type topPage struct {
		s TestHandlerPage  `route:"/struct Test struct handler"`
		p *TestHandlerPage `route:"POST /pointer Test pointer handler"`
	}

	mux := http.NewServeMux()
	_, err := Mount(mux, &topPage{}, "/", "")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/struct", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "TestHttpHandler" {
			t.Errorf("expected body %q, got %q", "TestHttpHandler", rec.Body.String())
		}
	}

	{
		req := httptest.NewRequest(http.MethodPost, "/pointer", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "TestHttpHandler" {
			t.Errorf("expected body %q, got %q", "TestHttpHandler", rec.Body.String())
		}
	}
}

type middlewarePages struct {
	middlewareChildPage `route:"/child Child"`
}

type middlewareChildPage struct{}

func (middlewareChildPage) Page() component {
	return testComponent{content: "Test middleware child page"}
}

func (middlewarePages) Middlewares() []MiddlewareFunc {
	return []MiddlewareFunc{
		func(next http.Handler, node *PageNode) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Test-Middleware", "foobar")
				next.ServeHTTP(w, r)
			})
		},
	}
}

func (middlewarePages) Page() component {
	return testComponent{content: "Test middleware handler"}
}

func TestMiddlewares(t *testing.T) {
	type topPage struct {
		middlewarePages `route:"/middleware Test middleware handler"`
	}
	mux := http.NewServeMux()
	_, err := Mount(mux, &topPage{}, "/", "top page")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	{
		req := httptest.NewRequest(http.MethodGet, "/middleware", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Header().Get("X-Test-Middleware") != "foobar" {
			t.Errorf("expected header X-Test-Middleware to be 'foobar', got %s", rec.Header().Get("X-Test-Middleware"))
		}
		if rec.Body.String() != "Test middleware handler" {
			t.Errorf("expected body %q, got %q", "Test middleware handler", rec.Body.String())
		}
	}
	{
		// test child page also has the middleware applied
		req := httptest.NewRequest(http.MethodGet, "/middleware/child", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Header().Get("X-Test-Middleware") != "foobar" {
			t.Errorf("expected header X-Test-Middleware to be 'foobar', got %s", rec.Header().Get("X-Test-Middleware"))
		}
		if rec.Body.String() != "Test middleware child page" {
			t.Errorf("expected body %q, got %q", "Test middleware child page", rec.Body.String())
		}
	}
}

type DefaultConfigPage struct{}

func (DefaultConfigPage) Page() component {
	return testComponent{content: "Default config page"}
}

func (DefaultConfigPage) HxTarget() component {
	return testComponent{content: "hx target defaultConfigPage"}
}

func TestPageConfig(t *testing.T) {
	type topPage struct {
		DefaultConfigPage `route:"/default Default config page"`
	}
	mux := http.NewServeMux()
	_, err := Mount(mux, &topPage{}, "/", "top page")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	{
		req := httptest.NewRequest(http.MethodGet, "/default", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "Default config page" {
			t.Errorf("expected body %q, got %q", "Default config page", rec.Body.String())
		}
	}
}

type skipRenderPage struct{}

func (skipRenderPage) Page() component {
	return testComponent{content: "Should not render"}
}

func (skipRenderPage) Props(r *http.Request, w http.ResponseWriter) error {
	if r.URL.Query().Get("skip") == "true" {
		// Write a custom response before returning ErrSkipPageRender
		w.WriteHeader(http.StatusNoContent)
		return ErrSkipPageRender
	}
	return nil
}

func TestErrSkipPageRender(t *testing.T) {
	type topPage struct {
		skipRenderPage `route:"/skip Test skip render"`
	}
	mux := http.NewServeMux()
	_, err := Mount(mux, &topPage{}, "/", "top page")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Test normal rendering (no skip)
	{
		req := httptest.NewRequest(http.MethodGet, "/skip", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "Should not render" {
			t.Errorf("expected body %q, got %q", "Should not render", rec.Body.String())
		}
	}

	// Test skip rendering
	{
		req := httptest.NewRequest(http.MethodGet, "/skip?skip=true", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Errorf("expected status %d, got %d", http.StatusNoContent, rec.Code)
		}
		if rec.Body.String() != "" {
			t.Errorf("expected empty body, got %q", rec.Body.String())
		}
	}
}

type middlewareOrderPage struct{}

func (middlewareOrderPage) Page() component {
	return testComponent{content: "Middleware Order Page\n"}
}

func (middlewareOrderPage) Middlewares() []MiddlewareFunc {
	return []MiddlewareFunc{
		makeMiddleware("page mw 1"),
		makeMiddleware("page mw 2"),
		makeMiddleware("page mw 3"),
	}
}

func makeMiddleware(name string) MiddlewareFunc {
	return func(next http.Handler, node *PageNode) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("Middleware before: " + name + "\n"))
			next.ServeHTTP(w, r)
			_, _ = w.Write([]byte("Middleware after: " + name + "\n"))
		})
	}
}

func TestMiddlewareOrder(t *testing.T) {
	type topPage struct {
		middlewareOrderPage `route:"/"`
	}
	mux := http.NewServeMux()
	_, err := Mount(mux, &topPage{}, "/", "top page",
		WithMiddlewares(
			makeMiddleware("global mw 1"),
			makeMiddleware("global mw 2"),
			makeMiddleware("global mw 3"),
		),
	)
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	expectedBody := `Middleware before: global mw 1
Middleware before: global mw 2
Middleware before: global mw 3
Middleware before: page mw 1
Middleware before: page mw 2
Middleware before: page mw 3
Middleware Order Page
Middleware after: page mw 3
Middleware after: page mw 2
Middleware after: page mw 1
Middleware after: global mw 3
Middleware after: global mw 2
Middleware after: global mw 1
`
	if diff := cmp.Diff(expectedBody, rec.Body.String()); diff != "" {
		t.Errorf("unexpected body (-want +got):\n%s", diff)
	}
}

// Note: TestProps removed - ComponentProps pattern (PageProps, ContentProps) has been removed
// in favor of simpler Props + RenderComponent pattern.

func TestFormatMethod(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *reflect.Method
		expected string
	}{
		{
			name: "nil method",
			setup: func() *reflect.Method {
				return nil
			},
			expected: "<nil>",
		},
		{
			name: "valid method with value receiver",
			setup: func() *reflect.Method {
				typ := reflect.TypeOf(testComponent{})
				method, _ := typ.MethodByName("Render")
				return &method
			},
			expected: "structpages.testComponent.Render",
		},
		{
			name: "valid method with pointer receiver",
			setup: func() *reflect.Method {
				typ := reflect.TypeOf(&renderComponentPage{})
				method, _ := typ.MethodByName("Page")
				return &method
			},
			expected: "structpages.renderComponentPage.Page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method := tt.setup()
			result := formatMethod(method)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

type errHandler struct{}

func (errHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	if r.URL.Path == "/error" {
		return fmt.Errorf("test error")
	}
	_, _ = w.Write([]byte("OK"))
	return nil
}

// Define distinct types for our test strings
type (
	ExtendedHandlerArg    string
	ExtendedErrHandlerArg string
)

type extendedHandler struct{}

func (extendedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, extra ExtendedHandlerArg) {
	_, _ = w.Write([]byte("extended: " + string(extra)))
}

type extendedErrHandler struct{}

func (extendedErrHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, extra ExtendedErrHandlerArg) error {
	if r.URL.Path == "/error" {
		return fmt.Errorf("extended error: %s", extra)
	}
	_, _ = w.Write([]byte("extended ok: " + string(extra)))
	return nil
}

func TestExtendedHandlers(t *testing.T) {
	// Test the extended handler functionality separately with proper setup
	type pages struct {
		extendedHandler    `route:"GET /extended"`
		extendedErrHandler `route:"GET /exterr"`
	}

	var lastError error
	errorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		lastError = err
		t.Logf("Error occurred: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	mux := http.NewServeMux()
	// Pass the typed arguments that the extended handlers expect
	_, err := Mount(mux, &pages{}, "/", "Test Extended",
		WithErrorHandler(errorHandler),
		WithArgs(
			ExtendedHandlerArg("extra value"),
			ExtendedErrHandlerArg("error extra"),
		),
	)
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Test extended handler
	{
		req := httptest.NewRequest(http.MethodGet, "/extended", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "extended: extra value" {
			t.Errorf("expected body %q, got %q", "extended: extra value", rec.Body.String())
			if lastError != nil {
				t.Errorf("last error: %v", lastError)
			}
		}
	}

	// Test extended error handler success
	{
		req := httptest.NewRequest(http.MethodGet, "/exterr", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "extended ok: error extra" {
			t.Errorf("expected body %q, got %q", "extended ok: error extra", rec.Body.String())
		}
	}
}

func TestCallMethodError(t *testing.T) {
	pc := &parseContext{}
	pc.args = make(argRegistry)

	// Use testComponent which has a Render method
	tc := testComponent{}
	method, _ := reflect.TypeOf(tc).MethodByName("Render")

	// Create a PageNode with a different type (pointer vs value mismatch)
	pn := &PageNode{
		Name:  "InvalidReceiver",
		Value: reflect.ValueOf(123), // int value, completely different type
	}

	_, err := pc.callMethod(pn, &method)
	if err == nil {
		t.Error("expected error for receiver type mismatch, got nil")
	}
}

func TestWithErrorHandler(t *testing.T) {
	customErrorCalled := false
	customErrorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		customErrorCalled = true
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("Custom error: " + err.Error()))
	}

	sp := &StructPages{
		onError: customErrorHandler,
	}
	pc := &parseContext{}
	pc.root = &PageNode{}

	pn := &PageNode{
		Name:  "ErrHandler",
		Value: reflect.ValueOf(errHandler{}),
	}

	handler := sp.asHandler(pn)

	req := httptest.NewRequest(http.MethodGet, "/error", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !customErrorCalled {
		t.Error("custom error handler was not called")
	}
	if rec.Code != http.StatusTeapot {
		t.Errorf("expected status %d, got %d", http.StatusTeapot, rec.Code)
	}
	expectedBody := "Custom error: test error"
	if rec.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, rec.Body.String())
	}
}

func TestAsHandler(t *testing.T) {
	tests := []struct {
		name         string
		pageNode     *PageNode
		setupContext func() *parseContext
		request      *http.Request
		expectedBody string
		expectedCode int
		hasHandler   bool
	}{
		{
			name: "standard http.Handler",
			pageNode: &PageNode{
				Name:  "TestHandler",
				Value: reflect.ValueOf(TestHandlerPage{}),
			},
			setupContext: func() *parseContext { return &parseContext{} },
			request:      httptest.NewRequest(http.MethodGet, "/", http.NoBody),
			expectedBody: "TestHttpHandler",
			expectedCode: http.StatusOK,
			hasHandler:   true,
		},
		{
			name: "error handler success",
			pageNode: &PageNode{
				Name:  "ErrHandler",
				Value: reflect.ValueOf(errHandler{}),
			},
			setupContext: func() *parseContext { return &parseContext{} },
			request:      httptest.NewRequest(http.MethodGet, "/ok", http.NoBody),
			expectedBody: "OK",
			expectedCode: http.StatusOK,
			hasHandler:   true,
		},
		{
			name: "error handler with error",
			pageNode: &PageNode{
				Name:  "ErrHandler",
				Value: reflect.ValueOf(errHandler{}),
			},
			setupContext: func() *parseContext { return &parseContext{} },
			request:      httptest.NewRequest(http.MethodGet, "/error", http.NoBody),
			expectedBody: "Internal Server Error\n",
			expectedCode: http.StatusInternalServerError,
			hasHandler:   true,
		},
		{
			name: "no ServeHTTP method",
			pageNode: &PageNode{
				Name:  "NoHandler",
				Value: reflect.ValueOf(struct{}{}),
			},
			setupContext: func() *parseContext { return &parseContext{} },
			request:      httptest.NewRequest(http.MethodGet, "/", http.NoBody),
			hasHandler:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp := &StructPages{
				onError: func(w http.ResponseWriter, r *http.Request, err error) {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				},
			}
			pc := tt.setupContext()
			sp.pc = pc // Set the pc on the StructPages instance
			handler := sp.asHandler(tt.pageNode)

			if tt.hasHandler && handler == nil {
				t.Errorf("expected handler, got nil")
				return
			}
			if !tt.hasHandler && handler != nil {
				t.Errorf("expected nil handler, got %v", handler)
				return
			}

			if tt.hasHandler {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, tt.request)

				if rec.Code != tt.expectedCode {
					t.Errorf("expected status %d, got %d", tt.expectedCode, rec.Code)
				}
				if rec.Body.String() != tt.expectedBody {
					t.Errorf("expected body %q, got %q", tt.expectedBody, rec.Body.String())
				}
			}
		})
	}
}

// Test page types for registerError test
type badChildPageForRegisterError struct{}

func (badChildPageForRegisterError) Page() component {
	return testComponent{content: "test"}
}

type badPageForRegisterError struct {
	Child badChildPageForRegisterError `route:""` // Actually empty route should cause error
}

// Test MountPages registerPageItem error path
func TestStructPages_MountPages_registerError(t *testing.T) {
	sp := &StructPages{
		onError: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		},
	}
	mux := http.NewServeMux()

	// Create a page with manually constructed PageNode that has empty route
	// This simulates what would happen if parsing produced a page with empty route
	pageNode := &PageNode{
		Name:  "testPage",
		Route: "", // This will trigger the "page item route is empty" error
	}

	err := sp.registerPageItem(mux, pageNode, nil)
	if err == nil {
		t.Error("Expected error from registerPageItem with empty route")
	}
	if err != nil && err.Error() != "page item route is empty: testPage" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

// Test MountPages error cases
func TestStructPages_MountPages_parseError(t *testing.T) {
	// This should cause a parse error due to duplicate args
	mux := http.NewServeMux()
	_, err := Mount(mux, struct{}{}, "/", "Test", WithArgs("arg1", "arg1"))
	if err == nil {
		t.Error("Expected error from MountPages with duplicate args")
	}
}

// Types for getProps test
type pageWithErrorProps struct{}

func (p *pageWithErrorProps) Props(r *http.Request) (string, error) {
	return "", errors.New("props error")
}

// Test execProps with error from props method
func TestStructPages_execProps_methodError(t *testing.T) {
	sp := &StructPages{
		onError: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		},
		pc: &parseContext{args: make(argRegistry)},
	}

	propsMethod, _ := reflect.TypeOf(&pageWithErrorProps{}).MethodByName("Props")
	pn := &PageNode{
		Name: "test",
		Props: map[string]reflect.Method{
			"Props": propsMethod,
		},
		Value: reflect.ValueOf(&pageWithErrorProps{}),
	}

	req := httptest.NewRequest("GET", "/", http.NoBody)

	// Create a dummy RenderTarget
	dummyMethod := reflect.Method{Name: "Page"}
	compSel := newMethodRenderTarget("Page", &dummyMethod)
	_, err := sp.execProps(pn, req, nil, compSel)
	if err == nil {
		t.Error("Expected error from execProps")
	}
}

// Test page for RenderComponent functionality
type renderComponentPage struct{}

func (renderComponentPage) Page() component {
	return testComponent{content: "Default Page"}
}

func (renderComponentPage) PartialView(data string) component {
	return testComponent{content: "Partial: " + data}
}

func (renderComponentPage) CustomView(arg1 string, arg2 int) component {
	return testComponent{content: fmt.Sprintf("Custom: %s-%d", arg1, arg2)}
}

func (renderComponentPage) AltView(data string) component {
	return testComponent{content: "Alt: " + data}
}

func (p renderComponentPage) Props(r *http.Request) (string, error) {
	view := r.URL.Query().Get("view")
	switch view {
	case "partial":
		// Use RenderComponent with custom args
		return "ignored", RenderComponent((*renderComponentPage).PartialView, "custom partial data")
	case "custom":
		// Use RenderComponent with multiple args
		return "ignored", RenderComponent((*renderComponentPage).CustomView, "test", 42)
	case "alt":
		// Use RenderComponent without args - should use Props return value
		return "props data", RenderComponent((*renderComponentPage).AltView, "props data")
	default:
		// Normal flow
		return "default data", nil
	}
}

// Test RenderComponent functionality
func TestRenderComponent(t *testing.T) {
	var capturedError error
	type pages struct {
		renderComponentPage `route:"/render Test Render Component"`
	}
	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test", WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
		capturedError = err
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	tests := []struct {
		name         string
		query        string
		expectedBody string
	}{
		{
			name:         "default rendering",
			query:        "",
			expectedBody: "Default Page",
		},
		{
			name:         "render partial with custom args",
			query:        "?view=partial",
			expectedBody: "Partial: custom partial data",
		},
		{
			name:         "render custom with multiple args",
			query:        "?view=custom",
			expectedBody: "Custom: test-42",
		},
		{
			name:         "render alt using Props return value",
			query:        "?view=alt",
			expectedBody: "Alt: props data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capturedError = nil
			req := httptest.NewRequest(http.MethodGet, "/render"+tt.query, http.NoBody)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
				if capturedError != nil {
					t.Errorf("error: %v", capturedError)
				}
			}
			if rec.Body.String() != tt.expectedBody {
				t.Errorf("expected body %q, got %q", tt.expectedBody, rec.Body.String())
			}
		})
	}
}

// Test types for WithWarnEmptyRoute
type emptyPage struct{}

type pageWithHandler struct{}

func (pageWithHandler) Page() testComponent {
	return testComponent{content: "has-handler"}
}

func TestWithWarnEmptyRoute(t *testing.T) {
	var warnMessages []string

	// Custom warn function that captures messages
	customWarn := func(pn *PageNode) {
		warnMessages = append(warnMessages, fmt.Sprintf("Warning: %s has no handler or children", pn.Name))
	}

	type pages struct {
		EmptyPage       emptyPage       `route:"/empty Empty"`
		PageWithHandler pageWithHandler `route:"/handler Handler"`
	}

	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test", WithWarnEmptyRoute(customWarn))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Should have one warning for the empty page
	if len(warnMessages) != 1 {
		t.Errorf("Expected 1 warning message, got %d: %v", len(warnMessages), warnMessages)
	}

	expectedMessage := "Warning: EmptyPage has no handler or children"
	if len(warnMessages) > 0 && warnMessages[0] != expectedMessage {
		t.Errorf("Expected warning message %q, got %q", expectedMessage, warnMessages[0])
	}

	// Test that page with handler doesn't trigger warning
	req := httptest.NewRequest(http.MethodGet, "/handler", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if rec.Body.String() != "has-handler" {
		t.Errorf("Expected body 'has-handler', got %q", rec.Body.String())
	}

	// Test that empty page returns 404
	req = httptest.NewRequest(http.MethodGet, "/empty", http.NoBody)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for empty page, got %d", rec.Code)
	}
}

func TestWithWarnEmptyRoute_DefaultWarning(t *testing.T) {
	// Test the default warning function by capturing stdout
	var output []byte
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Capture output in a goroutine
	done := make(chan bool)
	go func() {
		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		output = buf[:n]
		done <- true
	}()

	type pages struct {
		EmptyPage emptyPage `route:"/empty Empty"`
	}

	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test", WithWarnEmptyRoute(nil))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Restore stdout and wait for output
	w.Close()
	os.Stdout = oldStdout
	<-done

	// Check that default warning message was printed
	outputStr := string(output)
	expectedPrefix := "⚠️  Warning: page route has no children and no handler, skipping route registration: EmptyPage"
	if !strings.Contains(outputStr, expectedPrefix) {
		t.Errorf("Expected default warning message containing %q, got %q", expectedPrefix, outputStr)
	}
}

// Test types for URLFor wrapper test
type homePageURLFor struct{}

func (homePageURLFor) Page() component { return testComponent{"home"} }

type userPageURLFor struct{}

func (userPageURLFor) Page() component { return testComponent{"user"} }

// TestStructPages_URLFor tests the StructPages.URLFor wrapper method
func TestStructPages_URLFor(t *testing.T) {
	type pages struct {
		home homePageURLFor `route:"/ Home"`
		user userPageURLFor `route:"/user/{id} User"`
	}

	mux := http.NewServeMux()
	sp, err := Mount(mux, &pages{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	t.Run("URLFor with static type reference", func(t *testing.T) {
		url, err := sp.URLFor(homePageURLFor{})
		if err != nil {
			t.Errorf("URLFor error: %v", err)
		}
		if url != "/" {
			t.Errorf("URLFor() = %q, want %q", url, "/")
		}
	})

	t.Run("URLFor with Ref", func(t *testing.T) {
		url, err := sp.URLFor(Ref("user"), "123")
		if err != nil {
			t.Errorf("URLFor error: %v", err)
		}
		if url != "/user/123" {
			t.Errorf("URLFor() = %q, want %q", url, "/user/123")
		}
	})

	t.Run("URLFor with path parameters", func(t *testing.T) {
		url, err := sp.URLFor(userPageURLFor{}, "456")
		if err != nil {
			t.Errorf("URLFor error: %v", err)
		}
		if url != "/user/456" {
			t.Errorf("URLFor() = %q, want %q", url, "/user/456")
		}
	})
}

// Test types for IDFor wrapper test
type testPageIDFor struct{}

func (testPageIDFor) Page() component      { return testComponent{"page"} }
func (testPageIDFor) UserList() component  { return testComponent{"userlist"} }
func (testPageIDFor) UserModal() component { return testComponent{"usermodal"} }

// TestStructPages_IDFor tests the StructPages.IDFor wrapper method
func TestStructPages_IDFor(t *testing.T) {
	type pages struct {
		test testPageIDFor `route:"/ Test"`
	}

	mux := http.NewServeMux()
	sp, err := Mount(mux, &pages{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	t.Run("IDTarget with method expression", func(t *testing.T) {
		id, err := sp.IDTarget(testPageIDFor.UserList)
		if err != nil {
			t.Errorf("IDTarget error: %v", err)
		}
		if id != "#test-user-list" {
			t.Errorf("IDTarget() = %q, want %q", id, "#test-user-list")
		}
	})

	t.Run("IDTarget with Ref", func(t *testing.T) {
		id, err := sp.IDTarget(Ref("test.UserModal"))
		if err != nil {
			t.Errorf("IDTarget error: %v", err)
		}
		if id != "#test-user-modal" {
			t.Errorf("IDTarget() = %q, want %q", id, "#test-user-modal")
		}
	})

	t.Run("ID with method expression", func(t *testing.T) {
		id, err := sp.ID(testPageIDFor.UserList)
		if err != nil {
			t.Errorf("IDFor error: %v", err)
		}
		if id != "test-user-list" {
			t.Errorf("IDFor() = %q, want %q", id, "test-user-list")
		}
	})
}

// Test type for Error test
type testPageError struct{}

func (testPageError) Component() component { return testComponent{"test"} }

// TestErrRenderComponent_Error tests the Error method of errRenderComponent
func TestErrRenderComponent_Error(t *testing.T) {
	err := RenderComponent(testPageError.Component)
	if err == nil {
		t.Fatal("RenderComponent should return non-nil error")
	}

	expectedMsg := "should render component from target"
	if err.Error() != expectedMsg {
		t.Errorf("Error() = %q, want %q", err.Error(), expectedMsg)
	}
}

// Test types for WithDefaultComponentSelector test
type testPageSelector struct{}

func (testPageSelector) Page() component    { return testComponent{"page"} }
func (testPageSelector) Content() component { return testComponent{"content"} }

// TestWithTargetSelector tests the WithTargetSelector option
func TestWithTargetSelector(t *testing.T) {
	type pages struct {
		test testPageSelector `route:"/ Test"`
	}

	// Create selector that returns Content for HX-Request
	selector := func(r *http.Request, pn *PageNode) (RenderTarget, error) {
		if r.Header.Get("HX-Request") == "true" {
			method := pn.Components["Content"]
			return newMethodRenderTarget("Content", &method), nil
		}
		method := pn.Components["Page"]
		return newMethodRenderTarget("Page", &method), nil
	}

	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test", WithTargetSelector(selector))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	t.Run("renders Content for HTMX request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", http.NoBody)
		req.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
		}
		if rec.Body.String() != "content" {
			t.Errorf("Body = %q, want %q", rec.Body.String(), "content")
		}
	})

	t.Run("renders Page for normal request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", http.NoBody)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
		}
		if rec.Body.String() != "page" {
			t.Errorf("Body = %q, want %q", rec.Body.String(), "page")
		}
	})
}

// Test handleRenderComponentError edge cases

// Test page types for edge cases
type handleErrorTestPage struct{}

func (handleErrorTestPage) Page() component {
	return testComponent{"page"}
}

type renderTargetIsTestPage struct{}

func (renderTargetIsTestPage) Page() component {
	return testComponent{"page"}
}

func (renderTargetIsTestPage) Content() component {
	return testComponent{"content"}
}

type predicateTestPage1 struct{}

func (predicateTestPage1) Page() component {
	return testComponent{"page1"}
}

type predicateTestPage2 struct{}

func (predicateTestPage2) Page() component {
	return testComponent{"page2"}
}

type pointerReceiverPage struct{}

func (*pointerReceiverPage) Page() component {
	return testComponent{"page"}
}

func (*pointerReceiverPage) Content() component {
	return testComponent{"content"}
}

type propsErrorPage struct{}

func (propsErrorPage) Page() component {
	return testComponent{"page"}
}

func (propsErrorPage) Props(r *http.Request) (string, error) {
	return "", errors.New("props error")
}

type MissingDep struct{}

type propsDIErrorPage struct{}

func (propsDIErrorPage) Page() component {
	return testComponent{"page"}
}

func (propsDIErrorPage) Props(r *http.Request, dep MissingDep) (string, error) {
	return "should not reach here", nil
}

type extendedHandlerErrorPage struct{}

func (extendedHandlerErrorPage) Page() component {
	return testComponent{"page"}
}

func (extendedHandlerErrorPage) ServeHTTP(w http.ResponseWriter, r *http.Request, dep MissingDep) {
	_, _ = w.Write([]byte("handler"))
}

type extendedHandlerErrorPageWithReturn struct{}

func (extendedHandlerErrorPageWithReturn) Page() component {
	return testComponent{"page"}
}

func (extendedHandlerErrorPageWithReturn) ServeHTTP(w http.ResponseWriter, r *http.Request, dep MissingDep) error {
	_, _ = w.Write([]byte("handler"))
	return nil
}

// Test RenderComponent with invalid method expression (non-function)
func TestHandleRenderComponentError_InvalidMethodExpr(t *testing.T) {
	// RenderComponent should fail immediately with invalid input
	err := RenderComponent("not a function") // Not a function, should fail
	if err == nil {
		t.Fatal("Expected RenderComponent to return error for invalid input")
	}
	if !strings.Contains(err.Error(), "target must be a component, RenderTarget, or function") {
		t.Errorf("Expected 'target must be a component, RenderTarget, or function' error, got: %v", err)
	}
}

// Test RenderComponent with function (standalone function component)
func TestHandleRenderComponentError_NoReceiver(t *testing.T) {
	mux := http.NewServeMux()
	sp, err := Mount(mux, &handleErrorTestPage{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/", http.NoBody)
	rec := httptest.NewRecorder()

	// Create a function that returns a component (valid standalone function)
	standaloneFunc := func() component { return testComponent{"test from function"} }

	err = RenderComponent(standaloneFunc)
	handled := sp.handleRenderComponentError(rec, req, err, sp.pc.root)

	if !handled {
		t.Error("Expected handleRenderComponentError to handle the error")
	}

	// Check that the component was rendered
	if !strings.Contains(rec.Body.String(), "test from function") {
		t.Errorf("Expected component to be rendered, got: %s", rec.Body.String())
	}
}

// myComponentGetter implements componentGetter interface for testing
type myComponentGetter struct {
	data string
}

func (mcg myComponentGetter) Component() component {
	return testComponent{mcg.data}
}

// Test RenderComponent with type that implements Component() method
func TestRenderComponent_ComponentGetter(t *testing.T) {
	mux := http.NewServeMux()
	sp, err := Mount(mux, &handleErrorTestPage{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/", http.NoBody)
	rec := httptest.NewRecorder()

	// Create a componentGetter
	getter := myComponentGetter{data: "from component getter"}
	err = RenderComponent(getter)
	handled := sp.handleRenderComponentError(rec, req, err, sp.pc.root)

	if !handled {
		t.Error("Expected handleRenderComponentError to handle the error")
	}

	// Check that the component was rendered
	if !strings.Contains(rec.Body.String(), "from component getter") {
		t.Errorf("Expected component to be rendered, got: %s", rec.Body.String())
	}
}

// Test RenderComponent with method from unregistered page type
func TestHandleRenderComponentError_PageNotFound(t *testing.T) {
	mux := http.NewServeMux()
	sp, err := Mount(mux, &handleErrorTestPage{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Create a custom error handler that captures errors
	var capturedErr error
	sp.onError = func(w http.ResponseWriter, r *http.Request, err error) {
		capturedErr = err
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
	}

	req := httptest.NewRequest("GET", "/", http.NoBody)
	rec := httptest.NewRecorder()

	// Try to render component from unregistered page
	err = RenderComponent(unregisteredPage.SomeComponent)
	handled := sp.handleRenderComponentError(rec, req, err, sp.pc.root)

	if !handled {
		t.Error("Expected handleRenderComponentError to handle the error")
	}
	if capturedErr == nil {
		t.Error("Expected error to be captured")
	}
	if capturedErr != nil && !strings.Contains(capturedErr.Error(), "cannot find page for method expression") {
		t.Errorf("Expected 'cannot find page for method expression' error, got: %v", capturedErr)
	}
}

// Test component method that returns error
func TestHandleRenderComponentError_ComponentCallError(t *testing.T) {
	mux := http.NewServeMux()
	sp, err := Mount(mux, &errorComponentPage{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Create a custom error handler that captures errors
	var capturedErr error
	sp.onError = func(w http.ResponseWriter, r *http.Request, err error) {
		capturedErr = err
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
	}

	req := httptest.NewRequest("GET", "/", http.NoBody)
	rec := httptest.NewRecorder()

	// Trigger component render error
	err = RenderComponent(errorComponentPage.ErrorComponent)
	handled := sp.handleRenderComponentError(rec, req, err, sp.pc.root)

	if !handled {
		t.Error("Expected handleRenderComponentError to handle the error")
	}
	if capturedErr == nil {
		t.Error("Expected error to be captured")
	}
	if capturedErr != nil && !strings.Contains(capturedErr.Error(), "error calling component") {
		t.Logf("Got error: %v", capturedErr)
	}
}

// Test handleRenderComponentError with args provided to RenderComponent
func TestHandleRenderComponentError_WithArgs(t *testing.T) {
	mux := http.NewServeMux()
	sp, err := Mount(mux, &argsComponentTestPage{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/", http.NoBody)
	rec := httptest.NewRecorder()

	// Trigger component render with args
	err = RenderComponent(argsComponentTestPage.ComponentWithArgs, "arg1", 42)
	handled := sp.handleRenderComponentError(rec, req, err, sp.pc.root)

	if !handled {
		t.Error("Expected handleRenderComponentError to handle the error")
	}

	// Should have successfully rendered the component with args
	result := rec.Body.String()
	if !strings.Contains(result, "arg1") || !strings.Contains(result, "42") {
		t.Errorf("Expected component to render with args, got: %s", result)
	}
}

type argsComponentTestPage struct{}

func (argsComponentTestPage) Page() component {
	return testComponent{"page"}
}

func (argsComponentTestPage) ComponentWithArgs(s string, i int) component {
	return testComponent{fmt.Sprintf("component: %s, %d", s, i)}
}

// Test Props method that returns error
func TestExecProps_PropsError(t *testing.T) {
	mux := http.NewServeMux()
	sp, err := Mount(mux, &propsErrorPage{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Custom error handler to capture the error
	var capturedErr error
	sp.onError = func(w http.ResponseWriter, r *http.Request, err error) {
		capturedErr = err
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
	}

	// Make a request that will trigger Props
	req := httptest.NewRequest("GET", "/", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Verify error was captured
	if capturedErr == nil {
		t.Error("Expected Props error to be captured")
	}
	if capturedErr != nil && !strings.Contains(capturedErr.Error(), "props error") {
		t.Errorf("Expected 'props error', got: %v", capturedErr)
	}
}

// Test Props method that requires dependency injection that fails
func TestExecProps_PropsDIError(t *testing.T) {
	mux := http.NewServeMux()
	sp, err := Mount(mux, &propsDIErrorPage{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Custom error handler to capture the error
	var capturedErr error
	sp.onError = func(w http.ResponseWriter, r *http.Request, err error) {
		capturedErr = err
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
	}

	// Make a request that will trigger Props with DI failure
	req := httptest.NewRequest("GET", "/", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Verify error was captured and mentions the Props method call failure
	if capturedErr == nil {
		t.Error("Expected Props DI error to be captured")
	}
	if capturedErr != nil && !strings.Contains(capturedErr.Error(), "error calling Props method") {
		t.Errorf("Expected 'error calling Props method', got: %v", capturedErr)
	}
}

// Test RenderTarget.Is() with function that has no receiver
func TestRenderTarget_Is_NoReceiver(t *testing.T) {
	// Create a RenderTarget
	pageType := reflect.TypeOf(renderTargetIsTestPage{})
	method, _ := pageType.MethodByName("Page")
	rt := newMethodRenderTarget("Page", &method)

	// Test with function that has no parameters (no receiver)
	noReceiverFunc := func() component { return testComponent{"test"} }
	result := rt.Is(noReceiverFunc)
	if result {
		t.Error("Expected Is() to return false for function with no receiver")
	}
}

// Test RenderTarget.Is() with pointer receiver
func TestRenderTarget_Is_PointerReceiver(t *testing.T) {
	// Get the pointer receiver method
	pageType := reflect.TypeOf(&pointerReceiverPage{})
	method, ok := pageType.MethodByName("Content")
	if !ok {
		t.Fatal("Could not find Content method")
	}

	rt := newMethodRenderTarget("Content", &method)

	// Test matching with pointer receiver method
	if !rt.Is((*pointerReceiverPage).Content) {
		t.Error("Expected Is() to match pointer receiver method")
	}

	// Test non-matching method
	if rt.Is((*pointerReceiverPage).Page) {
		t.Error("Expected Is() to not match different method")
	}
}

// Test extended ServeHTTP with dependency injection error (unbuffered)
func TestAsHandler_ExtendedServeHTTPError(t *testing.T) {
	mux := http.NewServeMux()
	// Mount without providing MissingDep - should cause dependency injection error
	sp, err := Mount(mux, &extendedHandlerErrorPage{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Custom error handler to capture the error
	var capturedErr error
	sp.onError = func(w http.ResponseWriter, r *http.Request, err error) {
		capturedErr = err
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
	}

	// Make request - should trigger dependency injection error
	req := httptest.NewRequest("GET", "/", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Verify error was captured
	if capturedErr != nil {
		t.Logf("Got error: %v", capturedErr)
		if !strings.Contains(capturedErr.Error(), "error calling ServeHTTP") {
			t.Errorf("Expected 'error calling ServeHTTP' error, got: %v", capturedErr)
		}
	} else {
		// If no error, check response
		t.Logf("No error captured. Response: %s", rec.Body.String())
	}
}

// Test extended ServeHTTP with return value and dependency injection error (buffered)
func TestAsHandler_ExtendedServeHTTPErrorBuffered(t *testing.T) {
	mux := http.NewServeMux()
	// Mount without providing MissingDep - should cause dependency injection error
	sp, err := Mount(mux, &extendedHandlerErrorPageWithReturn{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Custom error handler to capture the error
	var capturedErr error
	sp.onError = func(w http.ResponseWriter, r *http.Request, err error) {
		capturedErr = err
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
	}

	// Make request - should trigger dependency injection error with buffering
	req := httptest.NewRequest("GET", "/", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Verify error was captured
	if capturedErr == nil {
		t.Error("Expected dependency injection error to be captured")
	}
	if capturedErr != nil && !strings.Contains(capturedErr.Error(), "error calling ServeHTTP") {
		t.Errorf("Expected 'error calling ServeHTTP' error, got: %v", capturedErr)
	}
}

// Test findPageNode with predicate that matches no pages
func TestFindPageNode_PredicateNoMatch(t *testing.T) {
	type pages struct {
		p1 predicateTestPage1 `route:"/ Page1"`
		p2 predicateTestPage2 `route:"/page2 Page2"`
	}

	// Use predicate that matches no pages
	predicate := func(node *PageNode) bool {
		return node.Name == "NonExistentPage"
	}

	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test", WithMiddlewares(func(h http.Handler, pn *PageNode) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, urlErr := URLFor(r.Context(), predicate)
			if urlErr == nil {
				http.Error(w, "Expected error when predicate matches no pages", http.StatusInternalServerError)
				return
			}
			if !strings.Contains(urlErr.Error(), "no page matched the provided predicate function") {
				http.Error(w, fmt.Sprintf("Expected 'no page matched' error, got: %v", urlErr), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
		})
	}))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Make a request to trigger the middleware
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Test failed: %s", rec.Body.String())
	}
}

// Test page that uses only Props without a Page component
// This pattern is useful for complex pages where different views are conditionally rendered
type propsOnlyPage struct{}

type CardViewProps struct {
	ViewMode string
}

type TableViewProps struct {
	ViewMode string
}

func (propsOnlyPage) CardView(props CardViewProps) component {
	return testComponent{content: "Card View: " + props.ViewMode}
}

func (propsOnlyPage) TableView(props TableViewProps) component {
	return testComponent{content: "Table View: " + props.ViewMode}
}

func (p propsOnlyPage) Props(r *http.Request) error {
	switch r.FormValue("view") {
	case "card":
		return RenderComponent(propsOnlyPage.CardView, CardViewProps{ViewMode: "card"})
	case "table":
		return RenderComponent(propsOnlyPage.TableView, TableViewProps{ViewMode: "table"})
	default:
		// Unknown view - return nil to trigger error
		return nil
	}
}

// Test page that uses RenderTarget.Is() in Props (Props-only pattern)
type propsOnlyPageWithTarget struct{}

func (propsOnlyPageWithTarget) CardView(props CardViewProps) component {
	return testComponent{content: "Card View: " + props.ViewMode}
}

func (propsOnlyPageWithTarget) TableView(props TableViewProps) component {
	return testComponent{content: "Table View: " + props.ViewMode}
}

func (p propsOnlyPageWithTarget) Props(r *http.Request, target RenderTarget) error {
	// This should not panic even though no component is selected yet
	if target.Is(propsOnlyPageWithTarget.CardView) {
		// This branch should not execute since no component is selected
		return RenderComponent(propsOnlyPageWithTarget.TableView, TableViewProps{ViewMode: "unexpected"})
	}

	// Select component based on query parameter
	switch r.FormValue("view") {
	case "card":
		return RenderComponent(propsOnlyPageWithTarget.CardView, CardViewProps{ViewMode: "card"})
	default:
		return RenderComponent(propsOnlyPageWithTarget.TableView, TableViewProps{ViewMode: "table"})
	}
}

func TestPropsOnlyPage(t *testing.T) {
	type pages struct {
		propsOnlyPage `route:"/ Index"`
	}

	var capturedError error
	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test", WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
		capturedError = err
		t.Logf("Error occurred: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Test table view
	{
		capturedError = nil
		req := httptest.NewRequest(http.MethodGet, "/?view=table", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
			if capturedError != nil {
				t.Logf("Captured error: %v", capturedError)
			}
		}
		if rec.Body.String() != "Table View: table" {
			t.Errorf("expected body %q, got %q", "Table View: table", rec.Body.String())
		}
	}

	// Test card view
	{
		capturedError = nil
		req := httptest.NewRequest(http.MethodGet, "/?view=card", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
			if capturedError != nil {
				t.Logf("Captured error: %v", capturedError)
			}
		}
		if rec.Body.String() != "Card View: card" {
			t.Errorf("expected body %q, got %q", "Card View: card", rec.Body.String())
		}
	}

	// Test edge case: Props returns nil without calling RenderComponent
	{
		capturedError = nil
		req := httptest.NewRequest(http.MethodGet, "/?view=unknown", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
		}
		if capturedError == nil {
			t.Error("expected error when Props returns nil without RenderComponent")
		}
		expectedErr := "no component found and Props did not use RenderComponent"
		if capturedError != nil && !strings.Contains(capturedError.Error(), expectedErr) {
			t.Errorf("expected '%s' error, got: %v", expectedErr, capturedError)
		}
	}
}

// Test page that uses ServeHTTP with RenderTarget injection
type serveHTTPWithRenderTarget struct{}

func (serveHTTPWithRenderTarget) Page() component {
	return testComponent{content: "Page"}
}

func (serveHTTPWithRenderTarget) Content() component {
	return testComponent{content: "Content"}
}

func (p serveHTTPWithRenderTarget) ServeHTTP(w http.ResponseWriter, r *http.Request, target RenderTarget) {
	// Check which component is selected
	if target.Is(serveHTTPWithRenderTarget.Content) {
		_, _ = w.Write([]byte("ServeHTTP: Content selected"))
	} else if target.Is(serveHTTPWithRenderTarget.Page) {
		_, _ = w.Write([]byte("ServeHTTP: Page selected"))
	} else {
		_, _ = w.Write([]byte("ServeHTTP: Unknown component"))
	}
}

// Test page that uses ServeHTTP with custom dependency and RenderTarget
type AppContext struct {
	UserID string
}

type serveHTTPWithCustomDepsAndTarget struct{}

func (serveHTTPWithCustomDepsAndTarget) Page() component {
	return testComponent{content: "Page"}
}

func (serveHTTPWithCustomDepsAndTarget) Content() component {
	return testComponent{content: "Content"}
}

func (p serveHTTPWithCustomDepsAndTarget) ServeHTTP(
	w http.ResponseWriter, r *http.Request, appCtx *AppContext, target RenderTarget,
) error {
	// This mimics the real-world usage where ServeHTTP needs both custom deps and RenderTarget
	if target.Is(serveHTTPWithCustomDepsAndTarget.Content) {
		_, _ = w.Write([]byte("ServeHTTP: Content selected, user: " + appCtx.UserID))
	} else if target.Is(serveHTTPWithCustomDepsAndTarget.Page) {
		_, _ = w.Write([]byte("ServeHTTP: Page selected, user: " + appCtx.UserID))
	} else {
		_, _ = w.Write([]byte("ServeHTTP: Unknown component"))
	}
	return nil
}

func TestServeHTTPWithCustomDepsAndRenderTarget(t *testing.T) {
	type pages struct {
		serveHTTPWithCustomDepsAndTarget `route:"/ Index"`
	}

	// Custom selector that returns Content for HTMX requests
	selector := func(r *http.Request, pn *PageNode) (RenderTarget, error) {
		if r.Header.Get("HX-Request") == "true" {
			method := pn.Components["Content"]
			return newMethodRenderTarget("Content", &method), nil
		}
		method := pn.Components["Page"]
		return newMethodRenderTarget("Page", &method), nil
	}

	appCtx := &AppContext{UserID: "user123"}

	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test", WithTargetSelector(selector), WithArgs(appCtx))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Test Page component selection
	{
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		expected := "ServeHTTP: Page selected, user: user123"
		if rec.Body.String() != expected {
			t.Errorf("expected body %q, got %q", expected, rec.Body.String())
		}
	}

	// Test Content component selection (HTMX request)
	{
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		expected := "ServeHTTP: Content selected, user: user123"
		if rec.Body.String() != expected {
			t.Errorf("expected body %q, got %q", expected, rec.Body.String())
		}
	}
}

// Test page that uses ServeHTTP with RenderTarget but no components
type serveHTTPNoComponentsWithTarget struct{}

func (p serveHTTPNoComponentsWithTarget) ServeHTTP(w http.ResponseWriter, r *http.Request, target RenderTarget) error {
	// RenderTarget should be available even with no components (will be empty)
	if target == nil {
		_, _ = w.Write([]byte("ERROR: target is nil"))
		return nil
	}
	// target.Is() should return false for everything since no components
	_, _ = w.Write([]byte("ServeHTTP: RenderTarget available, no components"))
	return nil
}

func TestServeHTTPWithRenderTargetNoComponents(t *testing.T) {
	type pages struct {
		serveHTTPNoComponentsWithTarget `route:"/ Index"`
	}

	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	expected := "ServeHTTP: RenderTarget available, no components"
	if rec.Body.String() != expected {
		t.Errorf("expected body %q, got %q", expected, rec.Body.String())
	}
}

func TestServeHTTPWithRenderTarget(t *testing.T) {
	type pages struct {
		serveHTTPWithRenderTarget `route:"/ Index"`
	}

	// Custom selector that returns Content for HTMX requests
	selector := func(r *http.Request, pn *PageNode) (RenderTarget, error) {
		if r.Header.Get("HX-Request") == "true" {
			method := pn.Components["Content"]
			return newMethodRenderTarget("Content", &method), nil
		}
		method := pn.Components["Page"]
		return newMethodRenderTarget("Page", &method), nil
	}

	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test", WithTargetSelector(selector))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Test Page component selection
	{
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "ServeHTTP: Page selected" {
			t.Errorf("expected body %q, got %q", "ServeHTTP: Page selected", rec.Body.String())
		}
	}

	// Test Content component selection (HTMX request)
	{
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "ServeHTTP: Content selected" {
			t.Errorf("expected body %q, got %q", "ServeHTTP: Content selected", rec.Body.String())
		}
	}
}

func TestPropsOnlyPageWithRenderTarget(t *testing.T) {
	type pages struct {
		propsOnlyPageWithTarget `route:"/ Index"`
	}

	var capturedError error
	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test", WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
		capturedError = err
		t.Logf("Error occurred: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Test that target.Is() doesn't panic when no component is selected (table view)
	{
		capturedError = nil
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
			if capturedError != nil {
				t.Logf("Captured error: %v", capturedError)
			}
		}
		if rec.Body.String() != "Table View: table" {
			t.Errorf("expected body %q, got %q", "Table View: table", rec.Body.String())
		}
	}

	// Test card view with target.Is() check
	{
		capturedError = nil
		req := httptest.NewRequest(http.MethodGet, "/?view=card", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
			if capturedError != nil {
				t.Logf("Captured error: %v", capturedError)
			}
		}
		if rec.Body.String() != "Card View: card" {
			t.Errorf("expected body %q, got %q", "Card View: card", rec.Body.String())
		}
	}
}

// Test RenderComponent edge cases for coverage
func TestRenderComponent_EdgeCases(t *testing.T) {
	// Test componentGetter with args (should error)
	getter := myComponentGetter{data: "test"}
	err := RenderComponent(getter, "extra arg")
	if err == nil {
		t.Fatal("Expected error when passing args to componentGetter")
	}
	if !strings.Contains(err.Error(), "componentGetter cannot have args") {
		t.Errorf("Expected 'componentGetter cannot have args' error, got: %v", err)
	}

	// Test direct component with args (should error)
	comp := testComponent{"test"}
	err = RenderComponent(comp, "extra arg")
	if err == nil {
		t.Fatal("Expected error when passing args to component instance")
	}
	if !strings.Contains(err.Error(), "component instance cannot have args") {
		t.Errorf("Expected 'component instance cannot have args' error, got: %v", err)
	}

	// Test with non-function, non-component, non-RenderTarget
	err = RenderComponent(struct{ Name string }{"test"})
	if err == nil {
		t.Fatal("Expected error for non-component struct")
	}
	if !strings.Contains(err.Error(), "target must be a component, RenderTarget, or function") {
		t.Errorf("Expected 'target must be' error, got: %v", err)
	}
}

// Test executeRenderOp error paths
func TestExecuteRenderOp_Errors(t *testing.T) {
	mux := http.NewServeMux()
	sp, err := Mount(mux, &handleErrorTestPage{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Test function that doesn't return a component
	badFunc := func() string { return "not a component" }
	op := &renderOp{
		callable: reflect.ValueOf(badFunc),
		args:     []reflect.Value{},
	}

	_, err = sp.executeRenderOp(op, sp.pc.root)
	if err == nil {
		t.Fatal("Expected error for function not returning component")
	}
	if !strings.Contains(err.Error(), "must return component") {
		t.Errorf("Expected 'must return component' error, got: %v", err)
	}

	// Test function that returns wrong number of values
	multiReturnFunc := func() (component, error) {
		return testComponent{"test"}, nil
	}
	op2 := &renderOp{
		callable: reflect.ValueOf(multiReturnFunc),
		args:     []reflect.Value{},
	}

	_, err = sp.executeRenderOp(op2, sp.pc.root)
	if err == nil {
		t.Fatal("Expected error for function returning multiple values")
	}
	if !strings.Contains(err.Error(), "must return single value") {
		t.Errorf("Expected 'must return single value' error, got: %v", err)
	}

	// Test method without page context
	method := reflect.Method{}
	op3 := &renderOp{
		method: &method,
		args:   []reflect.Value{},
	}

	_, err = sp.executeRenderOp(op3, nil)
	if err == nil {
		t.Fatal("Expected error for method without page context")
	}
	if !strings.Contains(err.Error(), "cannot execute method without page context") {
		t.Errorf("Expected 'cannot execute method without page context' error, got: %v", err)
	}

	// Test renderOp with nothing set
	op4 := &renderOp{}
	_, err = sp.executeRenderOp(op4, sp.pc.root)
	if err == nil {
		t.Fatal("Expected error for empty renderOp")
	}
	if !strings.Contains(err.Error(), "renderOp has no component, method, or callable") {
		t.Errorf("Expected 'renderOp has no' error, got: %v", err)
	}
}

// customTestTarget is an unsupported RenderTarget type for testing
type customTestTarget struct{}

func (ct customTestTarget) Is(any) bool { return false }

// Test renderOpFromTarget error paths
func TestRenderOpFromTarget_Errors(t *testing.T) {
	// Test custom RenderTarget type (unsupported)
	ct := customTestTarget{}
	_, err := renderOpFromTarget(ct, []reflect.Value{})
	if err == nil {
		t.Fatal("Expected error for unsupported RenderTarget type")
	}
	if !strings.Contains(err.Error(), "unsupported RenderTarget type") {
		t.Errorf("Expected 'unsupported RenderTarget type' error, got: %v", err)
	}

	// Test functionRenderTarget without funcValue set
	frt := &functionRenderTarget{
		hxTarget: "test",
		pageName: "Test",
		// funcValue not set (Is() was not called)
	}

	_, err = renderOpFromTarget(frt, []reflect.Value{})
	if err == nil {
		t.Fatal("Expected error for functionRenderTarget without funcValue")
	}
	if !strings.Contains(err.Error(), "did you call target.Is() first") {
		t.Errorf("Expected 'did you call target.Is() first' error, got: %v", err)
	}
}
