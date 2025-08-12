package structpages

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/a-h/templ"
	"github.com/google/go-cmp/cmp"
)

func Test_mixedCase(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		s    string
		want string
	}{
		{
			name: "Empty string",
			s:    "",
			want: "",
		},
		{
			name: "Single word",
			s:    "hello",
			want: "Hello",
		},
		{
			name: "Hyphenated words",
			s:    "hello-world",
			want: "HelloWorld",
		},
		{
			name: "Mixed case with hyphens",
			s:    "hello-World",
			want: "HelloWorld",
		},
		{
			name: "Multiple hyphenated words",
			s:    "hello-world-example",
			want: "HelloWorldExample",
		},
		{
			name: "No hyphens, just spaces",
			s:    "hello world",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mixedCase(tt.s)
			// Compare the result with expected value
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("mixedCase() mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func TestHTMXPageRetargetMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		headers        map[string]string
		pageConfig     func(r *http.Request, pn *PageNode) (string, error)
		pageNode       *PageNode
		wantRetarget   bool
		wantRetargetTo string
	}{
		{
			name: "Non-HTMX request should not retarget",
			headers: map[string]string{
				"HX-Target": "content",
			},
			pageConfig: func(r *http.Request, pn *PageNode) (string, error) {
				return "Page", nil
			},
			pageNode:     &PageNode{},
			wantRetarget: false,
		},
		{
			name: "HTMX request with no HX-Target should not retarget",
			headers: map[string]string{
				"HX-Request": "true",
			},
			pageConfig: func(r *http.Request, pn *PageNode) (string, error) {
				return "Page", nil
			},
			pageNode:     &PageNode{},
			wantRetarget: false,
		},
		{
			name: "HTMX request targeting body should not retarget",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "body",
			},
			pageConfig: func(r *http.Request, pn *PageNode) (string, error) {
				return "Page", nil
			},
			pageNode:     &PageNode{},
			wantRetarget: false,
		},
		{
			name: "HTMX request with partial component should not retarget",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "content",
			},
			pageConfig: func(r *http.Request, pn *PageNode) (string, error) {
				return "Content", nil
			},
			pageNode:     &PageNode{},
			wantRetarget: false,
		},
		{
			name: "HTMX request with target but page config returns Page should retarget",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "content",
			},
			pageConfig: func(r *http.Request, pn *PageNode) (string, error) {
				return "Page", nil
			},
			pageNode:       &PageNode{},
			wantRetarget:   true,
			wantRetargetTo: "body",
		},
		{
			name: "HTMX request with target but page config returns page (lowercase) should retarget",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "sidebar",
			},
			pageConfig: func(r *http.Request, pn *PageNode) (string, error) {
				return "page", nil
			},
			pageNode:       &PageNode{},
			wantRetarget:   true,
			wantRetargetTo: "body",
		},
		{
			name: "HTMX request with page config error should not retarget",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "content",
			},
			pageConfig: func(r *http.Request, pn *PageNode) (string, error) {
				return "", fmt.Errorf("config error")
			},
			pageNode:     &PageNode{},
			wantRetarget: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create middleware
			middleware := HTMXPageRetargetMiddleware(tt.pageConfig)

			// Create a test handler that does nothing
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Empty handler for testing
			})

			// Apply middleware
			handler := middleware(nextHandler, tt.pageNode)

			// Create request with headers
			req := httptest.NewRequest("GET", "/test", http.NoBody)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// Create response recorder
			rec := httptest.NewRecorder()

			// Execute handler
			handler.ServeHTTP(rec, req)

			// Check if HX-Retarget header is set correctly
			gotRetarget := rec.Header().Get("HX-Retarget")
			if tt.wantRetarget {
				if gotRetarget != tt.wantRetargetTo {
					t.Errorf("Expected HX-Retarget header to be %q, got %q", tt.wantRetargetTo, gotRetarget)
				}
			} else {
				if gotRetarget != "" {
					t.Errorf("Expected no HX-Retarget header, but got %q", gotRetarget)
				}
			}
		})
	}
}

// testPageForRetarget is a test page with both Page and Content methods
type testPageForRetarget struct{}

func (p testPageForRetarget) Page() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := w.Write([]byte("<html><body>Full Page</body></html>"))
		return err
	})
}

func (p testPageForRetarget) Content() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := w.Write([]byte("<div>Partial Content</div>"))
		return err
	})
}

func TestHTMXPageRetargetMiddleware_Integration(t *testing.T) {
	tests := []struct {
		name          string
		headers       map[string]string
		useHTMXConfig bool
		wantBody      string
		wantRetarget  string
	}{
		{
			name:          "Regular request should render full page",
			headers:       map[string]string{},
			useHTMXConfig: true,
			wantBody:      "<html><body>Full Page</body></html>",
			wantRetarget:  "",
		},
		{
			name: "HTMX request with content target should render partial",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "content",
			},
			useHTMXConfig: true,
			wantBody:      "<div>Partial Content</div>",
			wantRetarget:  "",
		},
		{
			name: "HTMX request with unknown target should render full page and retarget",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "sidebar",
			},
			useHTMXConfig: true,
			wantBody:      "<html><body>Full Page</body></html>",
			wantRetarget:  "body",
		},
		{
			name: "HTMX request targeting body should render full page without retarget",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "body",
			},
			useHTMXConfig: true,
			wantBody:      "<html><body>Full Page</body></html>",
			wantRetarget:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create StructPages with HTMXPageConfig and HTMXPageRetargetMiddleware
			var sp *StructPages
			if tt.useHTMXConfig {
				sp = New(
					WithDefaultPageConfig(HTMXPageConfig),
					WithMiddlewares(HTMXPageRetargetMiddleware(HTMXPageConfig)),
				)
			} else {
				sp = New()
			}

			// Create router
			mux := http.NewServeMux()
			router := NewRouter(mux)

			// Mount the test page
			testPage := testPageForRetarget{}
			err := sp.MountPages(router, testPage, "/test", "Test Page")
			if err != nil {
				t.Fatalf("Failed to handle pages: %v", err)
			}

			// Create request
			req := httptest.NewRequest("GET", "/test", http.NoBody)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// Create response recorder
			rec := httptest.NewRecorder()

			// Serve the request
			mux.ServeHTTP(rec, req)

			// Check response body
			gotBody := rec.Body.String()
			if gotBody != tt.wantBody {
				t.Errorf("Expected body %q, got %q", tt.wantBody, gotBody)
			}

			// Check HX-Retarget header
			gotRetarget := rec.Header().Get("HX-Retarget")
			if gotRetarget != tt.wantRetarget {
				t.Errorf("Expected HX-Retarget header %q, got %q", tt.wantRetarget, gotRetarget)
			}
		})
	}
}
