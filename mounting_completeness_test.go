package structpages

import (
	"fmt"
	"net/http"
	"testing"
)

// Reproduce the exact user issue: EventDeletePage has no handler and no children
type userReproduceIssue struct {
	AuthenticatedPages userAuthenticatedPages `route:"/"`
}

type userAuthenticatedPages struct {
	EventPages      userEventPages      `route:"/events "`
	SignoutHandler  userSignoutHandler  `route:"/signout Sign Out"`
	IndexPage       userIndexPage       `route:"/{$} Home"`
	AttachmentPages userAttachmentPages `route:"/attachments Attachments"`
	ActionPages     userActionPages     `route:"/actions Actions"`
	HistoryPages    userHistoryPages    `route:"/history History"`
}

type userEventPages struct {
	EventItemPage   userEventItemPage   `route:"/event/{id}  Event Details"`
	EventEditPage   userEventEditPage   `route:"/event/{id}/edit  Event Edit"`
	EventDeletePage userEventDeletePage `route:"/event/{id}  Event Delete"` // No handler, no children
}

func (userEventPages) Page() testComponent {
	return testComponent{content: "events"}
}

// Pages with handlers
type (
	userSignoutHandler  struct{}
	userIndexPage       struct{}
	userAttachmentPages struct{}
	userActionPages     struct{}
	userHistoryPages    struct{}
	userEventItemPage   struct{}
	userEventEditPage   struct{}
)

func (userSignoutHandler) Page() testComponent {
	return testComponent{content: "signout"}
}
func (userIndexPage) Page() testComponent { return testComponent{content: "home"} }
func (userAttachmentPages) Page() testComponent {
	return testComponent{content: "attachments"}
}

func (userActionPages) Page() testComponent {
	return testComponent{content: "actions"}
}

func (userHistoryPages) Page() testComponent {
	return testComponent{content: "history"}
}

func (userEventItemPage) Page() testComponent {
	return testComponent{content: "event-item"}
}

func (userEventEditPage) Page() testComponent {
	return testComponent{content: "event-edit"}
}

// This page type has NO Page() method and NO children - this causes the error
type userEventDeletePage struct{}

func printRoutes(counter *int) MiddlewareFunc {
	return func(h http.Handler, pn *PageNode) http.Handler {
		*counter++
		fmt.Printf("✓ Registered route #%d: %-30s -> %s\n", *counter, pn.FullRoute(), pn.Name)
		return h
	}
}

// Test that reproduces the exact user issue and verifies the fix
func TestUserIssueReproduction(t *testing.T) {
	mux := http.NewServeMux()
	router := NewRouter(mux)

	var routeCount int
	sp := New(WithMiddlewares(printRoutes(&routeCount)))

	err := sp.MountPages(router, userReproduceIssue{}, "/", "Test App")
	// After the fix, this should NOT fail - EventDeletePage should just be skipped
	if err != nil {
		t.Fatalf("Expected no error after fix, but got: %v", err)
	}

	// Assert that we have the expected number of routes registered
	// Expected routes:
	// 1. /events -> EventPages (parent with handler)
	// 2. /events/event/{id} -> EventItemPage
	// 3. /events/event/{id}/edit -> EventEditPage
	// 4. /signout -> SignoutHandler
	// 5. /{$} -> IndexPage
	// 6. /attachments -> AttachmentPages
	// 7. /actions -> ActionPages
	// 8. /history -> HistoryPages
	// Note: EventDeletePage is NOT registered because it has no handler and no children
	expectedRouteCount := 8
	if routeCount != expectedRouteCount {
		t.Errorf("Expected %d routes to be registered, but got %d", expectedRouteCount, routeCount)
	}

	t.Logf("Successfully registered %d routes", routeCount)
	t.Logf("✓ EventDeletePage was correctly skipped (no handler, no children)")
	t.Logf("✓ All other routes registered successfully despite EventDeletePage issue")

	// Test that all routes are properly registered
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"Events route", "/events", "events"},
		{"Event item route", "/events/event/123", "event-item"},
		{"Event edit route", "/events/event/123/edit", "event-edit"},
		{"Signout route", "/signout", "signout"},
		{"Root route", "/", "home"},
		{"Attachments route", "/attachments", "attachments"},
		{"Actions route", "/actions", "actions"},
		{"History route", "/history", "history"},
		// Note: EventDeletePage route should NOT be registered since it has no handler
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tt.path, http.NoBody)
			w := &responseRecorder{}

			router.ServeHTTP(w, req)

			if w.code != http.StatusOK {
				t.Errorf("Expected status 200, got %d for path %s", w.code, tt.path)
				return
			}

			if w.body != tt.expected {
				t.Errorf("Expected body %q, got %q for path %s", tt.expected, w.body, tt.path)
			}
		})
	}

	// Test that unregistered routes return 404
	t.Run("Unregistered route should return 404", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/events/nonexistent", http.NoBody)
		w := &responseRecorder{}

		router.ServeHTTP(w, req)

		if w.code != http.StatusNotFound {
			t.Errorf("Expected unregistered route to return 404, got %d", w.code)
		}
	})
}

// Simple response recorder for testing
type responseRecorder struct {
	code int
	body string
}

func (r *responseRecorder) Header() http.Header { return make(http.Header) }
func (r *responseRecorder) Write(data []byte) (int, error) {
	r.body = string(data)
	if r.code == 0 {
		r.code = 200
	}
	return len(data), nil
}
func (r *responseRecorder) WriteHeader(code int) { r.code = code }
