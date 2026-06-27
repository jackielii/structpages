package admin

import (
	"net/http"

	"github.com/jackielii/structpages"
	"github.com/jackielii/structpages/examples/blog/auth"
	"github.com/jackielii/structpages/examples/blog/store"
	"github.com/jackielii/structpages/examples/blog/ui/components"
	"github.com/jackielii/structpages/examples/blog/ui/layout"
)

type dashboardPage struct{}

type dashboardProps struct {
	User        store.User
	Stats       store.Stats
	RecentPosts []store.Post
}

// Props demonstrates the Props + RenderTarget pattern. Each widget is a
// standalone gsx function (StatsGrid, RecentPostsCard) so HTMX refresh
// requests with HX-Target: #stats-grid or #recent-posts-card resolve here
// and only the touched data is loaded — no full page work.
//
// Returns dashboardProps directly — gsx now emits method components as
// func (p dashboardPage) Page(props dashboardProps) gsx.Node without a wrapper struct.
func (p dashboardPage) Props(r *http.Request, s *store.Store, target structpages.RenderTarget) (dashboardProps, error) {
	switch {
	case target.Is(StatsGrid):
		return dashboardProps{}, structpages.RenderComponent(StatsGrid(StatsGridProps{Stats: s.Stats()}))

	case target.Is(RecentPostsCard):
		posts, _ := s.ListPosts(store.PostFilter{IncludeDraft: true, PageSize: 5})
		return dashboardProps{}, structpages.RenderComponent(RecentPostsCard(RecentPostsCardProps{Posts: posts}))
	}

	user, _ := auth.UserFromContext(r.Context())
	posts, _ := s.ListPosts(store.PostFilter{IncludeDraft: true, PageSize: 5})
	return dashboardProps{
		User:        user,
		Stats:       s.Stats(),
		RecentPosts: posts,
	}, nil
}

component (p dashboardPage) Page(props dashboardProps) {
	<layout.AdminShell title="Dashboard" current={props.User}>
		<header class="mb-6 flex items-end justify-between">
			<h1 class="text-2xl font-semibold">Dashboard</h1>
			<div class="flex gap-2 text-xs">
				<button
					class="rounded border px-2 py-1 hover:bg-white"
					hx-get={dashboardPage{} |> url}
					hx-target={StatsGrid |> target}
					hx-swap="outerHTML"
				>
					↻ Stats
				</button>
				<button
					class="rounded border px-2 py-1 hover:bg-white"
					hx-get={dashboardPage{} |> url}
					hx-target={RecentPostsCard |> target}
					hx-swap="outerHTML"
				>
					↻ Recent posts
				</button>
			</div>
		</header>
		<section class="space-y-6">
			<StatsGrid stats={props.Stats}/>
			<RecentPostsCard posts={props.RecentPosts}/>
			<components.Card title="Try it">
				<ul class="list-disc space-y-1 pl-5 text-sm text-slate-700">
					<li>
						Click ↻ Stats — only the StatsGrid widget refreshes (check Network tab).
					</li>
					<li>
						Click ↻ Recent posts — only that card reloads, with its own DB query.
					</li>
					<li>
						Hard refresh — the full document re-renders via Page().
					</li>
				</ul>
			</components.Card>
		</section>
	</layout.AdminShell>
}
