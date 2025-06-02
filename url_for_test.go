package structpages

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
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
			name: "path with args",
			path: "/path/{arg1}/{arg2}",
			args: []any{"value1", "value2"},
			want: "/path/value1/value2",
		},
		{
			name: "path with fewer args",
			path: "/path/{arg1}/{arg2}",
			args: []any{"value1"},
			want: "/path/{arg1}/{arg2}",
			err:  errors.New("pattern /path/{arg1}/{arg2}: use map[string]any for single arg or provide the full args"),
		},
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
			err:  errors.New("pattern /path/{arg1}/{arg2}: not enough arguments provided for segment: [arg1 arg2]"),
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
			err:  errors.New("pattern /path/{arg1}/{arg2}: argument arg2 not found in provided args"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatPathSegments(tt.path, tt.args...)
			if tt.err != nil {
				if err == nil || err.Error() != tt.err.Error() {
					t.Errorf("formatPathSegments() error = %v, want %v", err, tt.err)
				}
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp := New(WithMiddlewares(func(h http.Handler, pn *PageNode) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					url, err := UrlFor(r.Context(), tt.page, tt.args...)
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					w.Write([]byte(url))
				})
			}))
			r := NewRouter(http.NewServeMux())
			sp.MountPages(r, &index{}, "/", "index")
			req := httptest.NewRequest(http.MethodGet, "/", nil)
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
