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
		return "", RenderPageComponent(&renderTestErrorPage{}, "ErrorComponent", "Something went wrong")
	case "notfound":
		return "", RenderPageComponent(&renderTestErrorPage{}, "NotFoundComponent")
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

	sp := New()
	router := NewRouter(http.NewServeMux())

	if err := sp.MountPages(router, &pages{}, "/", "Test"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

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

	sp := New()
	router := NewRouter(http.NewServeMux())

	if err := sp.MountPages(router, &pages{}, "/", "Test"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test?trigger=error", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

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

	sp := New()
	router := NewRouter(http.NewServeMux())

	if err := sp.MountPages(router, &pages{}, "/", "Test"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test?trigger=notfound", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

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
	return "", RenderPageComponent(&multiArgPage{}, "MultiComponent", "test", 5, true)
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

	sp := New()
	router := NewRouter(http.NewServeMux())

	if err := sp.MountPages(router, &pages{}, "/", "Test"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/multitest", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

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

func (i invalidPageTestPage) Props(r *http.Request) (string, error) {
	// Reference a page that doesn't exist in the router
	return "", RenderPageComponent(&struct{}{}, "SomeComponent")
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

	sp := New(WithErrorHandler(errorHandler))
	router := NewRouter(http.NewServeMux())

	if err := sp.MountPages(router, &pages{}, "/", "Test"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/invalid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	if capturedError == nil {
		t.Error("expected error to be captured")
	} else {
		expectedErrorSubstring := "findPageNode: no page node found"
		if !strings.Contains(capturedError.Error(), expectedErrorSubstring) {
			t.Errorf("expected error to contain %q, got %q", expectedErrorSubstring, capturedError.Error())
		}
	}
}

// Test error handling when component not found
type invalidComponentTestPage struct{}

func (i invalidComponentTestPage) Props(r *http.Request) (string, error) {
	return "", RenderPageComponent(&renderTestErrorPage{}, "NonExistentComponent")
}

func (i invalidComponentTestPage) Page(message string) component {
	return renderTestSuccessComponent{message: message}
}

// Test RenderPageComponent with invalid component
func TestRenderPageComponentInvalidComponent(t *testing.T) {
	type pages struct {
		invalidComponentTestPage `route:"/invalidcomp"`
		renderTestErrorPage      `route:"/error"`
	}

	var capturedError error
	errorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		capturedError = err
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	sp := New(WithErrorHandler(errorHandler))
	router := NewRouter(http.NewServeMux())

	if err := sp.MountPages(router, &pages{}, "/", "Test"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/invalidcomp", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	if capturedError == nil {
		t.Error("expected error to be captured")
	} else {
		expectedErrorSubstring := "component NonExistentComponent not found"
		if !strings.Contains(capturedError.Error(), expectedErrorSubstring) {
			t.Errorf("expected error to contain %q, got %q", expectedErrorSubstring, capturedError.Error())
		}
	}
}

// Test RenderPageComponent with different component methods
type componentVariationsPage struct{}

func (c componentVariationsPage) Props(r *http.Request) (string, error) {
	component := r.URL.Query().Get("component")
	switch component {
	case "header":
		return "", RenderPageComponent(&headerPage{}, "Page")
	case "footer":
		return "", RenderPageComponent(&footerPage{}, "Page", "© 2024")
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

	sp := New()
	router := NewRouter(http.NewServeMux())

	if err := sp.MountPages(router, &pages{}, "/", "Test"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/shared?component=header", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

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

	sp := New(WithErrorHandler(errorHandler))
	router := NewRouter(http.NewServeMux())

	if err := sp.MountPages(router, &pages{}, "/", "Test"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/shared?component=footer", nil)
	rec := httptest.NewRecorder()
	capturedError = nil
	router.ServeHTTP(rec, req)

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
		return RenderPageComponent(&renderTestErrorPage{}, "ErrorComponent", "Error from handler")
	case "notfound":
		return RenderPageComponent(&renderTestErrorPage{}, "NotFoundComponent", "ignored")
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

	sp := New()
	router := NewRouter(http.NewServeMux())

	if err := sp.MountPages(router, &pages{}, "/", "Test"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
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
			router.ServeHTTP(rec, req)

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
		return RenderPageComponent(&renderTestErrorPage{}, "ErrorComponent", "Error with logging: "+logger)
	case "multi":
		return RenderPageComponent(&multiArgPage{}, "MultiComponent", "extended", 3, false)
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

	sp := New()
	router := NewRouter(http.NewServeMux())

	// Provide logger arg for dependency injection
	logger := "test-logger"
	if err := sp.MountPages(router, &pages{}, "/", "Test", logger); err != nil {
		t.Fatalf("MountPages failed: %v", err)
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
			router.ServeHTTP(rec, req)

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
		return RenderComponent("AltView", "alternative view")
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

	sp := New()
	router := NewRouter(http.NewServeMux())

	if err := sp.MountPages(router, &pages{}, "/", "Test"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
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
			router.ServeHTTP(rec, req)

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
