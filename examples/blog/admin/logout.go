package admin

import (
	"net/http"

	"github.com/jackielii/structpages/examples/blog/auth"
)

type logoutHandler struct{}

func (logoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, a *auth.Service) error {
	a.Logout(w, r)
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
	return nil
}
