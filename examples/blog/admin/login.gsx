package admin

import (
	"net/http"

	"github.com/jackielii/structpages"
	"github.com/jackielii/structpages/examples/blog/auth"
	"github.com/jackielii/structpages/examples/blog/ui/components"
)

// LoginPage is the sign-in document at GET /admin/login, and LoginAction
// handles the form's POST to the same path. They are mounted by main as
// siblings of admin.Pages, so RequireAdmin does not gate them.
//
// The split follows the route convention: a GET that shows something is a
// page (it has Page), and ServeHTTP only does things — here, sign in and
// redirect, or show the form again with the error.
type LoginPage struct{}

component (p LoginPage) Page() {
	<LoginShell username="" errMsg=""/>
}

type LoginAction struct{}

func (LoginAction) ServeHTTP(w http.ResponseWriter, r *http.Request, a *auth.Service) error {
	username := r.FormValue("username")
	if _, err := a.Login(w, username, r.FormValue("password")); err != nil {
		return structpages.RenderComponent(<LoginShell username={username} errMsg="Invalid username or password."/>)
	}
	dashboard, err := structpages.URLFor(r.Context(), structpages.Ref("dashboard"))
	if err != nil {
		return err
	}
	http.Redirect(w, r, dashboard, http.StatusSeeOther)
	return nil
}

// LoginShell is the whole sign-in document, rendered by LoginPage.Page and,
// with an error, by LoginAction. gsx requires
// component names to be Capitalized (lowercase = HTML element), so the templ
// `loginShell` becomes `LoginShell`.
component LoginShell(username, errMsg string) {
	<!DOCTYPE html>
	<html lang="en">
		<head>
			<meta charset="utf-8"/>
			<title>Sign in — blog admin</title>
			<script src="https://cdn.tailwindcss.com"></script>
		</head>
		<body class="bg-slate-100 text-slate-900">
			<main class="mx-auto max-w-sm px-4 py-16">
				<components.Card title="Sign in">
					<form method="POST" class="space-y-3">
						<components.Alert
							kind={components.AlertError}
							msg={errMsg}
						/>
						<label class="block text-sm">
							<span class="mb-1 block font-medium text-slate-700">
								Username
							</span>
							<input
								name="username"
								value={username}
								required
								autofocus
								class="w-full rounded border border-slate-300 px-2 py-1.5 text-sm"
							/>
						</label>
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
						<button
							type="submit"
							class="inline-flex w-full items-center justify-center rounded bg-slate-900 px-3 py-2 text-sm font-medium text-white hover:bg-slate-700"
						>
							Sign in
						</button>
						<p class="text-xs text-slate-500">
							Demo credentials: <code>admin</code> / <code>
								admin
							</code>
						</p>
					</form>
				</components.Card>
			</main>
		</body>
	</html>
}
