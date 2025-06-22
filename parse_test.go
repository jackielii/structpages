//lint:file-ignore U1000 Ignore unused code in test file

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
				//lint:ignore U1000 test field
				method string
				//lint:ignore U1000 test field
				path string
				//lint:ignore U1000 test field
				title string
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
				//lint:ignore U1000 test field
				method string
				//lint:ignore U1000 test field
				path string
				//lint:ignore U1000 test field
				title string
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
				//lint:ignore U1000 test field
				method string
				//lint:ignore U1000 test field
				path string
				//lint:ignore U1000 test field
				title string
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
				//lint:ignore U1000 test field
				method string
				//lint:ignore U1000 test field
				path string
				//lint:ignore U1000 test field
				title string
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
				//lint:ignore U1000 test field
				method string
				//lint:ignore U1000 test field
				path string
				//lint:ignore U1000 test field
				title string
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
				//lint:ignore U1000 test field
				method string
				//lint:ignore U1000 test field
				path string
				//lint:ignore U1000 test field
				title string
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
				//lint:ignore U1000 test field
				method string
				//lint:ignore U1000 test field
				path string
				//lint:ignore U1000 test field
				title string
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
	pc, err := parsePageTree("/", &topPage{})
	if err != nil {
		t.Fatalf("parsePageTree failed: %v", err)
	}
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
	pc, err := parsePageTree("/", &topPage{})
	if err != nil {
		t.Fatalf("parsePageTree failed: %v", err)
	}
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
				if err := args.addArg(arg); err != nil {
					t.Fatalf("Failed to add arg: %v", err)
				}
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

// Test parsePageTree field skipping
func TestParsePageTree_skipFields(t *testing.T) {
	// Struct with fields that should be skipped
	type pageWithSkippedFields struct {
		// Field without route tag - should be skipped
		NotARoute string
		// Exported field to test - but string fields cause error
		Page struct{} `route:"/page Page"`
	}

	pc, err := parsePageTree("/", &pageWithSkippedFields{})
	if err != nil {
		t.Fatal(err)
	}

	// Should have one child for the exported field with route tag
	if len(pc.root.Children) != 1 {
		t.Errorf("Expected 1 child, got %d", len(pc.root.Children))
	}
}

// Types for testing
type pageWithNoReturn struct{}

func (p *pageWithNoReturn) NoReturn() {
	// Returns nothing
}

// Test callComponentMethod with no results
func TestParseContext_callComponentMethod_noResults(t *testing.T) {
	pc := &parseContext{args: make(argRegistry)}
	pn := &PageNode{
		Name:  "test",
		Value: reflect.ValueOf(&pageWithNoReturn{}),
	}

	method, _ := reflect.TypeOf(&pageWithNoReturn{}).MethodByName("NoReturn")

	// This should return error because no results
	_, err := pc.callComponentMethod(pn, &method)
	if err == nil {
		t.Error("Expected error from callComponentMethod with no return value")
	}
}

// Test parsePageTree error cases
func TestParsePageTree_errors(t *testing.T) {
	// Test with duplicate argument types
	type testStruct struct{ Value string }
	arg1 := &testStruct{Value: "first"}
	arg2 := &testStruct{Value: "second"}

	_, err := parsePageTree("/", struct{}{}, arg1, arg2)
	if err == nil {
		t.Error("Expected error for duplicate argument types")
	}
}

// Test parsePageTree with non-struct input
func TestParsePageTree_nonStruct(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantErr bool
	}{
		{
			name:    "string input",
			input:   "not a struct",
			wantErr: true,
		},
		{
			name:    "slice input",
			input:   []string{"a", "b"},
			wantErr: true,
		},
		{
			name:    "map input",
			input:   map[string]int{"a": 1},
			wantErr: true,
		},
		{
			name:    "nil input",
			input:   nil,
			wantErr: true,
		},
		{
			name:    "int input",
			input:   42,
			wantErr: true,
		},
		{
			name:    "valid struct",
			input:   struct{}{},
			wantErr: false,
		},
		{
			name:    "pointer to struct",
			input:   &struct{}{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parsePageTree("/", tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePageTree() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

type pageWithWrongReturn struct{}

func (p *pageWithWrongReturn) WrongReturn() string {
	return "not a component"
}

// Test callComponentMethod error case
func TestParseContext_callComponentMethod_wrongReturnType(t *testing.T) {
	pc := &parseContext{args: make(argRegistry)}
	pn := &PageNode{
		Name:  "test",
		Value: reflect.ValueOf(&pageWithWrongReturn{}),
	}

	method, _ := reflect.TypeOf(&pageWithWrongReturn{}).MethodByName("WrongReturn")

	// This should return error because return type is not component
	_, err := pc.callComponentMethod(pn, &method)
	if err == nil {
		t.Error("Expected error from callComponentMethod with wrong return type")
	}
}

type pageWithBadMethod struct{}

func (p *pageWithBadMethod) BadMethod(needsString string) {
	// This method expects string but we'll pass int
}

// Test callMethod error when method call fails
func TestParseContext_callMethod_error(t *testing.T) {
	pc := &parseContext{args: make(argRegistry)}

	pn := &PageNode{
		Name:  "test",
		Value: reflect.ValueOf(&pageWithBadMethod{}),
	}

	method, _ := reflect.TypeOf(&pageWithBadMethod{}).MethodByName("BadMethod")

	// Try to call with wrong type - should cause error
	_, err := pc.callMethod(pn, &method, reflect.ValueOf(123)) // passing int instead of string
	if err == nil {
		t.Error("Expected error from callMethod with wrong argument type")
	}
}
