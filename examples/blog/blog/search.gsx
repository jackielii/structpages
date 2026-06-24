package blog

import (
	"net/http"

	"github.com/jackielii/structpages"
	"github.com/jackielii/structpages/examples/blog/store"
	"github.com/jackielii/structpages/examples/blog/ui/layout"
)

type searchPage struct{}

type searchProps struct {
	Query string
	Posts []store.Post
}

// Props uses RenderTarget to load less when HTMX only wants the results
// fragment. On the initial page load it returns the full searchProps; when the
// Results fragment is targeted it renders just that component via RenderComponent.
func (p searchPage) Props(r *http.Request, s *store.Store, target structpages.RenderTarget) (searchProps, error) {
	q := r.URL.Query().Get("q")
	sp := searchProps{Query: q}
	if q != "" {
		sp.Posts, _ = s.ListPosts(store.PostFilter{Search: q})
	}
	if target.Is(p.Results) {
		return searchProps{}, structpages.RenderComponent(p.Results(sp))
	}
	return sp, nil
}

component (p searchPage) Page(props searchProps) {
	<layout.PublicShell title="Search">
		<h1 class="mb-4 text-2xl font-semibold">Search</h1>
		<form
			class="mb-6"
			hx-get={ structpages.URLFor(ctx, searchPage{}) }
			hx-target={ structpages.IDTarget(ctx, p.Results) }
			hx-swap="outerHTML"
			hx-trigger="input changed delay:250ms from:input, submit"
			hx-push-url="true"
		>
			<input
				name="q"
				value={props.Query}
				placeholder="Search posts..."
				class="w-full rounded border border-slate-300 px-3 py-2 text-sm"
				autofocus
			/>
		</form>
		<p.Results { props... }/>
	</layout.PublicShell>
}

component (p searchPage) Results(props searchProps) {
	<div id={ structpages.ID(ctx, searchPage.Results) } class="space-y-4">
		{ if props.Query == "" {
			<p class="text-sm text-slate-500">Type a query to search.</p>
		} else if len(props.Posts) == 0 {
			<p class="text-sm text-slate-500">No results for "{props.Query}".</p>
		} else {
			<p class="text-xs text-slate-500">{resultsCount(len(props.Posts))}</p>
			{ for _, post := range props.Posts {
				<PostCard p={post}/>
			} }
		} }
	</div>
}
