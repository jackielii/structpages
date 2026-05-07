// Package admin hosts the authenticated CMS: dashboard with multi-widget
// HTMX refresh, post and user CRUD, and a logout action. The Pages struct's
// Middlewares() method gates every descendant behind RequireAdmin.
//
// LoginPage lives outside Pages and is mounted by main as a sibling, so
// signed-out visitors can reach the form without tripping the gate.
package admin

import (
	"github.com/jackielii/structpages"
	"github.com/jackielii/structpages/examples/blog/auth"
)

// Pages mounts at /admin. Every child is protected by RequireAdmin.
type Pages struct {
	dashboard dashboardPage `route:"/{$} Dashboard"`
	posts     postsPages    `route:"/posts Posts"`
	users     usersPages    `route:"/users Users"`
	logout    logoutHandler `route:"POST /logout Logout"`
}

// Middlewares is invoked once at mount time. Its dependencies are filled
// from the same registry that powers Props and ServeHTTP.
func (Pages) Middlewares(a *auth.Service) []structpages.MiddlewareFunc {
	return []structpages.MiddlewareFunc{auth.RequireAdmin(a)}
}
