package admin

import (
	"net/http"

	"github.com/jackielii/structpages"
	"github.com/jackielii/structpages/examples/blog/auth"
)

type logoutHandler struct{}

func (logoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, a *auth.Service) error {
	a.Logout(w, r)
	login, err := structpages.URLFor(r.Context(), LoginPage{})
	if err != nil {
		return err
	}
	http.Redirect(w, r, login, http.StatusSeeOther)
	return nil
}
