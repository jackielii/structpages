// Feature-local gsx components for the public blog. These are standalone
// functions (not page methods) so other handlers in this package can target
// them via RenderComponent — most notably the comment handler, which
// re-renders CommentsList after a successful POST.
package blog

import (
	"github.com/jackielii/structpages/examples/blog/store"
)

component PostMeta(p store.Post) {
	<p class="text-xs text-slate-500">
		Posted { p.CreatedAt.Format("Jan 2, 2006") }
	</p>
}

component PostCard(p store.Post) {
	<article class="rounded-lg border bg-white p-5 shadow-sm">
		<a
			class="text-lg font-semibold text-slate-900 hover:underline"
			href={postPage{} |> url("slug", p.Slug)}
		>
			{ p.Title }
		</a>
		<PostMeta p={p}/>
		<p class="mt-2 line-clamp-2 text-sm text-slate-700">{ p.Body }</p>
	</article>
}

// CommentsList is a standalone function component so the commentHandler
// can re-render it from ServeHTTP via RenderComponent(CommentsList(...)).
// It owns its own wrapper id, which doubles as the HTMX hx-target.
component CommentsList(comments []store.Comment) {
	<div id={CommentsList |> id} class="space-y-3">
		{ if len(comments) == 0 {
			<p class="text-sm text-slate-500">
				No comments yet — be the first.
			</p>
		} }
		{ for _, c := range comments {
			<article class="rounded border bg-slate-50 p-3">
				<header class="text-xs font-medium text-slate-700">
					{ c.Author } · { c.CreatedAt.Format("15:04 Jan 2") }
				</header>
				<p class="mt-1 text-sm text-slate-800">{ c.Body }</p>
			</article>
		} }
		<p class="text-xs text-slate-400">Total: { len(comments) }</p>
	</div>
}
