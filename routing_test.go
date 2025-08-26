package structpages

import (
	"context"
	"errors"
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

// Page with error-returning PageConfig
type errorPageConfigPage struct{}

func (errorPageConfigPage) PageConfig(r *http.Request) (string, error) {
	return "", errors.New("pageconfig error")
}

func (errorPageConfigPage) Page() component { return nil }

// Page with PageConfig returning unknown component
type unknownComponentPage struct{}

func (unknownComponentPage) PageConfig(r *http.Request) string {
	return "UnknownComponent"
}

func (unknownComponentPage) Page() component { return nil }

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

func (errorPropsPage) PageProps() (map[string]any, error) {
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
			name:    "no handler and no children",
			page:    &noHandlerPage{},
			route:   "/no-handler",
			wantErr: "page item noHandlerPage does not have a valid handler or children",
		},
		{
			name:    "child registration error",
			page:    &childErrorPage{},
			route:   "/parent",
			wantErr: "page item child does not have a valid handler or children",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp := New()
			router := NewRouter(http.NewServeMux())

			pc, err := parsePageTree(tt.route, tt.page)
			if err != nil {
				if tt.wantErr != "" && contains(err.Error(), tt.wantErr) {
					return // Expected error during parsing
				}
				t.Fatalf("parsePageTree failed unexpectedly: %v", err)
			}

			if tt.setupPage != nil {
				tt.setupPage(pc.root)
			}

			err = sp.registerPageItem(router, pc, pc.root, tt.middlewares)
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
			name:        "PageConfig method error",
			page:        &errorPageConfigPage{},
			route:       "/error-config",
			requestPath: "/error-config",
			wantError:   "error calling PageConfig method for errorPageConfigPage: pageconfig error",
		},
		{
			name:        "PageConfig unknown component",
			page:        &unknownComponentPage{},
			route:       "/unknown",
			requestPath: "/unknown",
			wantError:   "PageConfig method for unknownComponentPage returned unknown component name: UnknownComponent",
		},
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
			wantError:   "error finding props for errorPropsPage: props error",
		},
		{
			name:        "default page config error",
			page:        &emptyRoutePage{},
			route:       "/default-error",
			requestPath: "/default-error",
			wantError:   "error calling default page config for emptyRoutePage: default config error",
			defaultConfig: func(*http.Request, *PageNode) (string, error) {
				return "", errors.New("default config error")
			},
		},
		{
			name:        "default page config unknown component",
			page:        &mockPage{},
			route:       "/default-unknown",
			requestPath: "/default-unknown",
			wantError:   "default PageConfig for mockPage returned unknown component name: UnknownComponent",
			defaultConfig: func(*http.Request, *PageNode) (string, error) {
				return "UnknownComponent", nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capturedErrors = []error{}
			sp := New(WithErrorHandler(errorHandler))
			if tt.defaultConfig != nil {
				sp.defaultPageConfig = tt.defaultConfig
			}

			router := NewRouter(http.NewServeMux())

			err := sp.MountPages(router, tt.page, tt.route, "Test")
			if err != nil {
				t.Fatalf("MountPages failed: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, tt.requestPath, http.NoBody)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

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

// Page with PageConfig that requires missing argument
type pageConfigMissingArgPage struct{}

func (pageConfigMissingArgPage) PageConfig(r *http.Request, missingArg string) string {
	return "Page"
}

func (pageConfigMissingArgPage) Page() component { return mockComponent{} }

// Test findComponent edge cases
func TestFindComponent_NoPageComponent(t *testing.T) {
	sp := New()
	pc := &parseContext{args: make(argRegistry)}

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
	_, err := sp.findComponent(pc, pn, req)
	if err == nil {
		t.Errorf("expected error for no Page component")
	} else if !contains(err.Error(), "no Page component or PageConfig method found") {
		t.Errorf("unexpected error: %v", err)
	}
}

// Test findComponent callMethod error
func TestFindComponent_CallMethodError(t *testing.T) {
	sp := New()

	// Parse the page to get proper PageNode with Config method
	pc, err := parsePageTree("/test", &pageConfigMissingArgPage{})
	if err != nil {
		t.Fatalf("parsePageTree failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	_, err = sp.findComponent(pc, pc.root, req)
	if err == nil {
		t.Errorf("expected error for PageConfig with missing argument")
	} else if !contains(err.Error(), "error calling PageConfig method") {
		t.Errorf("unexpected error: %v", err)
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

	sp := New(WithErrorHandler(errorHandler))
	pc := &parseContext{args: make(argRegistry)}

	// Don't provide the required string argument
	pn := &PageNode{
		Value: reflect.ValueOf(&extendedNoReturnHandler{}),
		Name:  "extendedNoReturnHandler",
	}

	handler := sp.asHandler(pc, pn)
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

	sp := New(WithErrorHandler(errorHandler))
	pc := &parseContext{args: make(argRegistry)}

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

	handler := sp.buildHandler(pn, pc)
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
		if !contains(lastError.Error(), "does not have a Page or PageConfig method") {
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

	sp := New(WithErrorHandler(errorHandler))
	router := NewRouter(http.NewServeMux())

	err := sp.MountPages(router, &errorInComponentMethodPage{}, "/test", "Test")
	if err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

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
