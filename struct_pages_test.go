//lint:file-ignore U1000 Ignore unused code in test file

package structpages

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
	r := NewRouter(mux)
	sp := New()
	if err := sp.MountPages(r, &topPage{}, "/", ""); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/struct", http.NoBody)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
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
		r.ServeHTTP(rec, req)
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
	r := NewRouter(http.NewServeMux())
	sp := New()
	if err := sp.MountPages(r, &topPage{}, "/", "top page"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}
	{
		req := httptest.NewRequest(http.MethodGet, "/middleware", http.NoBody)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
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
		r.ServeHTTP(rec, req)
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

// type ConfiTestPage struct{}
//
// func (ConfiTestPage) PageConfig(r *http.Request) (string, error) {
// 	return "DefaultConfigPage", nil
// }

func TestPageConfig(t *testing.T) {
	sp := New()
	r := NewRouter(http.NewServeMux())
	type topPage struct {
		DefaultConfigPage `route:"/default Default config page"`
	}
	if err := sp.MountPages(r, &topPage{}, "/", "top page"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}
	{
		req := httptest.NewRequest(http.MethodGet, "/default", http.NoBody)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "Default config page" {
			t.Errorf("expected body %q, got %q", "Default config page", rec.Body.String())
		}
	}
}

func TestHTMXPageConfig(t *testing.T) {
	sp := New(WithDefaultPageConfig(HTMXPageConfig))
	r := NewRouter(http.NewServeMux())
	type topPage struct {
		DefaultConfigPage `route:"/default Default config page"`
	}
	if err := sp.MountPages(r, &topPage{}, "/", "top page"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/default", http.NoBody)
	req.Header.Set("Hx-Request", "true")
	req.Header.Set("Hx-Target", "hx-target")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	expectedBody := "hx target defaultConfigPage"
	if rec.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, rec.Body.String())
	}
}

type CustomConfigPage struct{}

func (CustomConfigPage) Custom() component {
	return testComponent{content: "Custom config page"}
}

func (CustomConfigPage) PageConfig(r *http.Request) (string, error) {
	return "Custom", nil
}

func TestCustomPageConfig(t *testing.T) {
	sp := New()
	r := NewRouter(http.NewServeMux())
	type topPage struct {
		CustomConfigPage `route:"/custom Custom config page"`
	}
	if err := sp.MountPages(r, &topPage{}, "/", "top page"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/custom", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	expectedBody := "Custom config page"
	if rec.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, rec.Body.String())
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
	sp := New()
	r := NewRouter(http.NewServeMux())
	type topPage struct {
		skipRenderPage `route:"/skip Test skip render"`
	}
	if err := sp.MountPages(r, &topPage{}, "/", "top page"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}

	// Test normal rendering (no skip)
	{
		req := httptest.NewRequest(http.MethodGet, "/skip", http.NoBody)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
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
		r.ServeHTTP(rec, req)
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
	sp := New(
		WithMiddlewares(
			makeMiddleware("global mw 1"),
			makeMiddleware("global mw 2"),
			makeMiddleware("global mw 3"),
		),
	)
	r := NewRouter(http.NewServeMux())
	type topPage struct {
		middlewareOrderPage `route:"/"`
	}
	if err := sp.MountPages(r, &topPage{}, "/", "top page"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

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

type testPropsPage struct{}

func (testPropsPage) Page(s string) component             { return testComponent{content: s} }
func (testPropsPage) Content(s string) component          { return testComponent{content: s} }
func (testPropsPage) Another(s string) component          { return testComponent{content: s} }
func (testPropsPage) Props() (string, error)              { return "Default Props", nil }
func (testPropsPage) PageProps(r *http.Request) string    { return "Page Props" }
func (testPropsPage) ContentProps(r *http.Request) string { return "Content Props" }

func TestProps(t *testing.T) {
	sp := New(WithDefaultPageConfig(HTMXPageConfig))
	r := NewRouter(http.NewServeMux())
	type topPage struct {
		testPropsPage `route:"/props Test Props Page"`
	}
	if err := sp.MountPages(r, &topPage{}, "/", "top page"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}

	tests := []struct {
		name         string
		hxTarget     string
		expectedBody string
	}{
		{
			name:         "Page Props",
			expectedBody: "Page Props",
		},
		{
			name:         "Content Props",
			hxTarget:     "content",
			expectedBody: "Content Props",
		},
		{
			name:         "Another Props fallback to Props",
			hxTarget:     "another",
			expectedBody: "Default Props",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/props", http.NoBody)
			if tt.hxTarget != "" {
				req.Header.Set("Hx-Request", "true")
				req.Header.Set("Hx-Target", tt.hxTarget)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
			}
			if diff := cmp.Diff(tt.expectedBody, rec.Body.String()); diff != "" {
				t.Errorf("unexpected body (-want +got):\n%s", diff)
			}
		})
	}
}

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
				typ := reflect.TypeOf(&testPropsPage{})
				method, _ := typ.MethodByName("Page")
				return &method
			},
			expected: "structpages.testPropsPage.Page",
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

	sp := New(WithErrorHandler(errorHandler))
	r := NewRouter(http.NewServeMux())
	// Pass the typed arguments that the extended handlers expect
	if err := sp.MountPages(r, &pages{}, "/", "Test Extended",
		ExtendedHandlerArg("extra value"),
		ExtendedErrHandlerArg("error extra")); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}

	// Test extended handler
	{
		req := httptest.NewRequest(http.MethodGet, "/extended", http.NoBody)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
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
		r.ServeHTTP(rec, req)
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

	sp := New(WithErrorHandler(customErrorHandler))
	pc := &parseContext{}
	pc.root = &PageNode{}

	pn := &PageNode{
		Name:  "ErrHandler",
		Value: reflect.ValueOf(errHandler{}),
	}

	handler := sp.asHandler(pc, pn)

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
			sp := New()
			pc := tt.setupContext()
			handler := sp.asHandler(pc, tt.pageNode)

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
	sp := New()
	router := NewRouter(nil)

	// Create a page with manually constructed PageNode that has empty route
	// This simulates what would happen if parsing produced a page with empty route
	pc := &parseContext{}
	pageNode := &PageNode{
		Name:  "testPage",
		Route: "", // This will trigger the "page item route is empty" error
	}

	err := sp.registerPageItem(router, pc, pageNode, nil)
	if err == nil {
		t.Error("Expected error from registerPageItem with empty route")
	}
	if err != nil && err.Error() != "page item route is empty: testPage" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

// Test MountPages error cases
func TestStructPages_MountPages_parseError(t *testing.T) {
	sp := New()
	router := NewRouter(http.NewServeMux())

	// This should cause a parse error due to duplicate args
	err := sp.MountPages(router, struct{}{}, "/", "Test", "arg1", "arg1")
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
	sp := New()
	pc := &parseContext{args: make(argRegistry)}

	propsMethod, _ := reflect.TypeOf(&pageWithErrorProps{}).MethodByName("Props")
	pn := &PageNode{
		Name: "test",
		Props: map[string]reflect.Method{
			"Props": propsMethod,
		},
		Value: reflect.ValueOf(&pageWithErrorProps{}),
	}

	req := httptest.NewRequest("GET", "/", http.NoBody)

	_, err := sp.execProps(pc, pn, req, nil)
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

func (renderComponentPage) Props(r *http.Request) (string, error) {
	view := r.URL.Query().Get("view")
	switch view {
	case "partial":
		// Use RenderComponent with custom args
		return "ignored", RenderComponent("PartialView", "custom partial data")
	case "custom":
		// Use RenderComponent with multiple args
		return "ignored", RenderComponent("CustomView", "test", 42)
	case "alt":
		// Use RenderComponent without args - should use Props return value
		return "props data", RenderComponent("AltView")
	default:
		// Normal flow
		return "default data", nil
	}
}

// Test RenderComponent functionality
func TestRenderComponent(t *testing.T) {
	sp := New()
	r := NewRouter(http.NewServeMux())
	type pages struct {
		renderComponentPage `route:"/render Test Render Component"`
	}
	if err := sp.MountPages(r, &pages{}, "/", "Test"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
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
			req := httptest.NewRequest(http.MethodGet, "/render"+tt.query, http.NoBody)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
			}
			if rec.Body.String() != tt.expectedBody {
				t.Errorf("expected body %q, got %q", tt.expectedBody, rec.Body.String())
			}
		})
	}
}

// Test RenderComponent with non-existent component
type renderComponentErrorPage struct{}

func (renderComponentErrorPage) Page() component {
	return testComponent{content: "Page"}
}

func (renderComponentErrorPage) Props(r *http.Request) error {
	return RenderComponent("NonExistent")
}

func TestRenderComponent_NonExistentComponent(t *testing.T) {
	sp := New()
	r := NewRouter(http.NewServeMux())

	var capturedError error
	sp.onError = func(w http.ResponseWriter, r *http.Request, err error) {
		capturedError = err
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	type pages struct {
		renderComponentErrorPage `route:"/error Test Error"`
	}
	if err := sp.MountPages(r, &pages{}, "/", "Test"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/error", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	if capturedError == nil {
		t.Error("expected error to be captured")
	} else if !strings.Contains(capturedError.Error(), "NonExistent") {
		t.Errorf("expected error to mention NonExistent, got: %v", capturedError)
	}
}

// Test RenderComponent overrides PageConfig
type renderComponentWithConfigPage struct{}

func (renderComponentWithConfigPage) Page() component {
	return testComponent{content: "Page"}
}

func (renderComponentWithConfigPage) Alt() component {
	return testComponent{content: "Alt from PageConfig"}
}

func (renderComponentWithConfigPage) Override() component {
	return testComponent{content: "Override from RenderComponent"}
}

func (renderComponentWithConfigPage) PageConfig(r *http.Request) string {
	// This would normally return "Alt"
	return "Alt"
}

func (renderComponentWithConfigPage) Props(r *http.Request) error {
	if r.URL.Query().Get("override") == "true" {
		// RenderComponent should override PageConfig
		return RenderComponent("Override")
	}
	return nil
}

func TestRenderComponent_OverridesPageConfig(t *testing.T) {
	sp := New()
	r := NewRouter(http.NewServeMux())

	type pages struct {
		renderComponentWithConfigPage `route:"/config Test Config Override"`
	}
	if err := sp.MountPages(r, &pages{}, "/", "Test"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}

	tests := []struct {
		name         string
		query        string
		expectedBody string
		description  string
	}{
		{
			name:         "PageConfig determines component",
			query:        "",
			expectedBody: "Alt from PageConfig",
			description:  "Without override, PageConfig should select Alt component",
		},
		{
			name:         "RenderComponent overrides PageConfig",
			query:        "?override=true",
			expectedBody: "Override from RenderComponent",
			description:  "RenderComponent should override PageConfig's selection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/config"+tt.query, http.NoBody)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
			}
			if rec.Body.String() != tt.expectedBody {
				t.Errorf("%s: expected body %q, got %q", tt.description, tt.expectedBody, rec.Body.String())
			}
		})
	}
}
