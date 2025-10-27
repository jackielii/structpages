//lint:file-ignore U1000 Ignore unused code in test file

package structpages

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
type (
	productPage struct{}
	userPage    struct{}
)

func (productPage) Page() component { return testComponent{"product"} }
func (userPage) Page() component    { return testComponent{"user"} }

// Test pages for URLFor_withExtractedParams handler test
type (
	editPage struct{}
	viewPage struct{}
)

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
		{
			name:     "wildcard with slashes in filename",
			page:     []any{func(p *PageNode) bool { return p.Name == "f1" }},
			args:     []any{"f1", "path/to/deep/file.txt"},
			expected: "/contact/path/to/deep/file.txt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			_, err := Mount(mux, &index{}, "/", "index", WithMiddlewares(func(h http.Handler, pn *PageNode) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					url, err := URLFor(r.Context(), tt.page, tt.args...)
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					_, _ = w.Write([]byte(url))
				})
			}))
			if err != nil {
				t.Fatalf("Mount failed: %v", err)
			}
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
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

// Test types for URLFor with Ref
type homePageRef struct{}
type userPageRef struct{}
type productPageRef struct{}

func (homePageRef) Page() component    { return testComponent{"home"} }
func (userPageRef) Page() component    { return testComponent{"user"} }
func (productPageRef) Page() component { return testComponent{"product"} }

// Test URLFor with Ref type for dynamic page references
func TestURLFor_withRef(t *testing.T) {
	type testPages struct {
		home    homePageRef    `route:"/ Home"`
		user    userPageRef    `route:"/user User"`
		product productPageRef `route:"/product/{id} Product"`
	}

	// Parse the page tree
	pc, err := parsePageTree("/", &testPages{})
	if err != nil {
		t.Fatalf("parsePageTree failed: %v", err)
	}

	ctx := pcCtx.WithValue(context.Background(), pc)

	t.Run("Ref by page name", func(t *testing.T) {
		url, err := URLFor(ctx, Ref("home"))
		if err != nil {
			t.Errorf("URLFor error: %v", err)
		}
		if url != "/" {
			t.Errorf("URLFor() = %q, want %q", url, "/")
		}

		url, err = URLFor(ctx, Ref("user"))
		if err != nil {
			t.Errorf("URLFor error: %v", err)
		}
		if url != "/user" {
			t.Errorf("URLFor() = %q, want %q", url, "/user")
		}
	})

	t.Run("Ref by route", func(t *testing.T) {
		url, err := URLFor(ctx, Ref("/"))
		if err != nil {
			t.Errorf("URLFor error: %v", err)
		}
		if url != "/" {
			t.Errorf("URLFor() = %q, want %q", url, "/")
		}

		url, err = URLFor(ctx, Ref("/user"))
		if err != nil {
			t.Errorf("URLFor error: %v", err)
		}
		if url != "/user" {
			t.Errorf("URLFor() = %q, want %q", url, "/user")
		}
	})

	t.Run("Ref with path parameters", func(t *testing.T) {
		url, err := URLFor(ctx, Ref("product"), "123")
		if err != nil {
			t.Errorf("URLFor error: %v", err)
		}
		if url != "/product/123" {
			t.Errorf("URLFor() = %q, want %q", url, "/product/123")
		}

		// Using route reference
		url, err = URLFor(ctx, Ref("/product/{id}"), "456")
		if err != nil {
			t.Errorf("URLFor error: %v", err)
		}
		if url != "/product/456" {
			t.Errorf("URLFor() = %q, want %q", url, "/product/456")
		}
	})

	t.Run("Ref with []any for composition", func(t *testing.T) {
		url, err := URLFor(ctx, []any{Ref("user"), "?tab=profile"})
		if err != nil {
			t.Errorf("URLFor error: %v", err)
		}
		if url != "/user?tab=profile" {
			t.Errorf("URLFor() = %q, want %q", url, "/user?tab=profile")
		}
	})

	t.Run("Ref error - page not found by name", func(t *testing.T) {
		_, err := URLFor(ctx, Ref("NonExistentPage"))
		if err == nil {
			t.Error("Expected error for non-existent page name")
		}
		expectedMsg := `no page found with name "NonExistentPage"`
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("Expected error to contain %q, got %q", expectedMsg, err.Error())
		}
	})

	t.Run("Ref error - page not found by route", func(t *testing.T) {
		_, err := URLFor(ctx, Ref("/nonexistent"))
		if err == nil {
			t.Error("Expected error for non-existent route")
		}
		expectedMsg := `no page found with route "/nonexistent"`
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("Expected error to contain %q, got %q", expectedMsg, err.Error())
		}
	})
}

// Test page for wildcard routes
type fileServer struct{}

func (fileServer) Page() component { return testComponent{"files"} }

// Test URLFor with wildcard routes
func TestURLFor_withWildcardRoutes(t *testing.T) {
	t.Run("URLFor with wildcard containing slashes", func(t *testing.T) {
		// Set up pages with wildcard route
		type testPages struct {
			files fileServer `route:"/files/{path...} File Server"`
		}

		// Parse the page tree
		pc, err := parsePageTree("/", &testPages{})
		if err != nil {
			t.Fatalf("parsePageTree failed: %v", err)
		}

		// Set up context
		ctx := context.Background()
		ctx = pcCtx.WithValue(ctx, pc)

		// Test generating URL with path containing slashes
		url, err := URLFor(ctx, fileServer{}, "docs/api/v1/reference.md")
		if err != nil {
			t.Errorf("URLFor error: %v", err)
		}
		expected := "/files/docs/api/v1/reference.md"
		if url != expected {
			t.Errorf("URLFor() = %q, want %q", url, expected)
		}

		// Test with multiple args (should only use first for wildcard)
		url2, err := URLFor(ctx, fileServer{}, "images/logo.png", "extra", "args")
		if err != nil {
			t.Errorf("URLFor error: %v", err)
		}
		expected2 := "/files/images/logo.png"
		if url2 != expected2 {
			t.Errorf("URLFor() = %q, want %q", url2, expected2)
		}
	})
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
		type testPages struct {
			product productPage `route:"/product/{id} Product"`
		}

		mux := http.NewServeMux()
		_, err := Mount(mux, &testPages{}, "/", "Test")
		if err != nil {
			t.Fatalf("Mount failed: %v", err)
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
		type testPages struct {
			view viewPage `route:"GET /product/{id} View Product"`
			edit editPage `route:"GET /product/{id}/edit Edit Product"`
		}

		mux := http.NewServeMux()
		_, err := Mount(mux, &testPages{}, "/", "Test")
		if err != nil {
			t.Fatalf("Mount failed: %v", err)
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
		{
			name:          "Key-value pairs with context params",
			pattern:       "/user/{a}/{b}/{c}",
			contextParams: map[string]string{"a": "1", "b": "2"},
			args:          []any{"b", "20", "c", "30"},
			expected:      "/user/1/20/30",
		},
		{
			name:          "Key-value pairs partial override with context",
			pattern:       "/api/{version}/users/{userId}/posts/{postId}",
			contextParams: map[string]string{"version": "v1", "userId": "100"},
			args:          []any{"postId", "999", "userId", "200"},
			expected:      "/api/v1/users/200/posts/999",
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

// Test for uncovered lines in formatPathSegments
func TestFormatPathSegments_uncoveredCases(t *testing.T) {
	t.Run("Non-string key in even args", func(t *testing.T) {
		// This should fall through to default case when args look like key-value pairs
		// but have non-string keys (even number of args with non-string in odd position)
		result, err := formatPathSegments(context.Background(), "/user/{a}/{b}", 123, "value")
		if err != nil {
			t.Errorf("Expected no error for non-string key in even args, got: %v", err)
		}
		// Should treat as positional args since it's not valid key-value pairs
		if result != "/user/123/value" {
			t.Errorf("Expected /user/123/value, got: %s", result)
		}
	})

	t.Run("Positional args with pre-filled context", func(t *testing.T) {
		// Pre-fill context with one param
		ctx := urlParamsCtx.WithValue(context.Background(), map[string]string{"a": "1"})

		// Provide one positional arg for the remaining param
		result, err := formatPathSegments(ctx, "/user/{a}/{b}", "2")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if result != "/user/1/2" {
			t.Errorf("Expected /user/1/2, got: %s", result)
		}
	})

	t.Run("Odd number of args less than params", func(t *testing.T) {
		// This tests the default case with insufficient args
		_, err := formatPathSegments(context.Background(), "/user/{a}/{b}/{c}", "1")
		if err == nil {
			t.Error("Expected error for insufficient args")
		}
		if err != nil && !strings.Contains(err.Error(), "not enough arguments") {
			t.Errorf("Expected 'not enough arguments' error, got: %v", err)
		}
	})

	t.Run("Valid key-value pairs that don't provide all params", func(t *testing.T) {
		// Context has some params, key-value pairs provide others but not all
		ctx := urlParamsCtx.WithValue(context.Background(), map[string]string{"a": "1"})

		// Valid key-value pairs but missing param "c"
		_, err := formatPathSegments(ctx, "/user/{a}/{b}/{c}", "b", "2")
		if err == nil {
			t.Error("Expected error for missing param c")
		}
		if err != nil && !strings.Contains(err.Error(), "argument c not found") {
			t.Errorf("Expected error about missing param c, got: %v", err)
		}
	})

	t.Run("Even args with first non-string key", func(t *testing.T) {
		// This should trigger the break in isKeyValuePairs check and fallthrough
		result, err := formatPathSegments(context.Background(), "/user/{a}/{b}/{c}/{d}", 123, "val1", "key2", "val2")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		// Should be treated as positional args
		if result != "/user/123/val1/key2/val2" {
			t.Errorf("Expected /user/123/val1/key2/val2, got: %s", result)
		}
	})

	t.Run("Malformed pattern that causes formatPathSegments error", func(t *testing.T) {
		// Create a context with parse context
		pc, err := parsePageTree("/", &index{})
		if err != nil {
			t.Fatalf("parsePageTree failed: %v", err)
		}
		ctx := pcCtx.WithValue(context.Background(), pc)

		// Use a malformed pattern in the join that will fail formatPathSegments
		_, err = URLFor(ctx, []any{index{}, "/{unclosed"})
		if err == nil {
			t.Error("Expected error for malformed pattern")
		}
		if !strings.Contains(err.Error(), "urlfor:") {
			t.Errorf("Expected urlfor error, got: %v", err)
		}
	})

	t.Run("Even args with non-string at even index triggers fallthrough", func(t *testing.T) {
		// This test specifically targets the uncovered lines:
		// - The check for non-string keys that sets isKeyValuePairs = false and breaks
		// - The fallthrough statement after isKeyValuePairs check

		// Even number of args where arg at index 2 (3rd arg) is not a string
		// The loop checks i=0, i=2, i=4... so having non-string at index 2
		// will trigger isKeyValuePairs = false and break, then fallthrough
		result, err := formatPathSegments(context.Background(), "/user/{a}/{b}/{c}/{d}", "a", "1", 123, "value")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		// Should be treated as positional args due to non-string key
		if result != "/user/a/1/123/value" {
			t.Errorf("Expected /user/a/1/123/value, got: %s", result)
		}

		// Let's also test with non-string at index 0
		result2, err2 := formatPathSegments(context.Background(), "/user/{a}/{b}", 123, "value")
		if err2 != nil {
			t.Errorf("Expected no error, got: %v", err2)
		}
		// Should be treated as positional args due to non-string key
		if result2 != "/user/123/value" {
			t.Errorf("Expected /user/123/value, got: %s", result2)
		}
	})
}

// Direct test for the uncovered lines
func TestFormatPathSegments_nonStringKey(t *testing.T) {
	// This test MUST hit lines 187-189 (isKeyValuePairs = false and break)
	// and line 212 (fallthrough)

	// Even number of args with non-string at even index (0)
	result, err := formatPathSegments(context.Background(), "/user/{a}/{b}", 123, "value")
	if err != nil {
		t.Errorf("Expected no error for non-string key, got: %v", err)
	}
	if result != "/user/123/value" {
		t.Errorf("Expected /user/123/value, got: %s", result)
	}
}

// Test uncovered lines in URLFor
func TestURLFor_uncoveredCases(t *testing.T) {
	t.Run("URLFor with invalid page type", func(t *testing.T) {
		// Create a parse context without the test page type
		pc, err := parsePageTree("/", &index{})
		if err != nil {
			t.Fatalf("parsePageTree failed: %v", err)
		}

		ctx := pcCtx.WithValue(context.Background(), pc)

		// Try to get URL for a page type that doesn't exist
		type unknownPage struct{}
		_, err = URLFor(ctx, unknownPage{})
		if err == nil {
			t.Error("Expected error for unknown page type")
		}
	})

	t.Run("URLFor with malformed pattern in page", func(t *testing.T) {
		// This is tricky to test because we'd need a page with malformed pattern
		// that passes parsePageTree but fails formatPathSegments
		// Skip for now as it's an edge case
	})
}

// TestFormatPathSegments_ForceFallthrough specifically tests the uncovered lines
// where even args have non-string keys and need to fall through to default case
func TestFormatPathSegments_ForceFallthrough(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		args     []any
		expected string
	}{
		{
			// 4 args, 3 params - will enter even args case
			// First arg is non-string, triggering isKeyValuePairs = false
			name:     "non-string key forces fallthrough",
			pattern:  "/api/{a}/{b}/{c}",
			args:     []any{100, "val1", "key2", "val2"},
			expected: "/api/100/val1/key2",
		},
		{
			// 4 args, 2 params - will enter even args case
			// First arg is non-string
			name:     "non-string key with extra args",
			pattern:  "/user/{id}/{name}",
			args:     []any{123, "john", "extra", "arg"},
			expected: "/user/123/john",
		},
		{
			// 6 args, 4 params - will enter even args case
			// Non-string at position 2
			name:     "non-string in middle position",
			pattern:  "/data/{a}/{b}/{c}/{d}",
			args:     []any{"key1", "val1", 999, "val3", "key4", "val4"},
			expected: "/data/key1/val1/999/val3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatPathSegments(context.Background(), tt.pattern, tt.args...)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Test edge case with context params and non-string positional args
func TestFormatPathSegments_ContextWithNonStringPositional(t *testing.T) {
	// When we have a pattern with params pre-filled from context
	// and provide non-string args, they should be treated as positional
	// and fill only the remaining unfilled params
	ctx := context.Background()
	ctx = urlParamsCtx.WithValue(ctx, map[string]string{"a": "xxx"})

	// Pattern has 2 params: {a} and {b}
	// Context provides: a="xxx"
	// Args: "123", "value", "456", "extra" which is even number of string args but not key value pairs
	// Expected: only {b} should be filled with 123
	result, err := formatPathSegments(ctx, "/user/{a}/{b}", "123", "value", "456", "extra")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	expected := "/user/xxx/123"
	if result != expected {
		t.Errorf("Expected %s, got: %s", expected, result)
	}
}

// Test wildcard patterns with slashes
func TestFormatPathSegments_WildcardWithSlashes(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		args     []any
		expected string
	}{
		{
			name:     "wildcard with single slash",
			pattern:  "/file/{filename...}",
			args:     []any{"path/to/file.txt"},
			expected: "/file/path/to/file.txt",
		},
		{
			name:     "wildcard with multiple slashes",
			pattern:  "/static/{path...}",
			args:     []any{"css/themes/dark/main.css"},
			expected: "/static/css/themes/dark/main.css",
		},
		{
			name:     "wildcard with leading slash",
			pattern:  "/assets/{resource...}",
			args:     []any{"/images/logo.png"},
			expected: "/assets//images/logo.png",
		},
		{
			name:     "multiple params with wildcard",
			pattern:  "/user/{id}/files/{path...}",
			args:     []any{"123", "documents/2024/report.pdf"},
			expected: "/user/123/files/documents/2024/report.pdf",
		},
		{
			name:     "wildcard with context params",
			pattern:  "/project/{projectId}/files/{filepath...}",
			args:     []any{"docs/api/v1/reference.md"},
			expected: "/project/{projectId}/files/docs/api/v1/reference.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			// Add context params for the test that needs it
			if tt.name == "wildcard with context params" {
				ctx = urlParamsCtx.WithValue(ctx, map[string]string{"projectId": "myproject"})
				tt.expected = "/project/myproject/files/docs/api/v1/reference.md"
			}

			result, err := formatPathSegments(ctx, tt.pattern, tt.args...)
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
