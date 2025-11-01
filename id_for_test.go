package structpages

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// TestCamelToKebab tests the camelToKebab conversion function
func TestCamelToKebab(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple camelCase",
			input:    "userList",
			expected: "user-list",
		},
		{
			name:     "simple PascalCase",
			input:    "UserList",
			expected: "user-list",
		},
		{
			name:     "single word lowercase",
			input:    "user",
			expected: "user",
		},
		{
			name:     "single word uppercase",
			input:    "User",
			expected: "user",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "multiple consecutive uppercase",
			input:    "HTMLParser",
			expected: "html-parser",
		},
		{
			name:     "all uppercase acronym",
			input:    "HTTP",
			expected: "http",
		},
		{
			name:     "mixed case with acronym",
			input:    "parseHTMLContent",
			expected: "parse-html-content",
		},
		{
			name:     "long camelCase",
			input:    "getUserProfileDataFromDatabase",
			expected: "get-user-profile-data-from-database",
		},
		{
			name:     "already kebab-case",
			input:    "user-list",
			expected: "user-list",
		},
		{
			name:     "with numbers",
			input:    "user123List",
			expected: "user123-list",
		},
		{
			name:     "modal container",
			input:    "UserModalContainer",
			expected: "user-modal-container",
		},
		{
			name:     "search input",
			input:    "GroupSearchInput",
			expected: "group-search-input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := camelToKebab(tt.input)
			if result != tt.expected {
				t.Errorf("camelToKebab(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Test page types for ID/IDTarget
type testPageWithMethods struct{}

func (testPageWithMethods) UserList() component       { return testComponent{"UserList"} }
func (testPageWithMethods) UserModal() component      { return testComponent{"UserModal"} }
func (testPageWithMethods) GroupSearch() component    { return testComponent{"GroupSearch"} }
func (testPageWithMethods) userProfile() component    { return testComponent{"userProfile"} }
func (testPageWithMethods) TeamManagement() component { return testComponent{"TeamManagement"} }
func (testPageWithMethods) Content() component        { return testComponent{"Content"} }
func (testPageWithMethods) HTMLContent() component    { return testComponent{"HTMLContent"} }
func (testPageWithMethods) GroupMembers() component   { return testComponent{"GroupMembers"} }

// TestIDTarget tests IDTarget function with method expressions (returns CSS selector)
func TestIDTarget(t *testing.T) {
	// Set up page tree
	type testPages struct {
		test testPageWithMethods `route:"/ Test"`
	}

	pc, err := parsePageTree("/", &testPages{})
	if err != nil {
		t.Fatalf("parsePageTree failed: %v", err)
	}

	// Set up context with parseContext
	ctx := context.Background()
	ctx = pcCtx.WithValue(ctx, pc)

	tests := []struct {
		name     string
		input    any
		expected string
		wantErr  bool
	}{
		{
			name:     "simple method - returns selector",
			input:    testPageWithMethods.UserList,
			expected: "#test-user-list",
		},
		{
			name:     "camelCase method name",
			input:    testPageWithMethods.userProfile,
			expected: "#test-user-profile",
		},
		{
			name:     "with acronym",
			input:    testPageWithMethods.HTMLContent,
			expected: "#test-html-content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IDTarget(ctx, tt.input)
			if tt.wantErr {
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
				t.Errorf("IDTarget() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestID tests ID function with method expressions (returns raw ID)
func TestID(t *testing.T) {
	// Set up page tree
	type testPages struct {
		test testPageWithMethods `route:"/ Test"`
	}

	pc, err := parsePageTree("/", &testPages{})
	if err != nil {
		t.Fatalf("parsePageTree failed: %v", err)
	}

	// Set up context with parseContext
	ctx := context.Background()
	ctx = pcCtx.WithValue(ctx, pc)

	tests := []struct {
		name     string
		input    any
		expected string
		wantErr  bool
	}{
		{
			name:     "simple method - returns raw ID",
			input:    testPageWithMethods.UserList,
			expected: "test-user-list",
		},
		{
			name:     "camelCase method name",
			input:    testPageWithMethods.userProfile,
			expected: "test-user-profile",
		},
		{
			name:     "with acronym",
			input:    testPageWithMethods.HTMLContent,
			expected: "test-html-content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ID(ctx, tt.input)
			if tt.wantErr {
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
				t.Errorf("ID() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// Test error cases
func TestID_Errors(t *testing.T) {
	t.Run("no context", func(t *testing.T) {
		_, err := ID(context.Background(), testPageWithMethods.UserList)
		if err == nil {
			t.Error("Expected error when parseContext not in context")
		}
	})

	t.Run("plain string is treated as literal ID", func(t *testing.T) {
		// Set up page tree
		type testPages struct {
			test testPageWithMethods `route:"/ Test"`
		}

		pc, err := parsePageTree("/", &testPages{})
		if err != nil {
			t.Fatalf("parsePageTree failed: %v", err)
		}

		ctx := pcCtx.WithValue(context.Background(), pc)

		stringTests := []struct {
			name     string
			input    string
			expected string
		}{
			{
				name:     "simple string without #",
				input:    "my-custom-target",
				expected: "my-custom-target",
			},
			{
				name:     "string with # prefix",
				input:    "#my-custom-id",
				expected: "#my-custom-id",
			},
			{
				name:     "empty string",
				input:    "",
				expected: "",
			},
			{
				name:     "string with numbers",
				input:    "item-123",
				expected: "item-123",
			},
			{
				name:     "string with underscores",
				input:    "my_custom_id",
				expected: "my_custom_id",
			},
			{
				name:     "camelCase string",
				input:    "myCustomTarget",
				expected: "myCustomTarget",
			},
			{
				name:     "body",
				input:    "body",
				expected: "body",
			},
			{
				name:     "CSS selector with #",
				input:    "#outer-div",
				expected: "#outer-div",
			},
			{
				name:     "single character",
				input:    "a",
				expected: "a",
			},
		}

		for _, tt := range stringTests {
			t.Run(tt.name, func(t *testing.T) {
				// Both ID and IDTarget return the string as-is
				result, err := IDTarget(ctx, tt.input)
				if err != nil {
					t.Errorf("Unexpected error for plain string: %v", err)
				}
				if result != tt.expected {
					t.Errorf("IDTarget(%q) = %q, want %q", tt.input, result, tt.expected)
				}

				result, err = ID(ctx, tt.input)
				if err != nil {
					t.Errorf("Unexpected error for plain string: %v", err)
				}
				if result != tt.expected {
					t.Errorf("ID(%q) = %q, want %q", tt.input, result, tt.expected)
				}
			})
		}
	})
}

// Test types for real-world examples
type (
	TeamManagementViewTest  struct{}
	AdminManagementViewTest struct{}
)

func (TeamManagementViewTest) UserList() component    { return testComponent{"UserList"} }
func (TeamManagementViewTest) GroupList() component   { return testComponent{"GroupList"} }
func (TeamManagementViewTest) UserModal() component   { return testComponent{"UserModal"} }
func (TeamManagementViewTest) GroupModal() component  { return testComponent{"GroupModal"} }
func (TeamManagementViewTest) UserSearch() component  { return testComponent{"UserSearch"} }
func (TeamManagementViewTest) GroupSearch() component { return testComponent{"GroupSearch"} }

func (AdminManagementViewTest) UserList() component { return testComponent{"UserList"} }

// TestIDFor_RealWorldExamples tests IDFor with real-world usage patterns
func TestIDFor_RealWorldExamples(t *testing.T) {
	// Set up page tree with multiple pages
	type testPages struct {
		teamManagement  TeamManagementViewTest  `route:"/team Team"`
		adminManagement AdminManagementViewTest `route:"/admin Admin"`
	}

	pc, err := parsePageTree("/", &testPages{})
	if err != nil {
		t.Fatalf("parsePageTree failed: %v", err)
	}

	ctx := pcCtx.WithValue(context.Background(), pc)

	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "team user list - selector",
			input:    (*TeamManagementViewTest).UserList,
			expected: "#team-management-user-list",
		},
		{
			name:     "team group list - selector",
			input:    (*TeamManagementViewTest).GroupList,
			expected: "#team-management-group-list",
		},
		{
			name:     "admin user list - different from team",
			input:    (*AdminManagementViewTest).UserList,
			expected: "#admin-management-user-list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IDTarget(ctx, tt.input)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("IDTarget() = %q, want %q", result, tt.expected)
			}
		})
	}

	// Test conflict prevention
	t.Run("conflict prevention", func(t *testing.T) {
		teamID, _ := IDTarget(ctx, (*TeamManagementViewTest).UserList)
		adminID, _ := IDTarget(ctx, (*AdminManagementViewTest).UserList)

		if teamID == adminID {
			t.Errorf("IDs should be different for same method on different pages: team=%q, admin=%q",
				teamID, adminID)
		}
	})
}

// TestIDFor_withRef tests IDFor with Ref type for dynamic method references
//
//nolint:gocyclo // Test function with multiple subtests
func TestIDFor_withRef(t *testing.T) {
	// Set up page tree with multiple pages
	type testPages struct {
		teamManagement  TeamManagementViewTest  `route:"/team Team"`
		adminManagement AdminManagementViewTest `route:"/admin Admin"`
	}

	pc, err := parsePageTree("/", &testPages{})
	if err != nil {
		t.Fatalf("parsePageTree failed: %v", err)
	}

	ctx := pcCtx.WithValue(context.Background(), pc)

	t.Run("qualified reference - PageName.MethodName", func(t *testing.T) {
		id, err := IDTarget(ctx, Ref("teamManagement.UserList"))
		if err != nil {
			t.Errorf("IDFor error: %v", err)
		}
		if id != "#team-management-user-list" {
			t.Errorf("IDTarget() = %q, want %q", id, "#team-management-user-list")
		}

		id, err = IDTarget(ctx, Ref("teamManagement.GroupModal"))
		if err != nil {
			t.Errorf("IDFor error: %v", err)
		}
		if id != "#team-management-group-modal" {
			t.Errorf("IDTarget() = %q, want %q", id, "#team-management-group-modal")
		}
	})

	t.Run("simple method name - unambiguous", func(t *testing.T) {
		// GroupList only exists on TeamManagementViewTest
		id, err := IDTarget(ctx, Ref("GroupList"))
		if err != nil {
			t.Errorf("IDFor error: %v", err)
		}
		if id != "#team-management-group-list" {
			t.Errorf("IDTarget() = %q, want %q", id, "#team-management-group-list")
		}
	})

	t.Run("Ref with ID - returns raw ID", func(t *testing.T) {
		id, err := ID(ctx, Ref("teamManagement.GroupSearch"))
		if err != nil {
			t.Errorf("ID error: %v", err)
		}
		if id != "team-management-group-search" {
			t.Errorf("ID() = %q, want %q", id, "team-management-group-search")
		}
	})

	t.Run("error - ambiguous method name", func(t *testing.T) {
		// UserList exists on both TeamManagementViewTest and AdminManagementViewTest
		_, err := IDTarget(ctx, Ref("UserList"))
		if err == nil {
			t.Error("Expected error for ambiguous method name")
		}
		// Should suggest using qualified name
		expectedSubstr := "found on multiple pages"
		if !strings.Contains(err.Error(), expectedSubstr) {
			t.Errorf("Expected error to contain %q, got %q", expectedSubstr, err.Error())
		}
		// Should list the pages (using field names)
		if !strings.Contains(err.Error(), "teamManagement") || !strings.Contains(err.Error(), "adminManagement") {
			t.Errorf("Expected error to list both pages, got %q", err.Error())
		}
	})

	t.Run("error - method not found", func(t *testing.T) {
		_, err := IDTarget(ctx, Ref("NonExistentMethod"))
		if err == nil {
			t.Error("Expected error for non-existent method")
		}
		expectedMsg := `method "NonExistentMethod" not found on any page`
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("Expected error to contain %q, got %q", expectedMsg, err.Error())
		}
	})

	t.Run("error - page not found", func(t *testing.T) {
		_, err := IDTarget(ctx, Ref("NonExistentPage.UserList"))
		if err == nil {
			t.Error("Expected error for non-existent page")
		}
		expectedMsg := `no page found with name "NonExistentPage"`
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("Expected error to contain %q, got %q", expectedMsg, err.Error())
		}
	})

	t.Run("error - method not on specified page", func(t *testing.T) {
		// GroupList doesn't exist on adminManagement page
		_, err := IDTarget(ctx, Ref("adminManagement.GroupList"))
		if err == nil {
			t.Error("Expected error for method not on page")
		}
		expectedMsg := `method "GroupList" not found on page "adminManagement"`
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("Expected error to contain %q, got %q", expectedMsg, err.Error())
		}
	})
}

// TestStringVsRef tests the difference between plain strings and Ref types
func TestStringVsRef(t *testing.T) {
	// Set up page tree with multiple pages
	type testPages struct {
		teamManagement TeamManagementViewTest `route:"/team Team"`
	}

	pc, err := parsePageTree("/", &testPages{})
	if err != nil {
		t.Fatalf("parsePageTree failed: %v", err)
	}

	ctx := pcCtx.WithValue(context.Background(), pc)

	t.Run("plain string returns literal ID", func(t *testing.T) {
		// Plain string "UserList" should be returned as-is
		id, err := IDTarget(ctx, "UserList")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		// Should return exactly what was passed in
		if id != "UserList" {
			t.Errorf("IDTarget(\"UserList\") = %q, want %q", id, "UserList")
		}
	})

	t.Run("Ref type performs dynamic lookup", func(t *testing.T) {
		// Ref("UserList") should lookup the method and build ID from page name
		id, err := IDTarget(ctx, Ref("UserList"))
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		// Should return the dynamically built ID
		if id != "#team-management-user-list" {
			t.Errorf("IDTarget(Ref(\"UserList\")) = %q, want %q", id, "#team-management-user-list")
		}
	})

	t.Run("plain string vs Ref produce different results", func(t *testing.T) {
		stringID, _ := IDTarget(ctx, "UserList")
		refID, _ := IDTarget(ctx, Ref("UserList"))

		if stringID == refID {
			t.Errorf("Plain string and Ref should produce different IDs: string=%q, ref=%q", stringID, refID)
		}

		// Plain string should be returned as-is
		if stringID != "UserList" {
			t.Errorf("Plain string ID = %q, want %q", stringID, "UserList")
		}

		// Ref should be dynamic lookup
		if refID != "#team-management-user-list" {
			t.Errorf("Ref ID = %q, want %q", refID, "#team-management-user-list")
		}
	})

	t.Run("plain string never errors for non-existent methods", func(t *testing.T) {
		// Plain string "NonExistent" should work (it's returned as-is)
		id, err := IDTarget(ctx, "NonExistent")
		if err != nil {
			t.Errorf("Unexpected error for plain string: %v", err)
		}
		if id != "NonExistent" {
			t.Errorf("IDTarget(\"NonExistent\") = %q, want %q", id, "NonExistent")
		}
	})

	t.Run("Ref errors for non-existent methods", func(t *testing.T) {
		// Ref("NonExistent") should error because method doesn't exist
		_, err := IDTarget(ctx, Ref("NonExistent"))
		if err == nil {
			t.Error("Expected error for non-existent method with Ref")
		}
	})
}

// TestExtractMethodName tests the extractMethodName helper function
func TestExtractMethodName(t *testing.T) {
	t.Run("valid method expression", func(t *testing.T) {
		name := extractMethodName(TeamManagementViewTest.UserList)
		if name != "UserList" {
			t.Errorf("extractMethodName() = %q, want %q", name, "UserList")
		}
	})

	t.Run("non-function returns empty string", func(t *testing.T) {
		name := extractMethodName("not a function")
		if name != "" {
			t.Errorf("extractMethodName() = %q, want empty string", name)
		}

		name = extractMethodName(123)
		if name != "" {
			t.Errorf("extractMethodName() = %q, want empty string", name)
		}

		name = extractMethodName(nil)
		if name != "" {
			t.Errorf("extractMethodName() = %q, want empty string", name)
		}
	})

	t.Run("method with -fm suffix", func(t *testing.T) {
		// Method values (bound methods) have "-fm" suffix internally
		// extractMethodName should strip it
		method := TeamManagementViewTest{}.UserList
		name := extractMethodName(method)
		if name != "UserList" {
			t.Errorf("extractMethodName() = %q, want %q", name, "UserList")
		}
	})
}

// TestExtractReceiverType tests the extractReceiverType helper function
func TestExtractReceiverType(t *testing.T) {
	t.Run("valid method expression", func(t *testing.T) {
		receiverType := extractReceiverType(TeamManagementViewTest.UserList)
		if receiverType == nil {
			t.Fatal("extractReceiverType() returned nil")
		}
		expectedType := reflect.TypeOf(TeamManagementViewTest{})
		if receiverType != expectedType {
			t.Errorf("extractReceiverType() = %v, want %v", receiverType, expectedType)
		}
	})

	t.Run("non-function returns nil", func(t *testing.T) {
		receiverType := extractReceiverType("not a function")
		if receiverType != nil {
			t.Errorf("extractReceiverType() = %v, want nil", receiverType)
		}

		receiverType = extractReceiverType(123)
		if receiverType != nil {
			t.Errorf("extractReceiverType() = %v, want nil", receiverType)
		}

		receiverType = extractReceiverType(nil)
		if receiverType != nil {
			t.Errorf("extractReceiverType() = %v, want nil", receiverType)
		}
	})

	t.Run("normalizes pointer to value type", func(t *testing.T) {
		// Test that pointer receiver is normalized to value type
		receiverType := extractReceiverType((*TeamManagementViewTest).UserList)
		if receiverType == nil {
			t.Fatal("extractReceiverType() returned nil")
		}
		expectedType := reflect.TypeOf(TeamManagementViewTest{})
		if receiverType != expectedType {
			t.Errorf("extractReceiverType() = %v, want %v (should normalize pointer)", receiverType, expectedType)
		}
	})

	t.Run("function with no parameters returns nil", func(t *testing.T) {
		// Create a function with no parameters
		noParamFunc := func() {}
		receiverType := extractReceiverType(noParamFunc)
		if receiverType != nil {
			t.Errorf("extractReceiverType() = %v, want nil for function with no parameters", receiverType)
		}
	})
}

// Test page type for IDFor error cases
type idForErrorTestPage struct{}

func (idForErrorTestPage) Page() component {
	return testComponent{"page"}
}

func (idForErrorTestPage) Content() component {
	return testComponent{"content"}
}

type idForUnregisteredPage struct{}

func (idForUnregisteredPage) SomeMethod() component {
	return testComponent{"unregistered"}
}

// Test IDFor error cases
func TestIDFor_ErrorCases(t *testing.T) {
	t.Run("context without parseContext", func(t *testing.T) {
		// Call IDFor with a context that doesn't have parseContext
		ctx := context.Background()
		_, err := IDTarget(ctx, idForErrorTestPage.Content)
		if err == nil {
			t.Error("Expected error when parseContext not in context")
		}
		if err != nil && !strings.Contains(err.Error(), "parseContext not found") {
			t.Errorf("Expected 'parseContext not found' error, got: %v", err)
		}
	})

	t.Run("non-function non-string types should error", func(t *testing.T) {
		// Set up a proper context with parseContext
		mux := http.NewServeMux()
		sp, err := Mount(mux, &idForErrorTestPage{}, "/", "Test")
		if err != nil {
			t.Fatalf("Mount failed: %v", err)
		}

		// Create context with parseContext
		ctx := pcCtx.WithValue(context.Background(), sp.pc)

		// Strings should work (returned as-is)
		result, err := IDTarget(ctx, "my-literal-id")
		if err != nil {
			t.Errorf("Unexpected error for string: %v", err)
		}
		if result != "my-literal-id" {
			t.Errorf("IDTarget() = %q, want %q", result, "my-literal-id")
		}

		// But other non-function types should error
		_, err = IDTarget(ctx, 123)
		if err == nil {
			t.Error("Expected error for integer")
		}
		if err != nil && !strings.Contains(err.Error(), "unsupported type") {
			t.Errorf("Expected 'unsupported type' error, got: %v", err)
		}
		if err != nil && !strings.Contains(err.Error(), "int") {
			t.Errorf("Expected error to mention the type 'int', got: %v", err)
		}
	})

	t.Run("function with no receiver", func(t *testing.T) {
		mux := http.NewServeMux()
		sp, err := Mount(mux, &idForErrorTestPage{}, "/", "Test")
		if err != nil {
			t.Fatalf("Mount failed: %v", err)
		}

		ctx := pcCtx.WithValue(context.Background(), sp.pc)

		// Call IDFor with function that has no receiver
		noReceiverFunc := func() component { return testComponent{"test"} }
		_, err = IDTarget(ctx, noReceiverFunc)
		if err == nil {
			t.Error("Expected error for function with no receiver")
		}
		if err != nil && !strings.Contains(err.Error(), "failed to extract receiver type") {
			t.Errorf("Expected 'failed to extract receiver type' error, got: %v", err)
		}
	})

	t.Run("method from unregistered page type", func(t *testing.T) {
		mux := http.NewServeMux()
		sp, err := Mount(mux, &idForErrorTestPage{}, "/", "Test")
		if err != nil {
			t.Fatalf("Mount failed: %v", err)
		}

		ctx := pcCtx.WithValue(context.Background(), sp.pc)

		// Call IDFor with method from unregistered page
		_, err = IDTarget(ctx, idForUnregisteredPage.SomeMethod)
		if err == nil {
			t.Error("Expected error for method from unregistered page")
		}
		if err != nil && !strings.Contains(err.Error(), "cannot find page for method expression") {
			t.Errorf("Expected 'cannot find page for method expression' error, got: %v", err)
		}
	})
}

// Test extractMethodName edge cases
func TestExtractMethodName_EdgeCases(t *testing.T) {
	t.Run("nil function returns empty", func(t *testing.T) {
		// Create a nil function value
		var nilFunc func()
		name := extractMethodName(nilFunc)
		if name != "" {
			t.Errorf("Expected empty name for nil function, got: %q", name)
		}
	})

	t.Run("zero value function returns empty", func(t *testing.T) {
		// Test with a reflect.Value that represents a nil function
		var f func()
		name := extractMethodName(f)
		if name != "" {
			t.Errorf("Expected empty name for zero value function, got: %q", name)
		}
	})
}

// TestIDFor_InstanceMethod tests IDFor with instance method values (bound methods)
// This is the pattern: p.UserList where p is an instance
func TestIDFor_InstanceMethod(t *testing.T) {
	// Set up page tree
	type testPages struct {
		test testPageWithMethods `route:"/ Test"`
	}

	pc, err := parsePageTree("/", &testPages{})
	if err != nil {
		t.Fatalf("parsePageTree failed: %v", err)
	}

	// Set up context with parseContext
	ctx := context.Background()
	ctx = pcCtx.WithValue(ctx, pc)

	// Create an instance - THIS IS THE KEY DIFFERENCE
	p := testPageWithMethods{}

	tests := []struct {
		name     string
		input    any
		expected string
		wantErr  bool
	}{
		{
			name:     "instance method - simple",
			input:    p.UserList,
			expected: "#test-user-list",
		},
		{
			name:     "instance method - another method",
			input:    p.UserModal,
			expected: "#test-user-modal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IDTarget(ctx, tt.input)
			if tt.wantErr {
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
				t.Errorf("IDTarget() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestExtractReceiverType_InstanceMethod tests extractReceiverType with instance methods
func TestExtractReceiverType_InstanceMethod(t *testing.T) {
	p := TeamManagementViewTest{}

	t.Run("instance method (bound method) - returns nil by design", func(t *testing.T) {
		receiverType := extractReceiverType(p.UserList)
		// Bound methods (instance.Method) return nil by design
		// They are handled separately by idForBoundMethod
		if receiverType != nil {
			t.Errorf("extractReceiverType() = %v for bound method, want nil (handled separately)", receiverType)
		}
	})

	t.Run("method expression (unbound) - extracts receiver type", func(t *testing.T) {
		receiverType := extractReceiverType(TeamManagementViewTest.UserList)
		if receiverType == nil {
			t.Fatal("extractReceiverType() returned nil for unbound method expression")
		}
		expectedType := reflect.TypeOf(TeamManagementViewTest{})
		if receiverType != expectedType {
			t.Errorf("extractReceiverType() = %v, want %v", receiverType, expectedType)
		}
	})
}

// TestIDFor_InstanceMethodVsMethodExpression demonstrates that both patterns work
func TestIDFor_InstanceMethodVsMethodExpression(t *testing.T) {
	// Set up page tree
	type testPages struct {
		team TeamManagementViewTest `route:"/team Team"`
	}

	pc, err := parsePageTree("/", &testPages{})
	if err != nil {
		t.Fatalf("parsePageTree failed: %v", err)
	}

	ctx := pcCtx.WithValue(context.Background(), pc)

	// Create an instance
	p := TeamManagementViewTest{}

	t.Run("instance method and method expression produce same ID", func(t *testing.T) {
		// Instance method (bound method)
		idInstance, err := IDTarget(ctx, p.UserList)
		if err != nil {
			t.Fatalf("IDFor with instance method failed: %v", err)
		}

		// Method expression (unbound method)
		idMethodExpr, err := IDTarget(ctx, TeamManagementViewTest.UserList)
		if err != nil {
			t.Fatalf("IDFor with method expression failed: %v", err)
		}

		// They should produce the same ID
		if idInstance != idMethodExpr {
			t.Errorf("Instance method and method expression should produce same ID:\n  instance: %q\n  method expr: %q",
				idInstance, idMethodExpr)
		}

		// Verify it's the expected ID (derived from field name "team")
		expected := "#team-user-list"
		if idInstance != expected {
			t.Errorf("IDTarget() = %q, want %q", idInstance, expected)
		}
	})

	t.Run("both patterns work with ID", func(t *testing.T) {
		// Instance method with ID
		idInstance, err := ID(ctx, p.UserModal)
		if err != nil {
			t.Fatalf("ID with instance method failed: %v", err)
		}

		// Method expression with ID
		idMethodExpr, err := ID(ctx, TeamManagementViewTest.UserModal)
		if err != nil {
			t.Fatalf("IDFor with method expression and RawID failed: %v", err)
		}

		// They should produce the same ID
		if idInstance != idMethodExpr {
			t.Errorf(
				"Instance method and method expression with RawID should produce same ID:\n"+
					"  instance: %q\n  method expr: %q",
				idInstance, idMethodExpr)
		}

		expected := "team-user-modal"
		if idInstance != expected {
			t.Errorf("IDTarget() = %q, want %q", idInstance, expected)
		}
	})
}

// Test pages for error path testing
type idErrorTestPage struct{}

func (idErrorTestPage) SomeMethod() component { return testComponent{"test"} }

type idUnregisteredPage struct{}

func (idUnregisteredPage) UnknownMethod() component { return testComponent{"test"} }

// Test edge cases for coverage
func TestID_ErrorPaths(t *testing.T) {
	type pages struct {
		idErrorTestPage `route:"/ Test"`
	}

	mux := http.NewServeMux()
	sp, err := Mount(mux, &pages{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Test with method expression for method not found on page
	// This should error because the page isn't registered
	_, err = sp.ID(idUnregisteredPage.UnknownMethod)
	if err == nil {
		t.Error("Expected error for unregistered page method")
	}
	if !strings.Contains(err.Error(), "no page node found") {
		t.Errorf("Expected 'no page node found' error, got: %v", err)
	}

	// Test Ref with qualified name for method that doesn't exist
	_, err = sp.ID(Ref("idErrorTestPage.NonExistentMethod"))
	if err == nil {
		t.Error("Expected error for non-existent method")
	}
	if !strings.Contains(err.Error(), "not found on page") {
		t.Errorf("Expected 'not found on page' error, got: %v", err)
	}

	// Test Ref with page name that doesn't exist
	_, err = sp.ID(Ref("NonExistentPage.SomeMethod"))
	if err == nil {
		t.Error("Expected error for non-existent page")
	}
	if !strings.Contains(err.Error(), "no page found") {
		t.Errorf("Expected 'no page found' error, got: %v", err)
	}
}

// Test types for findPageNodeByTypeName
type testPageWithMethod struct{}

func (testPageWithMethod) ExistingMethod() component { return testComponent{content: "test"} }

// Test findPageNodeByTypeName with method not found
func TestFindPageNodeByTypeName_MethodNotFound(t *testing.T) {
	type pages struct {
		testPageWithMethod `route:"/ Test"`
	}

	mux := http.NewServeMux()
	sp, err := Mount(mux, &pages{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Try to find a method that doesn't exist on the page
	info := &methodInfo{
		isBound:          true,
		receiverTypeName: "testPageWithMethod",
		methodName:       "NonExistentMethod",
	}

	_, err = sp.pc.findPageNodeByTypeName(info.receiverTypeName, info.methodName)
	if err == nil {
		t.Error("Expected error for method not found")
	}
	if !strings.Contains(err.Error(), "method \"NonExistentMethod\" not found on page") {
		t.Errorf("Expected 'method not found on page' error, got: %v", err)
	}
}

// Test buildID with empty pageName (standalone function)
func TestBuildID_StandaloneFunction(t *testing.T) {
	// Raw ID for standalone function (no page prefix)
	id := buildID("", "UserStatsWidget", true)
	if id != "user-stats-widget" {
		t.Errorf("buildID with empty pageName (raw) = %q, want %q", id, "user-stats-widget")
	}

	// Selector for standalone function (with # prefix)
	id = buildID("", "UserStatsWidget", false)
	if id != "#user-stats-widget" {
		t.Errorf("buildID with empty pageName (selector) = %q, want %q", id, "#user-stats-widget")
	}
}

// Test findPageNodeForMethod panic with standalone function
func TestFindPageNodeForMethod_PanicOnStandaloneFunction(t *testing.T) {
	type pages struct {
		testPageWithMethod `route:"/ Test"`
	}

	mux := http.NewServeMux()
	sp, err := Mount(mux, &pages{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Create a methodInfo for a standalone function
	info := &methodInfo{
		isFunction: true,
		methodName: "StandaloneFunction",
	}

	// This should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when calling findPageNodeForMethod with standalone function")
		}
	}()

	_, _ = sp.pc.findPageNodeForMethod(info)
}
