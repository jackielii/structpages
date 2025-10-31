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

func TestMatchComponentByTarget(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		pageNode *PageNode
		expected string
	}{
		{
			name:   "simple component name",
			target: "content",
			pageNode: &PageNode{
				Name:  "Index",
				Title: "Index",
				Components: map[string]reflect.Method{
					"Content": {},
				},
			},
			expected: "Content",
		},
		{
			name:   "with # prefix",
			target: "#content",
			pageNode: &PageNode{
				Name:  "Index",
				Title: "Index",
				Components: map[string]reflect.Method{
					"Content": {},
				},
			},
			expected: "Content",
		},
		{
			name:   "IDFor format - matches with page prefix",
			target: "index-todo-list",
			pageNode: &PageNode{
				Name:  "Index",
				Title: "Index",
				Components: map[string]reflect.Method{
					"TodoList": {},
				},
			},
			expected: "TodoList",
		},
		{
			name:   "IDFor format with # - matches with page prefix",
			target: "#index-todo-list",
			pageNode: &PageNode{
				Name:  "Index",
				Title: "Index",
				Components: map[string]reflect.Method{
					"TodoList": {},
				},
			},
			expected: "TodoList",
		},
		{
			name:   "multi-word page prefix",
			target: "user-profile-settings-form",
			pageNode: &PageNode{
				Name:  "UserProfile",
				Title: "User Profile",
				Components: map[string]reflect.Method{
					"SettingsForm": {},
				},
			},
			expected: "SettingsForm",
		},
		{
			name:   "Name and Title differ - matches using Name",
			target: "index-page-event-list-load-more",
			pageNode: &PageNode{
				Name:  "IndexPage",
				Title: "Home",
				Components: map[string]reflect.Method{
					"EventListLoadMore": {},
				},
			},
			expected: "EventListLoadMore",
		},
		{
			name:   "LoadMore suffix match with page prefix",
			target: "index-page-event-list-load-more",
			pageNode: &PageNode{
				Name: "IndexPage",
				Components: map[string]reflect.Method{
					"LoadMore": {},
				},
			},
			expected: "LoadMore",
		},
		{
			name:   "match without page prefix",
			target: "event-list-load-more",
			pageNode: &PageNode{
				Name:  "IndexPage",
				Title: "Home",
				Components: map[string]reflect.Method{
					"EventListLoadMore": {},
				},
			},
			expected: "EventListLoadMore",
		},
		{
			name:   "no matching component",
			target: "nonexistent",
			pageNode: &PageNode{
				Name:  "Index",
				Title: "Index",
				Components: map[string]reflect.Method{
					"TodoList": {},
				},
			},
			expected: "",
		},
		{
			name:   "empty target",
			target: "",
			pageNode: &PageNode{
				Name:       "Index",
				Title:      "Index",
				Components: map[string]reflect.Method{},
			},
			expected: "",
		},
		{
			name:   "target with spaces (invalid)",
			target: "todo list",
			pageNode: &PageNode{
				Name:       "Index",
				Title:      "Index",
				Components: map[string]reflect.Method{},
			},
			expected: "",
		},
		{
			name:   "exact match preferred over suffix match",
			target: "load-more",
			pageNode: &PageNode{
				Name:  "IndexPage",
				Title: "Home",
				Components: map[string]reflect.Method{
					"EventListLoadMore": {}, // Would match as suffix
					"LoadMore":          {}, // Matches exactly
				},
			},
			// Should prefer exact match "LoadMore" over suffix match "EventListLoadMore"
			expected: "LoadMore",
		},
		{
			name:   "suffix match - only one component",
			target: "list-load-more",
			pageNode: &PageNode{
				Name:  "IndexPage",
				Title: "Home",
				Components: map[string]reflect.Method{
					"EventListLoadMore": {},
				},
			},
			expected: "EventListLoadMore",
		},
		{
			name:   "suffix match with full ID",
			target: "page-event-list-load-more",
			pageNode: &PageNode{
				Name:  "IndexPage",
				Title: "Home",
				Components: map[string]reflect.Method{
					"EventListLoadMore": {},
				},
			},
			expected: "EventListLoadMore",
		},
		{
			name:   "does NOT match by Title - uses Name instead",
			target: "home-content",
			pageNode: &PageNode{
				Name:  "IndexPage",
				Title: "Home", // Title is "Home" but we match by Name
				Components: map[string]reflect.Method{
					"Content": {},
				},
			},
			// "home-content" doesn't match because Name is "IndexPage", not "Home"
			expected: "",
		},
		{
			name:   "matches by Name not Title",
			target: "index-page-content",
			pageNode: &PageNode{
				Name:  "IndexPage",
				Title: "Home", // Title differs from Name
				Components: map[string]reflect.Method{
					"Content": {},
				},
			},
			// Should match because Name is "IndexPage"
			expected: "Content",
		},
		{
			name:   "target ends with fullID - wrapper prefix case",
			target: "wrapper-index-page-load-more",
			pageNode: &PageNode{
				Name:  "IndexPage",
				Title: "Home",
				Components: map[string]reflect.Method{
					"LoadMore": {}, // fullID = "index-page-load-more"
				},
			},
			// target "wrapper-index-page-load-more" ends with fullID "index-page-load-more"
			expected: "LoadMore",
		},
		{
			name:   "target ends with fullID - prefers longer match",
			target: "custom-prefix-index-page-settings-form",
			pageNode: &PageNode{
				Name:  "IndexPage",
				Title: "Home",
				Components: map[string]reflect.Method{
					"SettingsForm": {}, // fullID = "index-page-settings-form"
					"Form":         {}, // fullID = "index-page-form"
				},
			},
			// Both match as suffix, but "index-page-settings-form" is longer
			expected: "SettingsForm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchComponentByTarget(tt.target, tt.pageNode)
			if result != tt.expected {
				t.Errorf("matchComponentByTarget(%q) = %q, want %q",
					tt.target, result, tt.expected)
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
			name:    "non-HTMX request returns Page",
			headers: map[string]string{},
			pageNode: &PageNode{
				Name:       "Index",
				Title:      "Index",
				Components: map[string]reflect.Method{},
			},
			expected: "Page",
		},
		{
			name: "HTMX request without target returns Page",
			headers: map[string]string{
				"HX-Request": "true",
			},
			pageNode: &PageNode{
				Name:       "Index",
				Title:      "Index",
				Components: map[string]reflect.Method{},
			},
			expected: "Page",
		},
		{
			name: "HTMX request with simple target",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "content",
			},
			pageNode: &PageNode{
				Name:  "Index",
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
				Name:  "Index",
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
				Name:  "Index",
				Title: "Index",
				Components: map[string]reflect.Method{
					"TodoList": {},
				},
			},
			expected: "TodoList",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			target, err := HTMXRenderTarget(req, tt.pageNode)
			if err != nil {
				t.Errorf("HTMXRenderTarget() unexpected error: %v", err)
				return
			}

			// Extract component name from target
			var result string
			if mrt, ok := target.(*methodRenderTarget); ok {
				result = mrt.name
			} else {
				t.Errorf("HTMXRenderTarget() returned unexpected target type: %T", target)
				return
			}

			if result != tt.expected {
				t.Errorf("HTMXRenderTarget() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// Test that HTMXRenderTarget returns functionRenderTarget for non-matching components
func TestHTMXRenderTarget_FunctionTarget(t *testing.T) {
	pn := &PageNode{
		Name:  "Index",
		Title: "Index",
		Components: map[string]reflect.Method{
			"Page": {Name: "Page"},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", "index-nonexistent")

	target, err := HTMXRenderTarget(req, pn)
	if err != nil {
		t.Errorf("HTMXRenderTarget() unexpected error: %v", err)
		return
	}

	// Should return functionRenderTarget for non-matching component
	if _, ok := target.(*functionRenderTarget); !ok {
		t.Errorf("HTMXRenderTarget() should return functionRenderTarget for non-matching component, got %T", target)
	}
}

func TestHTMXRenderTarget_Default(t *testing.T) {
	// Test that HTMXRenderTarget is the default target selector
	sp, _ := Mount(nil, struct{}{}, "/", "Test")
	if sp.targetSelector == nil {
		t.Error("Expected target selector to be set")
	}

	// Verify it behaves like HTMXRenderTarget
	pn := &PageNode{
		Title: "Test",
		Components: map[string]reflect.Method{
			"Content": {},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", "content")

	target, err := sp.targetSelector(req, pn)
	if err != nil {
		t.Errorf("target selector error: %v", err)
	}

	// Extract component name
	mrt, ok := target.(*methodRenderTarget)
	if !ok {
		t.Errorf("expected methodRenderTarget, got %T", target)
		return
	}
	if mrt.name != "Content" {
		t.Errorf("expected default selector to return Content, got %q", mrt.name)
	}
}
