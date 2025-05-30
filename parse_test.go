package srx

import (
	"net/http"
	"reflect"
	"testing"
)

func TestParseTag(t *testing.T) {
	tests := []struct {
		name     string
		route    string
		expected struct {
			method string
			path   string
			title  string
		}
	}{
		{
			name:  "Empty route",
			route: "",
			expected: struct {
				method string
				path   string
				title  string
			}{
				method: http.MethodGet,
				path:   "/",
				title:  "",
			},
		},
		{
			name:  "Only path",
			route: "/example",
			expected: struct {
				method string
				path   string
				title  string
			}{
				method: http.MethodGet,
				path:   "/example",
				title:  "",
			},
		},
		{
			name:  "invalid method and path",
			route: "INVALID /example",
			expected: struct {
				method string
				path   string
				title  string
			}{
				method: http.MethodGet,
				path:   "INVALID",
				title:  "/example",
			},
		},
		{
			name:  "Method and path",
			route: "POST /example",
			expected: struct {
				method string
				path   string
				title  string
			}{
				method: "POST",
				path:   "/example",
				title:  "",
			},
		},
		{
			name:  "Method, path, and title",
			route: "PUT /example Update Example",
			expected: struct {
				method string
				path   string
				title  string
			}{
				method: "PUT",
				path:   "/example",
				title:  "Update Example",
			},
		},
		{
			name:  "Invalid method",
			route: "INVALID /example Invalid Method",
			expected: struct {
				method string
				path   string
				title  string
			}{
				method: http.MethodGet,
				path:   "INVALID",
				title:  "/example Invalid Method",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, path, title := parseTag(tt.route)
			actual := struct {
				method string
				path   string
				title  string
			}{
				method: method,
				path:   path,
				title:  title,
			}
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("parseTag(%q) = %+v, want %+v", tt.route, actual, tt.expected)
			}
		})
	}
}
