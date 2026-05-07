package admin

import (
	"fmt"
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
		return fmt.Errorf("username and password are required")
	}
	if _, err := s.CreateUser(store.User{Username: username, Password: password, IsAdmin: isAdmin}); err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	http.Redirect(w, r, "/admin/users/", http.StatusSeeOther)
	return nil
}

type userDeleteHandler struct{}

func (userDeleteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, s *store.Store) error {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}
	if err := s.DeleteUser(id); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	http.Redirect(w, r, "/admin/users/", http.StatusSeeOther)
	return nil
}
