package structpages

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestKebabToPascal(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"content", "Content"},
		{"todo-list", "TodoList"},
		{"user-profile-settings", "UserProfileSettings"},
		{"html-content", "HtmlContent"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := kebabToPascal(tt.input)
			if result != tt.expected {
				t.Errorf("kebabToPascal(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractComponentName(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		pageNode *PageNode
		expected string
	}{
		{
			name:     "simple component name",
			target:   "content",
			pageNode: &PageNode{Title: "Index"},
			expected: "Content",
		},
		{
			name:     "with # prefix",
			target:   "#content",
			pageNode: &PageNode{Title: "Index"},
			expected: "Content",
		},
		{
			name:     "IDFor format - strips page prefix",
			target:   "index-todo-list",
			pageNode: &PageNode{Title: "Index"},
			expected: "TodoList",
		},
		{
			name:     "IDFor format with # - strips page prefix",
			target:   "#index-todo-list",
			pageNode: &PageNode{Title: "Index"},
			expected: "TodoList",
		},
		{
			name:     "multi-word page prefix",
			target:   "user-profile-settings-form",
			pageNode: &PageNode{Title: "UserProfile"},
			expected: "SettingsForm",
		},
		{
			name:     "empty target",
			target:   "",
			pageNode: &PageNode{Title: "Index"},
			expected: "",
		},
		{
			name:     "target with spaces (invalid)",
			target:   "todo list",
			pageNode: &PageNode{Title: "Index"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractComponentName(tt.target, tt.pageNode)
			if result != tt.expected {
				t.Errorf("extractComponentName(%q, %v) = %q, want %q",
					tt.target, tt.pageNode.Title, result, tt.expected)
			}
		})
	}
}

func TestHTMXPageConfig(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		pageNode   *PageNode
		expected   string
		shouldFail bool
	}{
		{
			name:     "non-HTMX request returns Page",
			headers:  map[string]string{},
			pageNode: &PageNode{Title: "Index", Components: map[string]reflect.Method{}},
			expected: "Page",
		},
		{
			name: "HTMX request without target returns Page",
			headers: map[string]string{
				"HX-Request": "true",
			},
			pageNode: &PageNode{Title: "Index", Components: map[string]reflect.Method{}},
			expected: "Page",
		},
		{
			name: "HTMX request with simple target",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "content",
			},
			pageNode: &PageNode{
				Title: "Index",
				Components: map[string]reflect.Method{
					"Content": {},
				},
			},
			expected: "Content",
		},
		{
			name: "HTMX request with IDFor-generated target",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "index-todo-list",
			},
			pageNode: &PageNode{
				Title: "Index",
				Components: map[string]reflect.Method{
					"TodoList": {},
				},
			},
			expected: "TodoList",
		},
		{
			name: "HTMX request with # prefix in target",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "#index-todo-list",
			},
			pageNode: &PageNode{
				Title: "Index",
				Components: map[string]reflect.Method{
					"TodoList": {},
				},
			},
			expected: "TodoList",
		},
		{
			name: "HTMX request with non-existent component falls back to Page",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "index-nonexistent",
			},
			pageNode: &PageNode{
				Title:      "Index",
				Components: map[string]reflect.Method{},
			},
			expected: "Page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result, err := HTMXPageConfig(req, tt.pageNode)
			if err != nil {
				t.Errorf("HTMXPageConfig() unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("HTMXPageConfig() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHTMXPageConfig_Default(t *testing.T) {
	// Test that HTMXPageConfig is the default component selector
	sp := New()
	if sp.defaultComponentSelector == nil {
		t.Error("Expected default component selector to be set")
	}

	// Verify it behaves like HTMXPageConfig
	pn := &PageNode{
		Title: "Test",
		Components: map[string]reflect.Method{
			"Content": {},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", "content")

	result, err := sp.defaultComponentSelector(req, pn)
	if err != nil {
		t.Errorf("default component selector error: %v", err)
	}
	if result != "Content" {
		t.Errorf("expected default selector to return Content, got %q", result)
	}
}
