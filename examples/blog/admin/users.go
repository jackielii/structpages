package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/jackielii/structpages/examples/blog/store"
)

type usersPages struct {
	userList   userListPage      `route:"/{$} All Users"`
	userCreate userCreateHandler `route:"POST /{$} Create"`
	userDelete userDeleteHandler `route:"POST /{id}/delete Delete"`
}

type userCreateHandler struct{}

func (userCreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, s *store.Store) error {
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	isAdmin := r.FormValue("is_admin") == "on"
	if username == "" || password == "" {
		http.Redirect(w, r, "/admin/users/", http.StatusSeeOther)
		return nil
	}
	_, _ = s.CreateUser(store.User{Username: username, Password: password, IsAdmin: isAdmin})
	http.Redirect(w, r, "/admin/users/", http.StatusSeeOther)
	return nil
}

type userDeleteHandler struct{}

func (userDeleteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, s *store.Store) error {
	id, _ := strconv.Atoi(r.PathValue("id"))
	_ = s.DeleteUser(id)
	http.Redirect(w, r, "/admin/users/", http.StatusSeeOther)
	return nil
}
