package structpages

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Test page that sets headers in Props method
type pageWithHeaders struct{}

func (p pageWithHeaders) Props(r *http.Request, w http.ResponseWriter) (map[string]string, error) {
	// Set custom headers
	w.Header().Set("X-Custom-Header", "test-value")
	w.Header().Set("X-Request-ID", "12345")

	// Return some data to be used in the page
	return map[string]string{
		"user": "testuser",
		"role": "admin",
	}, nil
}

func (p pageWithHeaders) Page() component {
	return testComponent{content: "<html><body>Page with headers</body></html>"}
}

// Test page that sets cache headers based on content
type cacheablePage struct {
	maxAge int
}

func (p cacheablePage) Props(w http.ResponseWriter, r *http.Request) error {
	// Note: arguments can be in any order thanks to type matching
	if p.maxAge > 0 {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", p.maxAge))
		w.Header().Set("Expires", time.Now().Add(time.Duration(p.maxAge)*time.Second).UTC().Format(http.TimeFormat))
	} else {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	}
	return nil
}

func (p cacheablePage) Page() component {
	return testComponent{content: fmt.Sprintf("<html><body>Cacheable content (max-age: %d)</body></html>", p.maxAge)}
}

// Test page that sets CORS headers
type corsHeadersPage struct{}

func (p corsHeadersPage) Props(w http.ResponseWriter, r *http.Request) error {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	return nil
}

func (p corsHeadersPage) Page() component {
	return testComponent{content: "<html><body>CORS enabled page</body></html>"}
}

// Test page with conditional headers
type conditionalPage struct {
	etag         string
	lastModified time.Time
	skipRender   bool
}

func (p *conditionalPage) Props(r *http.Request, w http.ResponseWriter) error {
	// Set ETag and Last-Modified headers
	if p.etag != "" {
		w.Header().Set("ETag", fmt.Sprintf("%q", p.etag))
	}
	if !p.lastModified.IsZero() {
		w.Header().Set("Last-Modified", p.lastModified.UTC().Format(http.TimeFormat))
	}

	// Check for conditional requests
	if p.etag != "" {
		if match := r.Header.Get("If-None-Match"); match == fmt.Sprintf("%q", p.etag) {
			w.WriteHeader(http.StatusNotModified)
			p.skipRender = true
			return nil
		}
	}

	if !p.lastModified.IsZero() {
		if ims := r.Header.Get("If-Modified-Since"); ims != "" {
			t, err := http.ParseTime(ims)
			if err == nil && !p.lastModified.After(t) {
				w.WriteHeader(http.StatusNotModified)
				p.skipRender = true
				return nil
			}
		}
	}

	return nil
}

func (p *conditionalPage) Page() component {
	if p.skipRender {
		// Don't write anything for 304 responses
		return testComponent{content: ""}
	}
	return testComponent{content: "<html><body>Conditional content</body></html>"}
}

// Helper function
func containsSubstr(s, substr string) bool {
	if s == "" || substr == "" {
		return false
	}
	if s == substr {
		return true
	}
	if len(s) <= len(substr) {
		return false
	}
	return s[:len(substr)] == substr ||
		s[len(s)-len(substr):] == substr ||
		containsSubstrMiddle(s, substr)
}

func containsSubstrMiddle(s, substr string) bool {
	for i := 1; i < len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestPropsWithHeaders(t *testing.T) {
	tests := []struct {
		name           string
		page           any
		route          string
		requestHeaders map[string]string
		wantHeaders    map[string]string
		wantStatus     int
		wantBodyPart   string
	}{
		{
			name:  "Props sets custom headers",
			page:  pageWithHeaders{},
			route: "/headers",
			wantHeaders: map[string]string{
				"X-Custom-Header": "test-value",
				"X-Request-ID":    "12345",
			},
			wantStatus:   http.StatusOK,
			wantBodyPart: "Page with headers",
		},
		{
			name:  "Props sets cache headers for cacheable content",
			page:  cacheablePage{maxAge: 3600},
			route: "/cache",
			wantHeaders: map[string]string{
				"Cache-Control": "public, max-age=3600",
			},
			wantStatus:   http.StatusOK,
			wantBodyPart: "max-age: 3600",
		},
		{
			name:  "Props sets no-cache headers",
			page:  cacheablePage{maxAge: 0},
			route: "/no-cache",
			wantHeaders: map[string]string{
				"Cache-Control": "no-cache, no-store, must-revalidate",
				"Pragma":        "no-cache",
				"Expires":       "0",
			},
			wantStatus:   http.StatusOK,
			wantBodyPart: "max-age: 0",
		},
		{
			name:  "Props sets CORS headers",
			page:  corsHeadersPage{},
			route: "/cors",
			wantHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type",
			},
			wantStatus:   http.StatusOK,
			wantBodyPart: "CORS enabled page",
		},
		{
			name:  "Props handles conditional requests - ETag match",
			page:  &conditionalPage{etag: "v1.0"},
			route: "/conditional",
			requestHeaders: map[string]string{
				"If-None-Match": `"v1.0"`,
			},
			wantHeaders: map[string]string{
				"ETag": `"v1.0"`,
			},
			wantStatus:   http.StatusNotModified,
			wantBodyPart: "", // No body for 304
		},
		{
			name:  "Props handles conditional requests - ETag no match",
			page:  &conditionalPage{etag: "v1.0"},
			route: "/conditional",
			requestHeaders: map[string]string{
				"If-None-Match": `"v0.9"`,
			},
			wantHeaders: map[string]string{
				"ETag": `"v1.0"`,
			},
			wantStatus:   http.StatusOK,
			wantBodyPart: "Conditional content",
		},
		{
			name:  "Props handles conditional requests - Last-Modified",
			page:  &conditionalPage{lastModified: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
			route: "/conditional",
			requestHeaders: map[string]string{
				"If-Modified-Since": time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC).Format(http.TimeFormat),
			},
			wantHeaders: map[string]string{
				"Last-Modified": time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Format(http.TimeFormat),
			},
			wantStatus:   http.StatusNotModified,
			wantBodyPart: "", // No body for 304
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create StructPages instance
			sp := New()

			// Create router
			mux := http.NewServeMux()
			router := NewRouter(mux)

			// Mount the test page
			err := sp.MountPages(router, tt.page, tt.route, "Test Page")
			if err != nil {
				t.Fatalf("Failed to mount page: %v", err)
			}

			// Create request
			req := httptest.NewRequest("GET", tt.route, http.NoBody)
			for key, value := range tt.requestHeaders {
				req.Header.Set(key, value)
			}

			// Create response recorder
			rec := httptest.NewRecorder()

			// Serve the request
			mux.ServeHTTP(rec, req)

			// Check status code
			if rec.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, rec.Code)
			}

			// Check headers
			for key, expectedValue := range tt.wantHeaders {
				gotValue := rec.Header().Get(key)
				if gotValue != expectedValue {
					t.Errorf("Expected header %s=%q, got %q", key, expectedValue, gotValue)
				}
			}

			// Check body content
			gotBody := rec.Body.String()
			if tt.wantBodyPart != "" && !containsSubstr(gotBody, tt.wantBodyPart) {
				t.Errorf("Expected body to contain %q, got %q", tt.wantBodyPart, gotBody)
			}
			if tt.wantBodyPart == "" && gotBody != "" {
				t.Errorf("Expected empty body for 304 response, got %q", gotBody)
			}
		})
	}
}

// Test to verify Props method arguments can be in any order
func TestPropsArgumentOrder(t *testing.T) {
	// Create the test page type
	sp := New()
	mux := http.NewServeMux()
	router := NewRouter(mux)

	// Mount the test page directly
	err := sp.MountPages(router, flexiblePropsPage{}, "/flex", "Flex Page")
	if err != nil {
		t.Fatalf("Failed to mount page: %v", err)
	}

	req := httptest.NewRequest("GET", "/flex", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if got := rec.Header().Get("X-Order"); got != "w-first" {
		t.Errorf("Expected header X-Order=w-first, got %q", got)
	}
}

// Define test page types outside the test function
type flexiblePropsPage struct{}

func (p flexiblePropsPage) Props(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("X-Order", "w-first")
	return nil
}

func (p flexiblePropsPage) Page() component {
	return testComponent{content: "Flexible args"}
}

// Test page that returns error after setting headers
type errorAfterHeadersPage struct{}

func (p errorAfterHeadersPage) Props(w http.ResponseWriter, r *http.Request) error {
	// Set a header
	w.Header().Set("X-Before-Error", "set")
	// Return an error
	return fmt.Errorf("props error after setting header")
}

func (p errorAfterHeadersPage) Page() component {
	return testComponent{content: "Should not render"}
}

// Test error handling when Props returns an error but already wrote headers
func TestPropsHeadersWithError(t *testing.T) {

	sp := New()
	mux := http.NewServeMux()
	router := NewRouter(mux)

	err := sp.MountPages(router, errorAfterHeadersPage{}, "/error", "Error Page")
	if err != nil {
		t.Fatalf("Failed to mount page: %v", err)
	}

	req := httptest.NewRequest("GET", "/error", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// The header should still be set even though Props returned an error
	if got := rec.Header().Get("X-Before-Error"); got != "set" {
		t.Errorf("Expected header X-Before-Error=set, got %q", got)
	}

	// Should get an error response
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rec.Code)
	}
}
