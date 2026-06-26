// Package layout exports the shared HTML shells used by every page.
// Feature packages call PublicShell or AdminShell with {children}
// instead of writing their own <html> document.
package layout

import (
	"github.com/jackielii/structpages"
	"github.com/jackielii/structpages/examples/blog/store"
)

// PublicShell wraps reader-facing pages. Cross-feature links (e.g. the admin
// link) use structpages.Ref so this package never imports admin or blog —
// keeping the dependency graph one-way (features → ui).
component PublicShell(title string) {
	<!DOCTYPE html>
	<html lang="en">
		<head>
			<meta charset="utf-8"/>
			<meta
				name="viewport"
				content="width=device-width, initial-scale=1"
			/>
			<title>{ title } — structpages blog</title>
			<script src="https://cdn.tailwindcss.com"></script>
			<script src="https://unpkg.com/htmx.org@2.0.4"></script>
		</head>
		<body class="bg-slate-50 text-slate-900">
			<header class="border-b bg-white">
				<div
					class="mx-auto flex max-w-3xl items-center justify-between px-4 py-3"
				>
					<a
						class="text-lg font-semibold"
						href={structpages.Ref("home") |> url}
					>
						structpages blog
					</a>
					<nav class="flex gap-4 text-sm">
						<a
							class="hover:underline"
							href={structpages.Ref("home") |> url}
						>
							Home
						</a>
						<a
							class="hover:underline"
							href={structpages.Ref("search") |> url}
						>
							Search
						</a>
						<a
							class="text-slate-500 hover:text-slate-900"
							href={structpages.Ref("loginPage") |> url}
						>
							Admin
						</a>
					</nav>
				</div>
			</header>
			<main id="content" class="mx-auto max-w-3xl px-4 py-8">
				{ children }
			</main>
		</body>
	</html>
}

// AdminShell wraps the authenticated admin app.
component AdminShell(title string, current store.User) {
	<!DOCTYPE html>
	<html lang="en">
		<head>
			<meta charset="utf-8"/>
			<meta
				name="viewport"
				content="width=device-width, initial-scale=1"
			/>
			<title>Admin — { title }</title>
			<script src="https://cdn.tailwindcss.com"></script>
			<script src="https://unpkg.com/htmx.org@2.0.4"></script>
		</head>
		<body class="bg-slate-100 text-slate-900">
			<header class="border-b bg-slate-900 text-slate-100">
				<div
					class="mx-auto flex max-w-5xl items-center justify-between px-4 py-3"
				>
					<a
						class="flex items-center gap-2 text-lg font-semibold"
						href={structpages.Ref("dashboard") |> url}
					>
						<img
							src="/admin/static/admin-logo.svg"
							alt=""
							class="h-5 w-5"
						/>
						blog admin
					</a>
					<nav class="flex items-center gap-4 text-sm">
						<a
							class="hover:underline"
							href={structpages.Ref("dashboard") |> url}
						>
							Dashboard
						</a>
						<a
							class="hover:underline"
							href={structpages.Ref("postList") |> url}
						>
							Posts
						</a>
						<a
							class="hover:underline"
							href={structpages.Ref("userList") |> url}
						>
							Users
						</a>
						<span class="text-slate-400">|</span>
						<span class="text-slate-300">{ current.Username }</span>
						<form
							method="POST"
							action={structpages.Ref("logout") |> url}
							class="m-0"
						>
							<button
								class="text-slate-300 hover:text-white"
								type="submit"
							>
								Sign out
							</button>
						</form>
					</nav>
				</div>
			</header>
			<main id="content" class="mx-auto max-w-5xl px-4 py-8">
				{ children }
			</main>
		</body>
	</html>
}
