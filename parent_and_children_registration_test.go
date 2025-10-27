package structpages

import (
	"net/http"
	"testing"
)

// Test that pages with both handlers and children register the correct number of routes
func TestParentAndChildrenRegistration(t *testing.T) {
	var routeCount int

	mux := http.NewServeMux()
	sp, err := Mount(mux, testParentChildStructure{}, "/", "Test App", WithMiddlewares(printRoutes(&routeCount)))
	if err != nil {
		t.Fatalf("Failed to mount pages: %v", err)
	}
	_ = sp

	// Expected routes:
	// 1. /admin -> AdminSection (parent with handler)
	// 2. /admin/users -> UserManagement (child)
	// 3. /admin/settings -> SettingsPage (child)
	// 4. /{$} -> HomePage (simple page with no children)

	expectedRouteCount := 4
	if routeCount != expectedRouteCount {
		t.Errorf("Expected %d routes to be registered, but got %d", expectedRouteCount, routeCount)
	}
}

// Test structure: pages that have both handlers AND children
type testParentChildStructure struct {
	AdminSection adminSection `route:"/admin Admin"`
	HomePage     homePage     `route:"/{$} Home"`
}

// Admin section - has BOTH a handler AND children
type adminSection struct {
	UserManagement userManagement `route:"/users User Management"`
	SettingsPage   settingsPage   `route:"/settings Settings"`
}

func (adminSection) Page() testComponent {
	return testComponent{content: "admin-section"}
}

type userManagement struct{}

func (userManagement) Page() testComponent {
	return testComponent{content: "user-management"}
}

type settingsPage struct{}

func (settingsPage) Page() testComponent {
	return testComponent{content: "settings-page"}
}

// Home page - has handler but NO children
type homePage struct{}

func (homePage) Page() testComponent {
	return testComponent{content: "home-page"}
}
