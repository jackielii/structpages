package structpages

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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
			result := matchComponentByTarget(tt.target, tt.pageNode, nil)
			if result != tt.expected {
				t.Errorf("matchComponentByTarget(%q) = %q, want %q",
					tt.target, result, tt.expected)
			}
		})
	}
}

// --- compacted-id routing regression ---------------------------------------
//
// A deeply nested tree whose detail id exceeds the default maxIDLen (40) and
// whose leaf name ("detail") is shared, so ID() degrades it to the compact
// "<leaf>-<method>-<hash>" form. This mirrors the production topology
// Authed > Admins > Admin > {Permissions,Roles} > Detail, where the
// field-name heuristic produced "detail-detail" but ID() emitted
// "detail-detail-<hash>" — leaving the inspector-pane swap unroutable so the
// page fell back to rendering Page (the full layout) into the drawer.

type hxPermDetail struct{}

func (hxPermDetail) Page() component    { return testComponent{"PERM-PAGE"} }
func (hxPermDetail) Content() component { return testComponent{"PERM-CONTENT"} }
func (hxPermDetail) Detail() component  { return testComponent{"PERM-DETAIL"} }

type hxRoleDetail struct{}

func (hxRoleDetail) Page() component   { return testComponent{"ROLE-PAGE"} }
func (hxRoleDetail) Detail() component { return testComponent{"ROLE-DETAIL"} }

type hxPerms struct {
	Detail hxPermDetail `route:"GET /{permID} Detail"`
}
type hxRoles struct {
	Detail hxRoleDetail `route:"GET /{roleID} Detail"`
}
type hxAdmin struct {
	Permissions hxPerms `route:"/permissions Permissions"`
	Roles       hxRoles `route:"/roles Roles"`
}
type hxAdmins struct {
	Admin hxAdmin `route:"/admin Admin"`
}
type hxAuthed struct {
	Admins hxAdmins `route:"/"`
}
type hxDeepRoot struct {
	Authed hxAuthed `route:"/"`
}

// nodeByRoute walks the tree for the node at fullRoute.
func nodeByRoute(pc *parseContext, fullRoute string) *PageNode {
	for n := range pc.root.All() {
		if n.FullRoute() == fullRoute {
			return n
		}
	}
	return nil
}

func TestMatchComponentByTarget_CompactedID(t *testing.T) {
	pc, err := parsePageTree("/", &hxDeepRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	// Default budget; the permissions detail id is 45 chars (>40) so it
	// compacts, and "detail" is a shared leaf so it carries a hash suffix.
	node := nodeByRoute(pc, "/admin/permissions/{permID}")
	if node == nil {
		t.Fatal("permissions detail node not found")
	}

	id := pc.componentID(node, "Detail", true)

	// Guard: this test is only meaningful if the id actually compacted to the
	// hashed leaf form. If the generator changes, fail loudly rather than
	// silently exercising the non-bug path.
	if !strings.HasPrefix(id, "detail-detail-") || len(id) != len("detail-detail-")+4 {
		t.Fatalf("precondition: expected compacted hashed id 'detail-detail-<hash>', got %q", id)
	}

	// Pass 0 (pc-aware) routes the real id to Detail.
	if got := matchComponentByTarget(id, node, pc); got != "Detail" {
		t.Errorf("matchComponentByTarget(%q, pc) = %q, want %q", id, got, "Detail")
	}
	// Documents the bug: the field-name heuristic alone (pc == nil) cannot
	// regenerate the hash suffix, so it fails to route the compacted id.
	if got := matchComponentByTarget(id, node, nil); got == "Detail" {
		t.Errorf("heuristic-only unexpectedly matched compacted id %q — fix is a no-op", id)
	}
}

// TestHTMXv4RenderTarget_CompactedID is the end-to-end proof: a real mount,
// an HTMX request whose HX-Target is the page's own compacted detail id, must
// render the Detail component — not fall back to Page.
func TestHTMXv4RenderTarget_CompactedID(t *testing.T) {
	mux := http.NewServeMux()
	sp, err := Mount(mux, &hxDeepRoot{}, "/", "App",
		WithTargetSelector(HTMXv4RenderTarget))
	if err != nil {
		t.Fatalf("Mount: %v", err)
	}

	id, err := sp.ID(hxPermDetail.Detail)
	if err != nil {
		t.Fatalf("sp.ID: %v", err)
	}
	if !strings.HasPrefix(id, "detail-detail-") {
		t.Fatalf("precondition: expected compacted id, got %q", id)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/permissions/24", http.NoBody)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", "div#"+id) // htmx 4 sends "<tag>#<id>"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if body := rec.Body.String(); body != "PERM-DETAIL" {
		t.Errorf("compacted-id HX-Target rendered %q, want %q (Page fallback = bug)", body, "PERM-DETAIL")
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

func TestHTMXv4RenderTarget(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		pageNode *PageNode
		// expected: methodRenderTarget name; "" means expect Page (or fallback).
		// expectFunction: true if a functionRenderTarget is expected.
		expected       string
		expectFunction bool
	}{
		{
			name: "v4 tag#id matches component by id",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "main#content",
			},
			pageNode: &PageNode{
				Name:       "Index",
				Title:      "Index",
				Components: map[string]reflect.Method{"Content": {}},
			},
			expected: "Content",
		},
		{
			name: "v4 tag#id with full prefix",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "div#index-todo-list",
			},
			pageNode: &PageNode{
				Name:       "Index",
				Title:      "Index",
				Components: map[string]reflect.Method{"TodoList": {}},
			},
			expected: "TodoList",
		},
		{
			name: "v4 tag-only target matches component by tag name",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "form", // no id - matches Form component
			},
			pageNode: &PageNode{
				Name:       "Index",
				Title:      "Index",
				Components: map[string]reflect.Method{"Form": {}, "Page": {}},
			},
			expected: "Form",
		},
		{
			name: "v4 tag-only target without matching component returns functionRenderTarget",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "div",
			},
			pageNode: &PageNode{
				Name:       "Index",
				Title:      "Index",
				Components: map[string]reflect.Method{"Page": {}},
			},
			expectFunction: true,
		},
		{
			name: "v4 missing HX-Target falls back to Page",
			headers: map[string]string{
				"HX-Request": "true",
			},
			pageNode: &PageNode{
				Name:       "Index",
				Title:      "Index",
				Components: map[string]reflect.Method{"Page": {}},
			},
			expected: "Page",
		},
		{
			name: "v4 unknown id returns functionRenderTarget",
			headers: map[string]string{
				"HX-Request": "true",
				"HX-Target":  "span#nonexistent",
			},
			pageNode: &PageNode{
				Name:       "Index",
				Title:      "Index",
				Components: map[string]reflect.Method{"Page": {}},
			},
			expectFunction: true,
		},
		{
			name:    "non-HTMX request returns Page",
			headers: map[string]string{},
			pageNode: &PageNode{
				Name:       "Index",
				Title:      "Index",
				Components: map[string]reflect.Method{"Page": {}},
			},
			expected: "Page",
		},
		{
			name: "v4 HX-Request-Type=full overrides HX-Target",
			headers: map[string]string{
				"HX-Request":      "true",
				"HX-Request-Type": "full",
				"HX-Target":       "main#content",
			},
			pageNode: &PageNode{
				Name:       "Index",
				Title:      "Index",
				Components: map[string]reflect.Method{"Content": {}, "Page": {}},
			},
			expected: "Page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			target, err := HTMXv4RenderTarget(req, tt.pageNode)
			if err != nil {
				t.Fatalf("HTMXv4RenderTarget: %v", err)
			}
			if tt.expectFunction {
				if _, ok := target.(*functionRenderTarget); !ok {
					t.Errorf("expected functionRenderTarget, got %T", target)
				}
				return
			}
			mrt, ok := target.(*methodRenderTarget)
			if !ok {
				t.Fatalf("expected methodRenderTarget, got %T", target)
			}
			if mrt.name != tt.expected {
				t.Errorf("got %q, want %q", mrt.name, tt.expected)
			}
		})
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
