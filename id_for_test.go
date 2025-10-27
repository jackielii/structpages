package structpages

import (
	"context"
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

// Test page types for IDFor
type testPageWithMethods struct{}

func (testPageWithMethods) UserList() component       { return testComponent{"UserList"} }
func (testPageWithMethods) UserModal() component      { return testComponent{"UserModal"} }
func (testPageWithMethods) GroupSearch() component    { return testComponent{"GroupSearch"} }
func (testPageWithMethods) userProfile() component    { return testComponent{"userProfile"} }
func (testPageWithMethods) TeamManagement() component { return testComponent{"TeamManagement"} }
func (testPageWithMethods) Content() component        { return testComponent{"Content"} }
func (testPageWithMethods) HTMLContent() component    { return testComponent{"HTMLContent"} }
func (testPageWithMethods) GroupMembers() component   { return testComponent{"GroupMembers"} }

// TestIDFor tests the IDFor function with method expressions
func TestIDFor(t *testing.T) {
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
			name: "method with suffix - returns selector",
			input: IDParams{
				Method:   testPageWithMethods.UserModal,
				Suffixes: []string{"container"},
			},
			expected: "#test-user-modal-container",
		},
		{
			name: "method with multiple suffixes",
			input: IDParams{
				Method:   testPageWithMethods.GroupSearch,
				Suffixes: []string{"input", "field"},
			},
			expected: "#test-group-search-input-field",
		},
		{
			name: "raw ID without selector",
			input: IDParams{
				Method: testPageWithMethods.UserList,
				RawID:  true,
			},
			expected: "test-user-list",
		},
		{
			name: "raw ID with suffix",
			input: IDParams{
				Method:   testPageWithMethods.UserModal,
				Suffixes: []string{"container"},
				RawID:    true,
			},
			expected: "test-user-modal-container",
		},
		{
			name:     "camelCase method name",
			input:    testPageWithMethods.userProfile,
			expected: "#test-user-profile",
		},
		{
			name: "PascalCase suffix",
			input: IDParams{
				Method:   testPageWithMethods.TeamManagement,
				Suffixes: []string{"AddUser", "Form"},
			},
			expected: "#test-team-management-add-user-form",
		},
		{
			name:     "with acronym",
			input:    testPageWithMethods.HTMLContent,
			expected: "#test-html-content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IDFor(ctx, tt.input)
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
				t.Errorf("IDFor() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// Test error cases
func TestIDFor_Errors(t *testing.T) {
	t.Run("no context", func(t *testing.T) {
		_, err := IDFor(context.Background(), testPageWithMethods.UserList)
		if err == nil {
			t.Error("Expected error when parseContext not in context")
		}
	})

	t.Run("invalid method expression", func(t *testing.T) {
		// Set up page tree
		type testPages struct {
			test testPageWithMethods `route:"/ Test"`
		}

		pc, err := parsePageTree("/", &testPages{})
		if err != nil {
			t.Fatalf("parsePageTree failed: %v", err)
		}

		ctx := pcCtx.WithValue(context.Background(), pc)

		// Pass a non-function value
		_, err = IDFor(ctx, "not a function")
		if err == nil {
			t.Error("Expected error for non-function input")
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
			name: "team user modal container",
			input: IDParams{
				Method:   (*TeamManagementViewTest).UserModal,
				Suffixes: []string{"container"},
			},
			expected: "#team-management-user-modal-container",
		},
		{
			name: "team group modal raw ID",
			input: IDParams{
				Method: (*TeamManagementViewTest).GroupModal,
				RawID:  true,
			},
			expected: "team-management-group-modal",
		},
		{
			name: "team user search input",
			input: IDParams{
				Method:   (*TeamManagementViewTest).UserSearch,
				Suffixes: []string{"input"},
			},
			expected: "#team-management-user-search-input",
		},
		{
			name:     "admin user list - different from team",
			input:    (*AdminManagementViewTest).UserList,
			expected: "#admin-management-user-list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IDFor(ctx, tt.input)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("IDFor() = %q, want %q", result, tt.expected)
			}
		})
	}

	// Test conflict prevention
	t.Run("conflict prevention", func(t *testing.T) {
		teamID, _ := IDFor(ctx, (*TeamManagementViewTest).UserList)
		adminID, _ := IDFor(ctx, (*AdminManagementViewTest).UserList)

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
		id, err := IDFor(ctx, Ref("teamManagement.UserList"))
		if err != nil {
			t.Errorf("IDFor error: %v", err)
		}
		if id != "#team-management-user-list" {
			t.Errorf("IDFor() = %q, want %q", id, "#team-management-user-list")
		}

		id, err = IDFor(ctx, Ref("teamManagement.GroupModal"))
		if err != nil {
			t.Errorf("IDFor error: %v", err)
		}
		if id != "#team-management-group-modal" {
			t.Errorf("IDFor() = %q, want %q", id, "#team-management-group-modal")
		}
	})

	t.Run("simple method name - unambiguous", func(t *testing.T) {
		// GroupList only exists on TeamManagementViewTest
		id, err := IDFor(ctx, Ref("GroupList"))
		if err != nil {
			t.Errorf("IDFor error: %v", err)
		}
		if id != "#team-management-group-list" {
			t.Errorf("IDFor() = %q, want %q", id, "#team-management-group-list")
		}
	})

	t.Run("Ref with IDParams - qualified", func(t *testing.T) {
		id, err := IDFor(ctx, IDParams{
			Method:   Ref("teamManagement.UserModal"),
			Suffixes: []string{"container"},
		})
		if err != nil {
			t.Errorf("IDFor error: %v", err)
		}
		if id != "#team-management-user-modal-container" {
			t.Errorf("IDFor() = %q, want %q", id, "#team-management-user-modal-container")
		}
	})

	t.Run("Ref with IDParams - RawID", func(t *testing.T) {
		id, err := IDFor(ctx, IDParams{
			Method: Ref("teamManagement.GroupSearch"),
			RawID:  true,
		})
		if err != nil {
			t.Errorf("IDFor error: %v", err)
		}
		if id != "team-management-group-search" {
			t.Errorf("IDFor() = %q, want %q", id, "team-management-group-search")
		}
	})

	t.Run("Ref with IDParams - suffixes and RawID", func(t *testing.T) {
		id, err := IDFor(ctx, IDParams{
			Method:   Ref("teamManagement.UserSearch"),
			Suffixes: []string{"input", "field"},
			RawID:    true,
		})
		if err != nil {
			t.Errorf("IDFor error: %v", err)
		}
		if id != "team-management-user-search-input-field" {
			t.Errorf("IDFor() = %q, want %q", id, "team-management-user-search-input-field")
		}
	})

	t.Run("error - ambiguous method name", func(t *testing.T) {
		// UserList exists on both TeamManagementViewTest and AdminManagementViewTest
		_, err := IDFor(ctx, Ref("UserList"))
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
		_, err := IDFor(ctx, Ref("NonExistentMethod"))
		if err == nil {
			t.Error("Expected error for non-existent method")
		}
		expectedMsg := `method "NonExistentMethod" not found on any page`
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("Expected error to contain %q, got %q", expectedMsg, err.Error())
		}
	})

	t.Run("error - page not found", func(t *testing.T) {
		_, err := IDFor(ctx, Ref("NonExistentPage.UserList"))
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
		_, err := IDFor(ctx, Ref("adminManagement.GroupList"))
		if err == nil {
			t.Error("Expected error for method not on page")
		}
		expectedMsg := `method "GroupList" not found on page "adminManagement"`
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("Expected error to contain %q, got %q", expectedMsg, err.Error())
		}
	})
}
