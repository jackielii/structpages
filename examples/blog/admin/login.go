package admin

import (
	"net/http"

	"github.com/jackielii/structpages/examples/blog/auth"
)

// LoginPage handles both GET and POST at /admin/login. It is mounted as a
// sibling of admin.Pages (in main), so RequireAdmin does not gate it.
//
// Because it defines ServeHTTP, structpages routes everything to that
// method directly — there's no Props/Page split for this page.
type LoginPage struct{}

func (LoginPage) ServeHTTP(w http.ResponseWriter, r *http.Request, a *auth.Service) error {
	var (
		username string
		errMsg   string
	)
	if r.Method == http.MethodPost {
		username = r.FormValue("username")
		password := r.FormValue("password")
		if _, err := a.Login(w, username, password); err != nil {
			errMsg = "Invalid username or password."
		} else {
			http.Redirect(w, r, "/admin/", http.StatusSeeOther)
			return nil
		}
	}
	return loginShell(username, errMsg).Render(r.Context(), w)
}
