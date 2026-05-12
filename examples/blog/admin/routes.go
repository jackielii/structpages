// Package admin hosts the authenticated CMS: dashboard with multi-widget
// HTMX refresh, post and user CRUD, and a logout action. The Pages struct's
// Middlewares() method gates every descendant behind RequireAdmin.
//
// LoginPage lives outside Pages and is mounted by main as a sibling, so
// signed-out visitors can reach the form without tripping the gate.
package admin

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/jackielii/structpages"
	"github.com/jackielii/structpages/examples/blog/auth"
)

// Pages mounts at /admin. Every child is protected by RequireAdmin.
//
// Assets is the module's own /static/ subtree (admin-only CSS, images,
// JS). It lives in the same struct as the page routes, so /admin and
// /admin/static/* are wired up together — no separate pub.Handle call in
// main, and renaming Pages' mount path moves the assets with it.
type Pages struct {
	dashboard dashboardPage `route:"/{$} Dashboard"`
	posts     postsPages    `route:"/posts Posts"`
	users     usersPages    `route:"/users Users"`
	logout    logoutHandler `route:"POST /logout Logout"`
	Assets    staticFiles   `route:"GET /static/{path...} Assets"`
}

// Middlewares is invoked once at mount time and applies to every child
// of Pages — including Assets — so /admin/static/* is gated behind
// RequireAdmin. The login page renders with its own loginShell (no
// references to /admin/static/), so unauthenticated visitors never need
// the assets.
func (Pages) Middlewares(a *auth.Service) []structpages.MiddlewareFunc {
	return []structpages.MiddlewareFunc{auth.RequireAdmin(a)}
}

//go:embed all:static
var staticFS embed.FS

var staticRoot = func() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err) // unreachable: directory is //go:embed'd above
	}
	return sub
}()

// staticFiles serves the embedded admin/static/ tree.
//
// The {path...} wildcard in the route tag captures everything after
// /admin/static/, so r.PathValue("path") is the file path inside the
// embedded FS — no http.StripPrefix needed, and the handler never has
// to know that it's mounted under /admin. Re-mount Pages somewhere
// else and this code keeps working unchanged.
type staticFiles struct{}

func (staticFiles) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, staticRoot, r.PathValue("path"))
}
