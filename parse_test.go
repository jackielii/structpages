package structpages

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
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
	if s == "" {
		t.Fatal("Page tree string representation is empty")
	}
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

func Test_parseContext_getArg(t *testing.T) {
	str := "test"
	type strct struct{}
	tests := []struct {
		name string
		args []any
		in   reflect.Type
		want any
	}{
		{
			name: "Simple type",
			args: []any{str},
			in:   reflect.TypeOf("test"),
			want: "test",
		},
		{
			name: "Pointer type",
			args: []any{&str},
			in:   reflect.TypeOf((*string)(nil)),
			want: &str,
		},
		{
			name: "save pointer, ask for value",
			args: []any{&str},
			in:   reflect.TypeOf(""),
			want: "test",
		},
		// {
		// 	name: "save value, ask for pointer",
		// 	args: []any{str},
		// 	in:   reflect.TypeOf((*string)(nil)),
		// 	want: &str,
		// },
		{
			name: "Struct type",
			args: []any{strct{}},
			in:   reflect.TypeOf(strct{}),
			want: strct{},
		},
		{
			name: "Pointer to struct type",
			args: []any{&strct{}},
			in:   reflect.TypeOf((*strct)(nil)),
			want: &strct{},
		},
		{
			name: "save pointer to struct, ask for value",
			args: []any{&strct{}},
			in:   reflect.TypeOf((*strct)(nil)).Elem(),
			want: strct{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := make(argRegistry)
			for _, arg := range tt.args {
				args.addArg(arg)
			}
			got, ok := args.getArg(tt.in)
			if !ok {
				t.Errorf("getArg() did not find type %v", tt.in)
				return
			}
			if diff := cmp.Diff(got.Interface(), tt.want); diff != "" {
				t.Errorf("getArg() mismatch (-got +want):\n%s", diff)
			}
		})
	}
}
