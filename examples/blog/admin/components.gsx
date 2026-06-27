// Admin-local gsx functions. StatsGrid, RecentPostsCard and PostsTable are
// standalone function components so the dashboard's Props+RenderTarget switch
// can refresh each widget independently with target.Is(StatsGrid) /
// target.Is(RecentPostsCard).
package admin

import (
	"github.com/jackielii/structpages/examples/blog/store"
	"github.com/jackielii/structpages/examples/blog/ui/components"
)

// StatCell — capitalized (gsx components must be Capitalized; the templ name
// was lowercase `statCell`).
component StatCell(label string, value int) {
	<div class="rounded-lg border bg-white p-4 text-center shadow-sm">
		<div class="text-3xl font-semibold text-slate-900">{ value }</div>
		<div class="mt-1 text-xs uppercase tracking-wide text-slate-500">
			{ label }
		</div>
	</div>
}

component StatsGrid(stats store.Stats) {
	<div id={StatsGrid |> id} class="grid grid-cols-2 gap-3 md:grid-cols-4">
		<StatCell label="Posts" value={stats.Posts}/>
		<StatCell label="Drafts" value={stats.Drafts}/>
		<StatCell label="Comments" value={stats.Comments}/>
		<StatCell label="Categories" value={stats.Categories}/>
	</div>
}

component RecentPostsCard(posts []store.Post) {
	<div id={RecentPostsCard |> id}>
		<components.Card title="Recent posts">
			<ul class="divide-y text-sm">
				{ if len(posts) == 0 {
					<li class="py-2 text-slate-500">No posts yet.</li>
				} }
				{ for _, p := range posts {
					<li class="flex items-center justify-between py-2">
						<a
							class="hover:underline"
							href={postEditPage{} |> url("id", p.ID)}
						>
							{ p.Title }
						</a>
						{ if p.Published {
							<span
								class="rounded bg-emerald-100 px-2 py-0.5 text-xs text-emerald-800"
							>
								live
							</span>
						} else {
							<span
								class="rounded bg-slate-200 px-2 py-0.5 text-xs text-slate-700"
							>
								draft
							</span>
						} }
					</li>
				} }
			</ul>
		</components.Card>
	</div>
}

component PostsTable(posts []store.Post) {
	<div
		id={PostsTable |> id}
		class="overflow-hidden rounded-lg border bg-white shadow-sm"
	>
		<table class="w-full text-sm">
			<thead
				class="bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500"
			>
				<tr>
					<th class="px-3 py-2">Title</th>
					<th class="px-3 py-2">Status</th>
					<th class="px-3 py-2">Created</th>
					<th class="px-3 py-2"></th>
				</tr>
			</thead>
			<tbody>
				{ for _, p := range posts {
					<tr class="border-t">
						<td class="px-3 py-2 font-medium">
							<a
								class="hover:underline"
								href={postEditPage{} |> url("id", p.ID)}
							>
								{ p.Title }
							</a>
						</td>
						<td class="px-3 py-2">
							{ if p.Published {
								<span
									class="rounded bg-emerald-100 px-2 py-0.5 text-xs text-emerald-800"
								>
									published
								</span>
							} else {
								<span
									class="rounded bg-slate-200 px-2 py-0.5 text-xs text-slate-700"
								>
									draft
								</span>
							} }
						</td>
						<td class="px-3 py-2 text-slate-500">
							{ p.CreatedAt.Format("Jan 2, 2006") }
						</td>
						<td class="px-3 py-2 text-right">
							<form
								method="POST"
								action={postDeleteHandler{} |> url("id", p.ID)}
								hx-post={postDeleteHandler{} |> url("id", p.ID)}
								hx-target={PostsTable |> target}
								hx-swap="outerHTML"
								hx-confirm="Delete this post?"
								class="inline"
							>
								<button
									class="text-xs text-red-600 hover:underline"
									type="submit"
								>
									Delete
								</button>
							</form>
						</td>
					</tr>
				} }
				{ if len(posts) == 0 {
					<tr>
						<td
							colspan="4"
							class="px-3 py-6 text-center text-slate-500"
						>
							No posts yet.
						</td>
					</tr>
				} }
			</tbody>
		</table>
	</div>
}
