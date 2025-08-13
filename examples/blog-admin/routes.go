package main

import (
	"net/http"

	"github.com/jackielii/structpages"
)

// Root pages structure demonstrating nested routing
type pages struct {
	// Public pages
	homePage     `route:"/{$} Home"`
	postPage     `route:"/posts/{slug} "`
	categoryPage `route:"/category/{slug} "`
	searchPage   `route:"/search Search"`

	// Auth pages
	loginPage  `route:"/login Login"`
	logoutPage `route:"POST /logout"`

	// Admin section with nested routes
	admin adminPages `route:"/admin Admin"`

	// API endpoints
	api apiPages `route:"/api"`
}

// Admin pages demonstrating nested structure
type adminPages struct {
	dashboard    `route:"/{$} Dashboard"`
	posts        adminPostPages    `route:"/posts Posts"`
	users        adminUserPages    `route:"/users Users"`
	settings     adminSettingsPage `route:"/settings Settings"`
	mediaLibrary mediaLibraryPage  `route:"/media Media Library"`
}

// Admin post management pages
type adminPostPages struct {
	list   adminPostListPage   `route:"/{$} All Posts"`
	new    adminPostNewPage    `route:"/new New Post"`
	edit   adminPostEditPage   `route:"/{id}/edit"`
	delete adminPostDeletePage `route:"POST /{id}/delete"`
}

// Admin user management pages
type adminUserPages struct {
	list   adminUserListPage   `route:"/{$} All Users"`
	new    adminUserNewPage    `route:"/new New User"`
	edit   adminUserEditPage   `route:"/{id}/edit"`
	delete adminUserDeletePage `route:"POST /{id}/delete"`
}

// API pages for programmatic access
type apiPages struct {
	posts apiPostPages  `route:"/posts"`
	media apiMediaPages `route:"/media"`
}

type apiPostPages struct {
	publish   apiPostPublishPage   `route:"POST /{id}/publish"`
	unpublish apiPostUnpublishPage `route:"POST /{id}/unpublish"`
	autosave  apiPostAutosavePage  `route:"POST /{id}/autosave"`
}

type apiMediaPages struct {
	upload apiMediaUploadPage `route:"POST /upload"`
}

// Middleware implementations

// adminPages implements middleware for authentication
func (a adminPages) Middlewares(auth *AuthService) []structpages.MiddlewareFunc {
	return []structpages.MiddlewareFunc{
		requireAuthMiddleware(auth),
		requireAdminMiddleware(auth),
	}
}

// adminPostPages can have additional middleware
func (p adminPostPages) Middlewares() []structpages.MiddlewareFunc {
	return []structpages.MiddlewareFunc{
		csrfProtectionMiddleware,
	}
}

// Helper middleware functions
func requireAuthMiddleware(auth *AuthService) structpages.MiddlewareFunc {
	return func(next http.Handler, pn *structpages.PageNode) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := auth.GetUser(r)
			if user == nil {
				http.Redirect(w, r, "/login?redirect="+r.URL.Path, http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requireAdminMiddleware(auth *AuthService) structpages.MiddlewareFunc {
	return func(next http.Handler, pn *structpages.PageNode) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := auth.GetUser(r)
			if user == nil || user.Role != "admin" {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func csrfProtectionMiddleware(next http.Handler, pn *structpages.PageNode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simplified CSRF check - in production use a proper CSRF library
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" {
			token := r.FormValue("csrf_token")
			if token == "" {
				token = r.Header.Get("X-CSRF-Token")
			}
			// Validate token here
		}
		next.ServeHTTP(w, r)
	})
}
