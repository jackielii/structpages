//lint:file-ignore U1000 Ignore unused code in test file

package structpages

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_formatPathSegments(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		path string
		args []any
		want string
		err  error
	}{
		{
			name: "Empty path",
			path: "",
			args: []any{},
			want: "",
		},
		{
			name: "static path",
			path: "/static/path",
			args: []any{},
			want: "/static/path",
		},
		{
			name: "path with equal param and args",
			path: "/path/{arg1}/{arg2}",
			args: []any{"value1", "value2"},
			want: "/path/value1/value2",
		},
		{
			name: "path with fewer args",
			path: "/path/{arg1}/{arg2}",
			args: []any{"value1"},
			want: "/path/{arg1}/{arg2}",
			err:  errors.New("pattern /path/{arg1}/{arg2}: not enough arguments provided, args: [value1]"),
		},
		// we allow more args
		// {
		// 	name: "path with more args",
		// 	path: "/path/{arg1}/{arg2}",
		// 	args: []any{"value1", "value2", "extra"},
		// 	want: "/path/{arg1}/{arg2}",
		// 	err:  errors.New("pattern /path/{arg1}/{arg2}: too many arguments provided for segment: {arg2}"),
		// },
		{
			name: "path with no args",
			path: "/path/{arg1}/{arg2}",
			args: []any{},
			want: "/path/{arg1}/{arg2}",
			err:  errors.New("pattern /path/{arg1}/{arg2}: no arguments provided"),
		},
		{
			name: "path with map args",
			path: "/path/{arg1}/{arg2}",
			args: []any{map[string]any{"arg1": "value1", "arg2": "value2"}},
			want: "/path/value1/value2",
			err:  nil,
		},
		{
			name: "path with map args missing key",
			path: "/path/{arg1}/{arg2}",
			args: []any{map[string]any{"arg1": "value1"}},
			want: "/path/{arg1}/{arg2}",
			err:  errors.New("pattern /path/{arg1}/{arg2}: argument arg2 not found in provided args: [map[arg1:value1]]"),
		},
		{
			name: "single arg with map",
			path: "/path/{arg1}",
			args: []any{map[string]any{"arg1": "value1"}},
			want: "/path/value1",
		},
		{
			name: "single arg with single value",
			path: "/path/{arg1}",
			args: []any{"value1"},
			want: "/path/value1",
		},
		{
			name: "first arg with map, second with value",
			path: "/path/{arg1}/{arg2}",
			args: []any{map[string]any{"arg1": "value1", "arg2": "value2"}, "value3"},
			want: "/path/value1/value2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatPathSegments(context.Background(), tt.path, tt.args...)
			if tt.err != nil {
				if err == nil {
					t.Errorf("formatPathSegments() expected error, got nil")
					return
				}
				if diff := cmp.Diff(tt.err.Error(), err.Error()); diff != "" {
					t.Errorf("formatPathSegments() error mismatch (-want +got):\n%s", diff)
				}
			}
			if tt.err == nil && err != nil {
				t.Errorf("formatPathSegments() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("formatPathSegments() = %v, want %v", got, tt.want)
			}
		})
	}
}

type index struct {
	product `route:"/product Product"`
	team    `route:"/team Team"`
	contact `route:"/contact Contact"`
	f1      contact `route:"/contact/{f1...} Contact"`
}
type (
	product struct{}
	team    struct{}
	contact struct{}
)

func (index) Page() component   { return testComponent{"index"} }
func (product) Page() component { return testComponent{"product"} }
func (team) Page() component    { return testComponent{"team"} }
func (contact) Page() component { return testComponent{"contact"} }

// Test pages for URLFor_withExtractedParams test
type productPage struct{}
type userPage struct{}

func (productPage) Page() component { return testComponent{"product"} }
func (userPage) Page() component    { return testComponent{"user"} }

// Test pages for URLFor_withExtractedParams handler test
type editPage struct{}
type viewPage struct{}

func (editPage) Page() component { return testComponent{"edit"} }

func (viewPage) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// In the view handler, generate URL to edit page
	// The {id} parameter should be automatically filled from the current request
	editURL, err := URLFor(req.Context(), editPage{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte(editURL))
}

func TestUrlFor(t *testing.T) {
	tests := []struct {
		name     string
		page     any
		args     []any
		expected string
	}{
		{
			name:     "Product page",
			page:     &product{},
			args:     nil,
			expected: "/product",
		},
		{
			name:     "Team page",
			page:     &team{},
			args:     nil,
			expected: "/team",
		},
		{
			name:     "Contact page",
			page:     contact{},
			args:     nil,
			expected: "/contact",
		},
		{
			name:     "Index page",
			page:     index{},
			args:     nil,
			expected: "/",
		},
		{
			name:     "with args",
			page:     []any{product{}, "?page={page}{extra}"},
			args:     []any{"page", "1", "extra", "&sort=asc"},
			expected: "/product?page=1&sort=asc",
		},
		{
			name:     "duplicate type with wildcard",
			page:     []any{func(p *PageNode) bool { return p.Name == "f1" }, "?p={p}"},
			args:     []any{"f1", "extra/path", "p", "0"},
			expected: "/contact/extra/path?p=0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp := New(WithMiddlewares(func(h http.Handler, pn *PageNode) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					url, err := URLFor(r.Context(), tt.page, tt.args...)
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					_, _ = w.Write([]byte(url))
				})
			}))
			r := NewRouter(http.NewServeMux())
			if err := sp.MountPages(r, &index{}, "/", "index"); err != nil {
				t.Fatalf("MountPages failed: %v", err)
			}
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
			}
			if rec.Body.String() != tt.expected {
				t.Errorf("expected body %q, got %q", tt.expected, rec.Body.String())
			}
		})
	}
}

func Test_parseSegments(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		pattern string
		want    []segment
		wantErr bool
	}{
		{
			name:    "Empty pattern",
			pattern: "",
			want:    []segment{},
		},
		{
			name:    "normal pattern",
			pattern: "/path/{id}/resource",
			want: []segment{
				{name: "/path/"},
				{name: "id", param: true},
				{name: "/resource"},
			},
		},
		{
			name:    "pattern with multiple params",
			pattern: "/path/{id}/resource/{action}",
			want: []segment{
				{name: "/path/"},
				{name: "id", param: true},
				{name: "/resource/"},
				{name: "action", param: true},
			},
		},
		{
			name:    "pattern with extra params",
			pattern: "/{action}?x={extra}&y={another}",
			want: []segment{
				{name: "/"},
				{name: "action", param: true},
				{name: "?x="},
				{name: "extra", param: true},
				{name: "&y="},
				{name: "another", param: true},
			},
		},
		{
			name:    "unmatched params",
			pattern: "/path/{id}/resource/{action",
			wantErr: true,
		},
		{
			name:    "just params",
			pattern: "{arg1}{arg2}",
			want: []segment{
				{name: "arg1", param: true},
				{name: "arg2", param: true},
			},
		},
		{
			name:    "wildcard pattern",
			pattern: "/path/{wildcard...}",
			want: []segment{
				{name: "/path/"},
				{name: "wildcard", param: true},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := parseSegments(tt.pattern)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("parseSegments() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("parseSegments() succeeded unexpectedly")
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parseSegments() length mismatch: got %d, want %d: got: %#+v", len(got), len(tt.want), got)
			}
			for i, segment := range got {
				want := tt.want[i]
				if diff := cmp.Diff(segment.name, want.name); diff != "" {
					t.Errorf("parseSegments() mismatch at index %d (-got +want):\n%s", i, diff)
				}
				if segment.param != want.param {
					t.Errorf("parseSegments() param mismatch at index %d: got %v, want %v", i, segment.param, want.param)
				}
			}
		})
	}
}

// Test more edge cases for parseSegments
func TestParseSegments_moreEdgeCases(t *testing.T) {
	// Test the {$} segment handling
	segments, err := parseSegments("/path/{$}/end")
	if err != nil {
		t.Fatal(err)
	}

	// Check that {$} is preserved
	found := false
	for _, seg := range segments {
		if seg.name == "{$}" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find {$} segment")
	}
}

// Test formatPathSegments error case
func TestFormatPathSegments_error(t *testing.T) {
	// Malformed pattern that will cause parseSegments to error
	_, err := formatPathSegments(context.Background(), "/path/{unclosed", "value")
	if err == nil {
		t.Error("Expected error for malformed pattern")
	}
}

// Test URLFor with not found page
func TestURLFor_notFound(t *testing.T) {
	ctx := context.Background()

	type myPage struct{}
	url, err := URLFor(ctx, &myPage{})
	if err == nil {
		t.Error("Expected error for context without parse context")
	}
	if url != "" {
		t.Errorf("Expected empty string for not found page, got %s", url)
	}
}

// Test URLFor with extracted URL parameters from context
func TestURLFor_withExtractedParams(t *testing.T) {
	// Integration test with real page tree
	t.Run("URLFor with context params", func(t *testing.T) {
		// Set up pages
		type testPages struct {
			product productPage `route:"/product/{id} Product"`
		}

		// Parse the page tree
		pc, err := parsePageTree("/", &testPages{})
		if err != nil {
			t.Fatalf("parsePageTree failed: %v", err)
		}

		// Set up context with pre-extracted params
		ctx := context.Background()
		ctx = pcCtx.WithValue(ctx, pc)
		ctx = urlParamsCtx.WithValue(ctx, map[string]string{"id": "123"})

		// Test URLFor with context params
		url, err := URLFor(ctx, productPage{})
		if err != nil {
			t.Errorf("URLFor error: %v", err)
		}
		if url != "/product/123" {
			t.Errorf("URLFor() = %q, want %q", url, "/product/123")
		}

		// Test URLFor with override
		url, err = URLFor(ctx, productPage{}, "456")
		if err != nil {
			t.Errorf("URLFor error: %v", err)
		}
		if url != "/product/456" {
			t.Errorf("URLFor() = %q, want %q", url, "/product/456")
		}
	})

	// Test complete middleware integration
	t.Run("Full integration with middleware", func(t *testing.T) {
		sp := New()
		mux := http.NewServeMux()
		r := NewRouter(mux)

		type testPages struct {
			product productPage `route:"/product/{id} Product"`
		}

		if err := sp.MountPages(r, &testPages{}, "/", "Test"); err != nil {
			t.Fatalf("MountPages failed: %v", err)
		}

		// Create a handler that will check if params are extracted
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			url, err := URLFor(req.Context(), productPage{})
			if err != nil {
				t.Logf("URLFor error in handler: %v", err)
			} else {
				t.Logf("URLFor result in handler: %s", url)
			}
		})

		// Mount test handler at a different route
		mux.HandleFunc("GET /test", testHandler)

		// First request to /product/123 to simulate a real request
		req := httptest.NewRequest(http.MethodGet, "/product/123", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		// The middleware should have extracted params during the above request
		// but they won't be available in a different request.
		// This shows why the feature is useful - params are extracted per request
	})

	// Test real handler scenario
	t.Run("Handler with URLFor using extracted params", func(t *testing.T) {
		sp := New()
		mux := http.NewServeMux()
		r := NewRouter(mux)

		type testPages struct {
			view viewPage `route:"GET /product/{id} View Product"`
			edit editPage `route:"GET /product/{id}/edit Edit Product"`
		}

		if err := sp.MountPages(r, &testPages{}, "/", "Test"); err != nil {
			t.Fatalf("MountPages failed: %v", err)
		}

		// Request the view page with ID 123
		req := httptest.NewRequest(http.MethodGet, "/product/123", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		// The handler should return the edit URL with the same ID
		// without needing to specify the ID parameter explicitly
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
		expectedURL := "/product/123/edit"
		if rec.Body.String() != expectedURL {
			t.Errorf("Expected body %q, got %q", expectedURL, rec.Body.String())
		}
	})
}

// Test formatPathSegments
func TestFormatPathSegmentsWithContext(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		contextParams map[string]string
		args          []any
		expected      string
		expectError   bool
	}{
		{
			name:          "Use params from context",
			pattern:       "/product/{id}",
			contextParams: map[string]string{"id": "123"},
			args:          nil,
			expected:      "/product/123",
		},
		{
			name:          "Override context params",
			pattern:       "/product/{id}",
			contextParams: map[string]string{"id": "123"},
			args:          []any{"456"},
			expected:      "/product/456",
		},
		{
			name:          "Mix context and explicit args",
			pattern:       "/user/{userId}/posts/{postId}",
			contextParams: map[string]string{"userId": "100"},
			args:          []any{map[string]any{"postId": "200"}},
			expected:      "/user/100/posts/200",
		},
		{
			name:          "Missing param in context",
			pattern:       "/product/{id}",
			contextParams: map[string]string{},
			args:          nil,
			expectError:   true,
		},
		{
			name:          "Positional args with context params",
			pattern:       "/user/{a}/{b}/{c}",
			contextParams: map[string]string{"a": "1", "b": "2"},
			args:          []any{"3"},
			expected:      "/user/1/2/3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.contextParams != nil {
				ctx = urlParamsCtx.WithValue(ctx, tt.contextParams)
			}

			result, err := formatPathSegments(ctx, tt.pattern, tt.args...)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("formatPathSegments() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// Test formatPathSegments and parseSegments edge cases
func TestPathSegments_edgeCases(t *testing.T) {
	tests := []struct {
		name     string
		route    string
		values   []any
		expected string
	}{
		{
			name:     "empty route",
			route:    "",
			values:   []any{},
			expected: "",
		},
		{
			name:     "route with consecutive slashes",
			route:    "/users//posts",
			values:   []any{},
			expected: "/users//posts",
		},
		{
			name:     "route with trailing slash and params",
			route:    "/users/{id}/",
			values:   []any{"123"},
			expected: "/users/123/",
		},
		{
			name:     "complex nested params",
			route:    "/api/{version}/users/{id}/posts/{postId}",
			values:   []any{"v1", "123", "456"},
			expected: "/api/v1/users/123/posts/456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatPathSegments(context.Background(), tt.route, tt.values...)
			if err != nil {
				t.Errorf("formatPathSegments() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("formatPathSegments() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test parseSegments with various inputs
func TestParseSegments(t *testing.T) {
	tests := []struct {
		name        string
		route       string
		expectedLen int
		checkFirst  bool
		firstValue  string
		firstParam  bool
	}{
		{
			name:        "simple route",
			route:       "/users/{id}",
			expectedLen: 2,
			checkFirst:  true,
			firstValue:  "/users/",
			firstParam:  false,
		},
		{
			name:        "multiple params",
			route:       "/users/{id}/posts/{postId}",
			expectedLen: 4,
		},
		{
			name:        "no params",
			route:       "/users/list",
			expectedLen: 1,
			checkFirst:  true,
			firstValue:  "/users/list",
			firstParam:  false,
		},
		{
			name:        "param at start",
			route:       "{id}/users",
			expectedLen: 2,
			checkFirst:  true,
			firstValue:  "id",
			firstParam:  true,
		},
		{
			name:        "empty route",
			route:       "",
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSegments(tt.route)
			if err != nil {
				t.Errorf("parseSegments() error = %v", err)
				return
			}
			if len(result) != tt.expectedLen {
				t.Errorf("parseSegments() returned %d segments, want %d", len(result), tt.expectedLen)
				return
			}
			if tt.checkFirst && len(result) > 0 {
				if result[0].name != tt.firstValue {
					t.Errorf("first segment name = %v, want %v", result[0].name, tt.firstValue)
				}
				if result[0].param != tt.firstParam {
					t.Errorf("first segment param = %v, want %v", result[0].param, tt.firstParam)
				}
			}
		})
	}
}
