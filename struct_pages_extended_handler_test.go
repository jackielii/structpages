package structpages

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Test types for extended ServeHTTP with extra arguments
type (
	ExtendedArg1 string
	ExtendedArg2 int
	ExtendedArg3 struct {
		Data string
	}
)

// Handler that returns an error (to test buffered writer path)
type extendedErrorReturningHandler struct{}

func (extendedErrorReturningHandler) ServeHTTP(
	w http.ResponseWriter, r *http.Request, arg1 ExtendedArg1, arg2 ExtendedArg2,
) error {
	// Write some data first
	fmt.Fprintf(w, "arg1=%s, arg2=%d", arg1, arg2)

	// Return error based on query param
	if r.URL.Query().Get("error") == "true" {
		return fmt.Errorf("handler error with args: %s, %d", arg1, arg2)
	}
	return nil
}

// Handler that returns multiple values including error
type extendedMultiReturnHandler struct{}

func (extendedMultiReturnHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, arg ExtendedArg3) (string, error) {
	if r.URL.Query().Get("error") == "true" {
		return "", fmt.Errorf("multi-return error: %s", arg.Data)
	}
	fmt.Fprintf(w, "success with %s", arg.Data)
	return "result", nil
}

func TestExtendedServeHTTPWithExtraArgs(t *testing.T) {
	tests := []struct {
		name           string
		handler        any
		args           []any
		path           string
		query          string
		expectedStatus int
		expectedBody   string
		expectError    bool
	}{
		{
			name:           "error returning handler - success",
			handler:        &extendedErrorReturningHandler{},
			args:           []any{ExtendedArg1("test1"), ExtendedArg2(42)},
			path:           "/error-handler",
			expectedStatus: http.StatusOK,
			expectedBody:   "arg1=test1, arg2=42",
		},
		{
			name:           "error returning handler - error",
			handler:        &extendedErrorReturningHandler{},
			args:           []any{ExtendedArg1("test1"), ExtendedArg2(42)},
			path:           "/error-handler",
			query:          "?error=true",
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "handler error with args: test1, 42", // only error message when handler returns error
			expectError:    true,
		},
		{
			name:           "multi return handler - success",
			handler:        &extendedMultiReturnHandler{},
			args:           []any{ExtendedArg3{Data: "multi-data"}},
			path:           "/multi-handler",
			expectedStatus: http.StatusOK,
			expectedBody:   "success with multi-data",
		},
		{
			name:           "multi return handler - error",
			handler:        &extendedMultiReturnHandler{},
			args:           []any{ExtendedArg3{Data: "multi-data"}},
			path:           "/multi-handler",
			query:          "?error=true",
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "multi-return error: multi-data",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create pages struct based on handler type
			var p any
			var capturedError error
			errorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
				capturedError = err
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

			sp := New(WithErrorHandler(errorHandler))
			r := NewRouter(http.NewServeMux())

			if tt.path == "/error-handler" {
				type errorPages struct {
					extendedErrorReturningHandler `route:"GET /error-handler"`
				}
				p = &errorPages{}
			} else {
				type multiPages struct {
					extendedMultiReturnHandler `route:"GET /multi-handler"`
				}
				p = &multiPages{}
			}

			if err := sp.MountPages(r, p, "/", "Test Extended", tt.args...); err != nil {
				t.Fatalf("MountPages failed: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, tt.path+tt.query, http.NoBody)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			body := rec.Body.String()
			// For error cases, http.Error adds a newline, and buffered content is discarded
			if tt.expectError {
				expectedWithNewline := tt.expectedBody + "\n"
				if body != expectedWithNewline {
					t.Errorf("expected body %q, got %q", expectedWithNewline, body)
				}
			} else if body != tt.expectedBody {
				t.Errorf("expected body %q, got %q", tt.expectedBody, body)
			}

			if tt.expectError && capturedError == nil {
				t.Error("expected error but got none")
			} else if !tt.expectError && capturedError != nil {
				t.Errorf("unexpected error: %v", capturedError)
			}
		})
	}
}

// Handler with wrong argument count expectation
type wrongArgHandler struct{}

// This handler expects an argument that won't be provided
func (wrongArgHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, missing string) {
	fmt.Fprint(w, "should not reach here")
}

// Test callMethod error scenarios
func TestExtendedServeHTTPCallMethodError(t *testing.T) {
	type pages struct {
		wrongArgHandler `route:"GET /wrong"`
	}

	var capturedError error
	errorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		capturedError = err
		t.Logf("Captured error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}

	sp := New(WithErrorHandler(errorHandler))
	router := NewRouter(http.NewServeMux())
	// Don't provide the string argument that the handler expects
	if err := sp.MountPages(router, &pages{}, "/", "Test"); err != nil {
		t.Fatalf("MountPages failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/wrong", http.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	if capturedError == nil {
		t.Error("expected error for missing argument")
	} else if !contains(capturedError.Error(), "error calling ServeHTTP method") {
		t.Errorf("unexpected error message: %v", capturedError)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s != "" && (s[0:len(substr)] == substr || contains(s[1:], substr)))
}
