package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackielii/structpages/examples/blog/auth"
	"github.com/jackielii/structpages/examples/blog/store"
	"github.com/jackielii/structpages/examples/blog/ui/components"
	"github.com/jackielii/structpages/examples/blog/ui/layout"
)

type usersPages struct {
	userList   userListPage      `route:"/{$} All Users"`
	userCreate userCreateHandler `route:"POST /{$} Create"`
	userDelete userDeleteHandler `route:"POST /{id}/delete Delete"`
}

type userListPage struct{}

type userListProps struct {
	User  store.User
	Users []store.User
}

func (userListPage) Props(r *http.Request, s *store.Store) (userListProps, error) {
	user, _ := auth.UserFromContext(r.Context())
	return userListProps{User: user, Users: s.ListUsers()}, nil
}

component (p userListPage) Page(props userListProps) {
	<layout.AdminShell title="Users" current={props.User}>
		<h1 class="mb-4 text-2xl font-semibold">Users</h1>
		<div class="grid gap-4 md:grid-cols-2">
			<components.Card title="Existing users">
				<ul class="divide-y text-sm">
					{ for _, u := range props.Users {
						<li class="flex items-center justify-between py-2">
							<span>
								{ u.Username }
								{ if u.IsAdmin {
									<span
										class="ml-2 rounded bg-slate-900 px-2 py-0.5 text-xs text-white"
									>
										admin
									</span>
								} }
							</span>
							<form
								method="POST"
								action={userDeleteHandler{} |> url("id", u.ID)}
								class="m-0"
							>
								<button
									class="text-xs text-red-600 hover:underline"
									type="submit"
								>
									Delete
								</button>
							</form>
						</li>
					} }
				</ul>
			</components.Card>
			<components.Card title="Create user">
				<form
					method="POST"
					action={userCreateHandler{} |> url}
					class="space-y-3"
				>
					<components.Input
						name="username"
						label="Username"
						value=""
						errMsg=""
					/>
					<label class="block text-sm">
						<span class="mb-1 block font-medium text-slate-700">
							Password
						</span>
						<input
							type="password"
							name="password"
							required
							class="w-full rounded border border-slate-300 px-2 py-1.5 text-sm"
						/>
					</label>
					<label class="flex items-center gap-2 text-sm">
						<input type="checkbox" name="is_admin"/>
						Grant admin
					</label>
					<components.Button label="Create" type="submit"/>
				</form>
			</components.Card>
		</div>
	</layout.AdminShell>
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
