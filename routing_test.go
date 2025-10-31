package structpages

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// Test types for improving coverage

// Page with empty route
type emptyRoutePage struct{}

func (emptyRoutePage) Page() component { return nil }

// Page with error-returning Middlewares method
type errorMiddlewaresPage struct{}

func (errorMiddlewaresPage) Middlewares(arg string) ([]MiddlewareFunc, error) {
	// This will cause callMethod to fail due to missing argument
	return nil, errors.New("middleware error")
}

func (errorMiddlewaresPage) Page() component { return nil }

// Page with wrong return count from Middlewares
type wrongCountMiddlewaresPage struct{}

func (wrongCountMiddlewaresPage) Middlewares() ([]MiddlewareFunc, string, error) {
	return nil, "extra", nil
}

func (wrongCountMiddlewaresPage) Page() component { return nil }

// Page with wrong return type from Middlewares
type wrongTypeMiddlewaresPage struct{}

func (wrongTypeMiddlewaresPage) Middlewares() string {
	return "wrong type"
}

func (wrongTypeMiddlewaresPage) Page() component { return nil }

// Page with no handler and no children
type noHandlerPage struct{}

// Page with error-returning component
type errorComponentPage struct{}

func (errorComponentPage) Page() component {
	return errorComponent{}
}

func (errorComponentPage) ErrorComponent() component {
	return errorComponent{}
}

type errorComponent struct{}

func (errorComponent) Render(ctx context.Context, w io.Writer) error {
	return errors.New("render error")
}

// Page with error-returning Props method
type errorPropsPage struct{}

func (errorPropsPage) Page() component { return mockComponent{} }

func (errorPropsPage) Props() (map[string]any, error) {
	return nil, errors.New("props error")
}

// Page with invalid component method is created dynamically in TestBuildHandler_InvalidComponentMethod

// Page for child registration error
type childErrorPage struct {
	child noHandlerPage `route:"/child"` //lint:ignore U1000 Used for structtag routing
}

func (childErrorPage) Page() component { return mockComponent{} }

// Test registerPageItem error scenarios
func TestRegisterPageItem_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name        string
		page        any
		route       string
		setupPage   func(*PageNode)
		wantErr     string
		middlewares []MiddlewareFunc
	}{
		{
			name:    "empty route",
			page:    &emptyRoutePage{},
			route:   "/valid",
			wantErr: "page item route is empty",
			setupPage: func(pn *PageNode) {
				pn.Route = "" // Clear the route after parsing
			},
		},
		{
			name:    "error from Middlewares method",
			page:    &errorMiddlewaresPage{},
			route:   "/error-middlewares",
			wantErr: "error calling Middlewares method on errorMiddlewaresPage",
		},
		{
			name:    "wrong return count from Middlewares",
			page:    &wrongCountMiddlewaresPage{},
			route:   "/wrong-count",
			wantErr: "middlewares method on wrongCountMiddlewaresPage did not return single result",
		},
		{
			name:  "wrong return type from Middlewares",
			page:  &wrongTypeMiddlewaresPage{},
			route: "/wrong-type",
			wantErr: "middlewares method on wrongTypeMiddlewaresPage did not return " +
				"[]func(http.Handler, *PageNode) http.Handler",
		},
		{
			name:    "no handler and no children - should be skipped",
			page:    &noHandlerPage{},
			route:   "/no-handler",
			wantErr: "", // No longer an error - pages with no handler and no children are just skipped
		},
		{
			name:    "child with no handler and no children - should be skipped",
			page:    &childErrorPage{},
			route:   "/parent",
			wantErr: "", // No longer an error - child pages with no handler and no children are just skipped
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp := &StructPages{}
			mux := http.NewServeMux()

			pc, err := parsePageTree(tt.route, tt.page)
			if err != nil {
				if tt.wantErr != "" && contains(err.Error(), tt.wantErr) {
					return // Expected error during parsing
				}
				t.Fatalf("parsePageTree failed unexpectedly: %v", err)
			}

			sp.pc = pc // Set the pc on the StructPages instance
			if tt.setupPage != nil {
				tt.setupPage(pc.root)
			}

			err = sp.registerPageItem(mux, pc.root, tt.middlewares)
			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// Test buildHandler error scenarios
func TestBuildHandler_ErrorScenarios(t *testing.T) {
	capturedErrors := []error{}
	errorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		capturedErrors = append(capturedErrors, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	tests := []struct {
		name          string
		page          any
		route         string
		requestPath   string
		wantError     string
		setupPage     func(*PageNode)
		defaultConfig func(*http.Request, *PageNode) (string, error)
	}{
		{
			name:        "render error",
			page:        &errorComponentPage{},
			route:       "/error-render",
			requestPath: "/error-render",
			wantError:   "render error",
		},
		{
			name:        "props error",
			page:        &errorPropsPage{},
			route:       "/error-props",
			requestPath: "/error-props",
			wantError:   "error running props for errorPropsPage: props error",
		},
		{
			name:        "default component selector error",
			page:        &emptyRoutePage{},
			route:       "/default-error",
			requestPath: "/default-error",
			wantError:   "error selecting target for emptyRoutePage: default config error",
			defaultConfig: func(*http.Request, *PageNode) (string, error) {
				return "", errors.New("default config error")
			},
		},
		{
			name:        "default component selector unknown component",
			page:        &mockPage{},
			route:       "/default-unknown",
			requestPath: "/default-unknown",
			wantError:   "error selecting target for mockPage: component not found: UnknownComponent",
			defaultConfig: func(*http.Request, *PageNode) (string, error) {
				return "UnknownComponent", nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capturedErrors = []error{}
			mux := http.NewServeMux()
			sp, err := Mount(mux, tt.page, tt.route, "Test", WithErrorHandler(errorHandler))
			if err != nil {
				t.Fatalf("Mount failed: %v", err)
			}
			if tt.defaultConfig != nil {
				// Wrap old-style selector in new TargetSelector
				oldSelector := tt.defaultConfig
				sp.targetSelector = func(r *http.Request, pn *PageNode) (RenderTarget, error) {
					name, err := oldSelector(r, pn)
					if err != nil {
						return nil, err
					}
					method, ok := pn.Components[name]
					if !ok {
						return nil, fmt.Errorf("component not found: %s", name)
					}
					return newMethodRenderTarget(name, &method), nil
				}
			}

			req := httptest.NewRequest(http.MethodGet, tt.requestPath, http.NoBody)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if len(capturedErrors) == 0 {
				t.Errorf("expected error to be captured, but none were")
			} else {
				lastError := capturedErrors[len(capturedErrors)-1]
				if !contains(lastError.Error(), tt.wantError) {
					t.Errorf("expected error containing %q, got %q", tt.wantError, lastError.Error())
				}
			}
		})
	}
}

// Test TargetSelector edge cases
func TestTargetSelector_NoPageComponent(t *testing.T) {
	sp := &StructPages{
		targetSelector: HTMXRenderTarget,
	}

	// Create a PageNode without Page component manually
	pn := &PageNode{
		Value: reflect.ValueOf(&struct{}{}),
		Name:  "noPageComponentPage",
		Components: map[string]reflect.Method{
			"OtherComponent": {
				Name: "OtherComponent",
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	target, err := sp.targetSelector(req, pn)

	// HTMXRenderTarget should return a target even if Page component doesn't exist
	// The error will occur later when trying to render
	if err != nil {
		t.Errorf("unexpected error from targetSelector: %v", err)
	}
	if target == nil {
		t.Errorf("expected target, got nil")
	}
}

// Test extended handler without buffered writer that returns error
type extendedNoReturnHandler struct{}

func (extendedNoReturnHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, arg string) {
	// This handler doesn't return anything, so no buffered writer
	_, _ = w.Write([]byte("written"))
}

// Test asHandler edge cases
func TestAsHandler_ExtendedHandlerErrors(t *testing.T) {
	capturedErrors := []error{}
	errorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		capturedErrors = append(capturedErrors, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	sp := &StructPages{
		onError: errorHandler,
		pc:      &parseContext{args: make(argRegistry)},
	}

	// Don't provide the required string argument
	pn := &PageNode{
		Value: reflect.ValueOf(&extendedNoReturnHandler{}),
		Name:  "extendedNoReturnHandler",
	}

	handler := sp.asHandler(pn)
	if handler == nil {
		t.Fatal("expected handler, got nil")
	}

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if len(capturedErrors) == 0 {
		t.Errorf("expected error for missing argument")
	}
}

// Handler with component method that requires arguments
type errorInComponentMethodPage struct{}

func (errorInComponentMethodPage) Page(arg string) component {
	// This will fail because we won't provide the string argument
	return mockComponent{}
}

// Test invalid component method
func TestBuildHandler_InvalidComponentMethod(t *testing.T) {
	capturedErrors := []error{}
	errorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		capturedErrors = append(capturedErrors, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// Create a page node with an invalid component method
	pn := &PageNode{
		Value: reflect.ValueOf(&mockPage{}),
		Name:  "invalidMethodPage",
		Components: map[string]reflect.Method{
			"Page": {
				Name: "Page",
				Type: reflect.TypeOf((*mockPage)(nil)).Method(0).Type,
				Func: reflect.Value{}, // Invalid Func field
			},
		},
	}

	sp := &StructPages{
		onError: errorHandler,
		pc:      &parseContext{args: make(argRegistry)},
		targetSelector: func(r *http.Request, pageNode *PageNode) (RenderTarget, error) {
			// Return the invalid Page method
			method := pageNode.Components["Page"]
			return newMethodRenderTarget("Page", &method), nil
		},
	}

	handler := sp.buildHandler(pn)
	if handler == nil {
		t.Fatal("expected handler, got nil")
	}

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if len(capturedErrors) == 0 {
		t.Errorf("expected error for invalid component method")
	} else {
		lastError := capturedErrors[len(capturedErrors)-1]
		if !contains(lastError.Error(), "does not have a Page component method") {
			t.Errorf("unexpected error: %v", lastError.Error())
		}
	}
}

// Test component method that fails when called
func TestBuildHandler_ComponentMethodError(t *testing.T) {
	capturedErrors := []error{}
	errorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		capturedErrors = append(capturedErrors, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	mux := http.NewServeMux()
	_, err := Mount(mux, &errorInComponentMethodPage{}, "/test", "Test", WithErrorHandler(errorHandler))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if len(capturedErrors) == 0 {
		t.Errorf("expected error for component method call")
	} else {
		lastError := capturedErrors[len(capturedErrors)-1]
		if !contains(lastError.Error(), "error calling component errorInComponentMethodPage.Page") {
			t.Errorf("unexpected error: %v", lastError.Error())
		}
	}
}

// Mock component for testing
type mockComponent struct{}

func (mockComponent) Render(ctx context.Context, w io.Writer) error {
	_, err := w.Write([]byte("mock"))
	return err
}

// Minimal mock page for testing
type mockPage struct{}

func (mockPage) Page() component { return mockComponent{} }
