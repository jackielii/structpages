//lint:file-ignore U1000 Ignore unused code in test file

package structpages

import (
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
			got, err := formatPathSegments(tt.path, tt.args...)
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
					url, err := UrlFor(r.Context(), tt.page, tt.args...)
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					_, _ = w.Write([]byte(url))
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
