package structpages

import (
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
				method: methodAll,
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
				method: methodAll,
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
				method: methodAll,
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
				method: methodAll,
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

func TestParseSimple(t *testing.T) {
	type topPage struct {
		f1 *TestHandlerPage `route:"/ Test Page"`
		f2 *TestHandlerPage `route:"/f2 Test Page 2"`
	}
	pc := parsePageTree("/", &topPage{})
	if pc.root == nil {
		t.Fatal("parsePageTree returned nil")
	}
	s := pc.root.String()
	println(s)
}

func Test_pc_UrlFor(t *testing.T) {
	type topPage struct {
		f1 *TestHandlerPage `route:"/f1 Test Page"`
		f2 *TestHandlerPage `route:"/f2 Test Page 2"`
	}
	pc := parsePageTree("/", &topPage{})
	if pc.root == nil {
		t.Fatal("parsePageTree returned nil")
	}
	{
		url, err := pc.urlFor(&TestHandlerPage{})
		if err != nil {
			t.Fatalf("urlFor failed: %v", err)
		}
		if url != "/f1" {
			t.Errorf("Expected URL '/f1', got '%s'", url)
		}
	}
	{
		url, err := pc.urlFor(&topPage{})
		if err != nil {
			t.Fatalf("urlFor failed: %v", err)
		}
		if url != "/" {
			t.Errorf("Expected URL '/', got '%s'", url)
		}
	}
}
