package structpages

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Test page with error component
type renderTestErrorPage struct{}

func (e renderTestErrorPage) ErrorComponent(message string) component {
	return renderTestErrorComponent{message: message}
}

func (e renderTestErrorPage) NotFoundComponent(ignored string) component {
	return renderTestNotFoundComponent{}
}

type renderTestErrorComponent struct {
	message string
}

func (e renderTestErrorComponent) Render(ctx context.Context, w io.Writer) error {
	_, err := w.Write([]byte("<div class=\"error\">" + e.message + "</div>"))
	return err
}

type renderTestNotFoundComponent struct{}

func (n renderTestNotFoundComponent) Render(ctx context.Context, w io.Writer) error {
	_, err := w.Write([]byte("<div class=\"not-found\">Page not found</div>"))
	return err
}

// Test page that conditionally renders error components
type renderTestConditionalPage struct{}

func (c renderTestConditionalPage) Props(r *http.Request) (string, error) {
	trigger := r.URL.Query().Get("trigger")
	switch trigger {
	case "error":
		return "", RenderComponent((*renderTestErrorPage).ErrorComponent, "Something went wrong")
	case "notfound":
		return "", RenderComponent((*renderTestErrorPage).NotFoundComponent, "ignored")
	default:
		return "success", nil
	}
}

func (c renderTestConditionalPage) Page(message string) component {
	return renderTestSuccessComponent{message: message}
}

type renderTestSuccessComponent struct {
	message string
}

func (s renderTestSuccessComponent) Render(ctx context.Context, w io.Writer) error {
	_, err := w.Write([]byte("<div class=\"success\">" + s.message + "</div>"))
	return err
}

// Test RenderPageComponent normal rendering
func TestRenderPageComponent_Normal(t *testing.T) {
	type pages struct {
		renderTestConditionalPage `route:"/test"`
		renderTestErrorPage       `route:"/error"`
	}

	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	expected := "<div class=\"success\">success</div>"
	if body != expected {
		t.Errorf("expected body %q, got %q", expected, body)
	}
}

// Test RenderPageComponent with error component
func TestRenderPageComponent_Error(t *testing.T) {
	type pages struct {
		renderTestConditionalPage `route:"/test"`
		renderTestErrorPage       `route:"/error"`
	}

	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test?trigger=error", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	expected := "<div class=\"error\">Something went wrong</div>"
	if body != expected {
		t.Errorf("expected body %q, got %q", expected, body)
	}
}

// Test RenderPageComponent with not found component
func TestRenderPageComponent_NotFound(t *testing.T) {
	type pages struct {
		renderTestConditionalPage `route:"/test"`
		renderTestErrorPage       `route:"/error"`
	}

	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test?trigger=notfound", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	expected := "<div class=\"not-found\">Page not found</div>"
	if body != expected {
		t.Errorf("expected body %q, got %q", expected, body)
	}
}

// Test multiple arguments
type multiArgPage struct{}

func (m multiArgPage) MultiComponent(name string, count int, enabled bool) component {
	return multiArgComponent{name: name, count: count, enabled: enabled}
}

type multiArgComponent struct {
	name    string
	count   int
	enabled bool
}

func (m multiArgComponent) Render(ctx context.Context, w io.Writer) error {
	status := "disabled"
	if m.enabled {
		status = "enabled"
	}
	content := "<div>" + m.name + " count:" + string(rune(m.count+'0')) + " status:" + status + "</div>"
	_, err := w.Write([]byte(content))
	return err
}

type multiArgTestPage struct{}

func (m multiArgTestPage) Props(r *http.Request) (string, error) {
	return "", RenderComponent((*multiArgPage).MultiComponent, "test", 5, true)
}

func (m multiArgTestPage) Page(message string) component {
	return renderTestSuccessComponent{message: message}
}

// Test RenderPageComponent with multiple arguments
func TestRenderPageComponentMultipleArgs(t *testing.T) {
	type pages struct {
		multiArgTestPage `route:"/multitest"`
		multiArgPage     `route:"/multi"`
	}

	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/multitest", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	expectedBody := "<div>test count:5 status:enabled</div>"
	body := rec.Body.String()
	if body != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, body)
	}
}

// Test error handling when page not found
type invalidPageTestPage struct{}

// Helper type that has methods but won't be registered
type unregisteredPage struct{}

func (u unregisteredPage) SomeComponent() component {
	return renderTestSuccessComponent{message: "unregistered"}
}

func (i invalidPageTestPage) Props(r *http.Request) (string, error) {
	// Reference a page that doesn't exist in the router
	return "", RenderComponent((*unregisteredPage).SomeComponent)
}

func (i invalidPageTestPage) Page(message string) component {
	return renderTestSuccessComponent{message: message}
}

// Test RenderPageComponent with invalid page
func TestRenderPageComponentInvalidPage(t *testing.T) {
	type pages struct {
		invalidPageTestPage `route:"/invalid"`
	}

	var capturedError error
	errorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		capturedError = err
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test", WithErrorHandler(errorHandler))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/invalid", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	if capturedError == nil {
		t.Error("expected error to be captured")
	} else {
		expectedErrorSubstring := "no page node found"
		if !strings.Contains(capturedError.Error(), expectedErrorSubstring) {
			t.Errorf("expected error to contain %q, got %q", expectedErrorSubstring, capturedError.Error())
		}
	}
}

// Note: Test for "component not found" removed - with method expressions,
// referencing a non-existent component is now a compile-time error, not a runtime error.
// This is intentional and provides better type safety.

// Test RenderPageComponent with different component methods
type componentVariationsPage struct{}

func (c componentVariationsPage) Props(r *http.Request) (string, error) {
	component := r.URL.Query().Get("component")
	switch component {
	case "header":
		return "", RenderComponent((*headerPage).Page, "ignored")
	case "footer":
		return "", RenderComponent((*footerPage).Page, "© 2024")
	default:
		return "default", nil
	}
}

func (c componentVariationsPage) Page(message string) component {
	return renderTestSuccessComponent{message: message}
}

type headerPage struct{}

func (h headerPage) Page(ignored string) component {
	return headerComponent{}
}

type footerPage struct{}

func (f footerPage) Page(text string) component {
	return footerComponent{text: text}
}

type headerComponent struct{}

func (h headerComponent) Render(ctx context.Context, w io.Writer) error {
	_, err := w.Write([]byte("<header>Site Header</header>"))
	return err
}

type footerComponent struct {
	text string
}

func (f footerComponent) Render(ctx context.Context, w io.Writer) error {
	_, err := w.Write([]byte("<footer>" + f.text + "</footer>"))
	return err
}

// Test RenderPageComponent with header component
func TestRenderPageComponentSharedComponents_Header(t *testing.T) {
	type pages struct {
		componentVariationsPage `route:"/shared"`
		headerPage              `route:"/header"`
		footerPage              `route:"/footer"`
	}

	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/shared?component=header", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	expected := "<header>Site Header</header>"
	if body != expected {
		t.Errorf("expected body %q, got %q", expected, body)
	}
}

// Test RenderPageComponent with footer component
func TestRenderPageComponentSharedComponents_Footer(t *testing.T) {
	type pages struct {
		componentVariationsPage `route:"/shared"`
		headerPage              `route:"/header"`
		footerPage              `route:"/footer"`
	}

	var capturedError error
	errorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		capturedError = err
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test", WithErrorHandler(errorHandler))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/shared?component=footer", nil)
	rec := httptest.NewRecorder()
	capturedError = nil
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		if capturedError != nil {
			t.Errorf("captured error: %v", capturedError)
		}
	}

	body := rec.Body.String()
	expected := "<footer>© 2024</footer>"
	if body != expected {
		t.Errorf("expected body %q, got %q", expected, body)
		if capturedError != nil {
			t.Errorf("captured error: %v", capturedError)
		}
	}
}

// Test RenderPageComponent from httpErrHandler.ServeHTTP
type errHandlerWithRenderComponent struct{}

func (e errHandlerWithRenderComponent) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	trigger := r.URL.Query().Get("trigger")
	switch trigger {
	case "error":
		return RenderComponent((*renderTestErrorPage).ErrorComponent, "Error from handler")
	case "notfound":
		return RenderComponent((*renderTestErrorPage).NotFoundComponent, "ignored")
	default:
		_, _ = w.Write([]byte("<div>normal response</div>"))
		return nil
	}
}

func TestRenderPageComponent_FromErrHandler(t *testing.T) {
	type pages struct {
		errHandlerWithRenderComponent `route:"/handler"`
		renderTestErrorPage           `route:"/error"`
	}

	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "normal response",
			query:    "",
			expected: "<div>normal response</div>",
		},
		{
			name:     "render error component",
			query:    "?trigger=error",
			expected: `<div class="error">Error from handler</div>`,
		},
		{
			name:     "render not found component",
			query:    "?trigger=notfound",
			expected: `<div class="not-found">Page not found</div>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/handler"+tt.query, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
			}

			body := rec.Body.String()
			if body != tt.expected {
				t.Errorf("expected body %q, got %q", tt.expected, body)
			}
		})
	}
}

// Test RenderPageComponent from extended ServeHTTP (with extra args and error return)
type extendedHandlerWithRenderComponent struct{}

func (e extendedHandlerWithRenderComponent) ServeHTTP(w http.ResponseWriter, r *http.Request, logger string) error {
	trigger := r.URL.Query().Get("trigger")
	switch trigger {
	case "error":
		return RenderComponent((*renderTestErrorPage).ErrorComponent, "Error with logging: "+logger)
	case "multi":
		return RenderComponent((*multiArgPage).MultiComponent, "extended", 3, false)
	default:
		_, _ = w.Write([]byte("<div>extended handler: " + logger + "</div>"))
		return nil
	}
}

func TestRenderPageComponent_FromExtendedServeHTTP(t *testing.T) {
	type pages struct {
		extendedHandlerWithRenderComponent `route:"/extended"`
		renderTestErrorPage                `route:"/error"`
		multiArgPage                       `route:"/multi"`
	}

	// Provide logger arg for dependency injection
	logger := "test-logger"
	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test", WithArgs(logger))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "normal response",
			query:    "",
			expected: "<div>extended handler: test-logger</div>",
		},
		{
			name:     "render error component",
			query:    "?trigger=error",
			expected: `<div class="error">Error with logging: test-logger</div>`,
		},
		{
			name:     "render multi-arg component",
			query:    "?trigger=multi",
			expected: "<div>extended count:3 status:disabled</div>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/extended"+tt.query, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
			}

			body := rec.Body.String()
			if body != tt.expected {
				t.Errorf("expected body %q, got %q", tt.expected, body)
			}
		})
	}
}

// Test RenderComponent (same-page component) from error handlers
type errHandlerWithRenderSamePageComponent struct{}

func (e errHandlerWithRenderSamePageComponent) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	if r.URL.Query().Get("alt") == "true" {
		return RenderComponent((*errHandlerWithRenderSamePageComponent).AltView, "alternative view")
	}
	_, _ = w.Write([]byte("<div>default view</div>"))
	return nil
}

func (e errHandlerWithRenderSamePageComponent) AltView(message string) component {
	return renderTestSuccessComponent{message: message}
}

func TestRenderComponent_FromErrHandler(t *testing.T) {
	type pages struct {
		errHandlerWithRenderSamePageComponent `route:"/samecomp"`
	}

	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "default view",
			query:    "",
			expected: "<div>default view</div>",
		},
		{
			name:     "alternative view via RenderComponent",
			query:    "?alt=true",
			expected: `<div class="success">alternative view</div>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/samecomp"+tt.query, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
			}

			body := rec.Body.String()
			if body != tt.expected {
				t.Errorf("expected body %q, got %q", tt.expected, body)
			}
		})
	}
}

// Test renderOpFromTarget with invalid funcValue
func TestRenderOpFromTarget_InvalidFuncValue(t *testing.T) {
	// Create a functionRenderTarget with no funcValue set
	frt := &functionRenderTarget{
		hxTarget: "test-target",
		pageName: "TestPage",
		// funcValue is left as zero value (invalid)
	}

	// Try to create renderOp from this target - should error
	_, err := renderOpFromTarget(frt, nil)
	if err == nil {
		t.Error("Expected error when functionRenderTarget has invalid funcValue")
	}
	if !strings.Contains(err.Error(), "has no funcValue") {
		t.Errorf("Expected 'has no funcValue' error, got: %v", err)
	}
}

// Custom RenderTarget type for testing unsupported type error
type unsupportedRenderTarget struct{}

func (unsupportedRenderTarget) Is(any) bool { return false }

// Test renderOpFromTarget with unsupported RenderTarget type
func TestRenderOpFromTarget_UnsupportedType(t *testing.T) {
	// Create an unsupported RenderTarget type
	urt := unsupportedRenderTarget{}

	// Try to create renderOp from this target - should error
	_, err := renderOpFromTarget(urt, nil)
	if err == nil {
		t.Error("Expected error when renderOpFromTarget receives unsupported type")
	}
	if !strings.Contains(err.Error(), "unsupported RenderTarget type") {
		t.Errorf("Expected 'unsupported RenderTarget type' error, got: %v", err)
	}
}

// Test resolveRenderOp with direct component instance with args (error)
func TestResolveRenderOp_ComponentWithArgs(t *testing.T) {
	comp := testComponent{content: "test"}
	err := RenderComponent(comp, "extra arg")
	if err == nil {
		t.Error("Expected error when RenderComponent called with component and args")
	}
	if !strings.Contains(err.Error(), "component instance cannot have args") {
		t.Errorf("Expected 'cannot have args' error, got: %v", err)
	}
}

// Test resolveRenderOp with non-function, non-component target
func TestResolveRenderOp_InvalidTarget(t *testing.T) {
	// Try to render a string (not a valid target type)
	err := RenderComponent("not valid")
	if err == nil {
		t.Error("Expected error when RenderComponent called with invalid target")
	}
	if !strings.Contains(err.Error(), "must be a component, RenderTarget, or function") {
		t.Errorf("Expected 'must be a component, RenderTarget, or function' error, got: %v", err)
	}
}

// Test page that uses RenderComponent with direct component
type directComponentPage struct{}

func (directComponentPage) Page(data string) component {
	return testComponent{content: "Page: " + data}
}

func (p directComponentPage) Props(r *http.Request, target RenderTarget) (string, error) {
	// Return a direct component instance
	comp := testComponent{content: "Direct component"}
	return "", RenderComponent(comp)
}

// Test RenderComponent with direct component instance (no args)
func TestRenderComponent_DirectComponent(t *testing.T) {
	type pages struct {
		directComponentPage `route:"/ DirectPage"`
	}

	mux := http.NewServeMux()
	sp, err := Mount(mux, &pages{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	_ = sp

	// Make HTMX request with any target (Props will return direct component)
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", "some-target")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if body != "Direct component" {
		t.Errorf("Expected 'Direct component', got %q", body)
	}
}

// Test page with a method that's NOT a component
type pageWithNonComponentMethod struct{}

func (pageWithNonComponentMethod) Page(data string) component {
	return testComponent{content: "Page: " + data}
}

// This is NOT a component method (doesn't return component)
func (pageWithNonComponentMethod) NonComponentMethod() string {
	return "not a component"
}

// Test page that tries to render non-component method from another page
type crossPageErrorPage struct{}

func (crossPageErrorPage) Page(data string) component {
	return testComponent{content: "CrossPage: " + data}
}

func (p crossPageErrorPage) Props(r *http.Request, target RenderTarget) (string, error) {
	// Try to render a method from pageWithNonComponentMethod
	// This method exists on the type but isn't registered as a component
	return "", RenderComponent(pageWithNonComponentMethod.NonComponentMethod)
}

// Test error when method not found in Components map
func TestHandleRenderComponentError_MethodNotInComponents(t *testing.T) {
	type pages struct {
		crossPageErrorPage         `route:"/ CrossPageError"`
		pageWithNonComponentMethod `route:"/other Other"`
	}

	mux := http.NewServeMux()
	var capturedErr error
	sp, err := Mount(mux, &pages{}, "/", "Test", WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
		capturedErr = err
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
	}))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	_ = sp

	// Make HTMX request
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", "some-target")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Should get error because NonComponentMethod is not in Components map
	if capturedErr == nil {
		t.Error("Expected error when method not in Components map")
	}
	if capturedErr != nil && !strings.Contains(capturedErr.Error(), "not found in page") {
		t.Errorf("Expected 'not found in page' error, got: %v", capturedErr)
	}
}

// Function that returns wrong type (for testing executeRenderOp error)
func badFunctionReturnsString() string {
	return "not a component"
}

// Test page that tries to render function with wrong return type
type renderBadFunctionPage struct{}

func (renderBadFunctionPage) Page(data string) component {
	return testComponent{content: "RenderBad: " + data}
}

func (p renderBadFunctionPage) Props(r *http.Request, target RenderTarget) (string, error) {
	// Try to render a function that returns wrong type
	return "", RenderComponent(badFunctionReturnsString)
}

// Test executeRenderOp error when function returns wrong type
func TestHandleRenderComponentError_ExecuteRenderOpFails(t *testing.T) {
	type pages struct {
		renderBadFunctionPage `route:"/ RenderBad"`
	}

	mux := http.NewServeMux()
	var capturedErr error
	sp, err := Mount(mux, &pages{}, "/", "Test", WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
		capturedErr = err
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
	}))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	_ = sp

	// Make HTMX request
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", "some-target")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Should get error because function returns wrong type
	if capturedErr == nil {
		t.Error("Expected error when function returns wrong type")
	}
	if capturedErr != nil && !strings.Contains(capturedErr.Error(), "error rendering component") {
		t.Errorf("Expected 'error rendering component' error, got: %v", capturedErr)
	}
}

// Custom unsupported RenderTarget
type customUnsupportedTarget struct{}

func (customUnsupportedTarget) Is(any) bool { return true }

// Page that uses unsupported RenderTarget
type unsupportedTargetPage struct{}

func (unsupportedTargetPage) Page(data string) component {
	return testComponent{content: "Page: " + data}
}

func (p unsupportedTargetPage) Props(r *http.Request, target RenderTarget) (string, error) {
	// Create and use an unsupported RenderTarget type
	customTarget := customUnsupportedTarget{}
	return "", RenderComponent(customTarget)
}

// Test resolveRenderOp error when renderOpFromTarget fails
func TestResolveRenderOp_RenderOpFromTargetError(t *testing.T) {
	type pages struct {
		unsupportedTargetPage `route:"/ UnsupportedTarget"`
	}

	mux := http.NewServeMux()
	var capturedErr error
	sp, err := Mount(mux, &pages{}, "/", "Test", WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
		capturedErr = err
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
	}))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	_ = sp

	// Make HTMX request
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", "some-target")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Should get error because customUnsupportedTarget is not supported
	if capturedErr == nil {
		t.Error("Expected error when using unsupported RenderTarget type")
	}
	if capturedErr != nil && !strings.Contains(capturedErr.Error(), "unsupported RenderTarget type") {
		t.Errorf("Expected 'unsupported RenderTarget type' error, got: %v", capturedErr)
	}
}
