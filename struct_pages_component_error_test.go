package structpages

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Component that writes partial content before returning an error
type partialWriteErrorComponent struct {
	partialContent string
	errorMsg       string
}

func (c partialWriteErrorComponent) Render(ctx context.Context, w io.Writer) error {
	// Write partial content
	if _, err := fmt.Fprint(w, c.partialContent); err != nil {
		return err
	}
	// Then return an error
	return fmt.Errorf("%s", c.errorMsg)
}

// Page that returns a component that writes partial content before failing
type partialWriteErrorPage struct{}

func (partialWriteErrorPage) Page() component {
	return partialWriteErrorComponent{
		partialContent: "This content should be discarded",
		errorMsg:       "component render failed",
	}
}

// Test that partial component renders are discarded when an error occurs
func TestComponentPartialWriteDiscardedOnError(t *testing.T) {
	type pages struct {
		partialWriteErrorPage `route:"/partial-error"`
	}

	var capturedError error
	errorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		capturedError = err
		http.Error(w, "Component Error: "+err.Error(), http.StatusInternalServerError)
	}

	mux := http.NewServeMux()
	_, err := Mount(mux, &pages{}, "/", "Test", WithErrorHandler(errorHandler))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/partial-error", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Check response
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	body := rec.Body.String()
	// The partial content should NOT be in the response
	if strings.Contains(body, "This content should be discarded") {
		t.Error("partial component content was not discarded on error")
	}

	// Only the error message should be in the response
	expectedBody := "Component Error: component render failed\n"
	if body != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, body)
	}

	// Verify the error was captured
	if capturedError == nil {
		t.Error("expected error to be captured")
	} else if capturedError.Error() != "component render failed" {
		t.Errorf("unexpected error: %v", capturedError)
	}
}

// Component that writes partial content with headers before returning an error
type partialWriteWithHeadersErrorComponent struct{}

func (partialWriteWithHeadersErrorComponent) Render(ctx context.Context, w io.Writer) error {
	// Try to set a header (this won't work since we're writing to a buffer)
	if hw, ok := w.(http.ResponseWriter); ok {
		hw.Header().Set("X-Custom", "should-not-appear")
	}

	// Write partial content
	fmt.Fprint(w, "Partial content with attempted header")

	// Return error
	return fmt.Errorf("render failed after writing")
}

// Test component with multiple partial writes before error
type multiplePartialWritesErrorComponent struct{}

func (multiplePartialWritesErrorComponent) Render(ctx context.Context, w io.Writer) error {
	// Multiple writes
	fmt.Fprint(w, "First partial write. ")
	fmt.Fprint(w, "Second partial write. ")
	fmt.Fprint(w, "Third partial write. ")

	// Then error
	return fmt.Errorf("failed after multiple writes")
}

// Page that returns components with different error scenarios
type complexErrorPage struct{}

func (c complexErrorPage) Page() component {
	return multiplePartialWritesErrorComponent{}
}

func (c complexErrorPage) WithHeaders() component {
	return partialWriteWithHeadersErrorComponent{}
}

func (c complexErrorPage) Props(r *http.Request) error {
	if r.URL.Query().Get("headers") == "true" {
		return RenderComponent((*complexErrorPage).WithHeaders)
	}
	return nil
}

// Test various partial write scenarios
func TestComponentVariousPartialWriteScenarios(t *testing.T) {
	type pages struct {
		complexErrorPage `route:"/complex-error"`
	}

	tests := []struct {
		name             string
		path             string
		expectedStatus   int
		shouldNotContain []string
		expectedError    string
	}{
		{
			name:           "multiple partial writes discarded",
			path:           "/complex-error",
			expectedStatus: http.StatusInternalServerError,
			shouldNotContain: []string{
				"First partial write",
				"Second partial write",
				"Third partial write",
			},
			expectedError: "failed after multiple writes",
		},
		{
			name:           "partial write with headers discarded",
			path:           "/complex-error?headers=true",
			expectedStatus: http.StatusInternalServerError,
			shouldNotContain: []string{
				"Partial content with attempted header",
			},
			expectedError: "render failed after writing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			req := httptest.NewRequest(http.MethodGet, tt.path, http.NoBody)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			// Check status
			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			body := rec.Body.String()

			// Check that partial content is not present
			for _, content := range tt.shouldNotContain {
				if strings.Contains(body, content) {
					t.Errorf("response should not contain %q, but it does: %s", content, body)
				}
			}

			// Check that the custom header is not set
			if h := rec.Header().Get("X-Custom"); h != "" {
				t.Errorf("expected no X-Custom header, got %q", h)
			}

			// Verify the error message
			if !strings.Contains(body, tt.expectedError) {
				t.Errorf("expected error message %q in body, got %q", tt.expectedError, body)
			}

			// Verify error was captured
			if capturedError == nil {
				t.Error("expected error to be captured")
			}
		})
	}
}
