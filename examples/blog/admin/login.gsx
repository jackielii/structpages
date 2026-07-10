package admin

import (
	"net/http"

	"github.com/jackielii/structpages"
	"github.com/jackielii/structpages/examples/blog/auth"
	"github.com/jackielii/structpages/examples/blog/ui/components"
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
	return structpages.RenderComponent(<LoginShell username={username} errMsg={errMsg}/>)
}

// LoginShell is rendered by LoginPage.ServeHTTP above. gsx requires
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
