package blog

import (
	"net/http"

	"github.com/jackielii/structpages/examples/blog/store"
	"github.com/jackielii/structpages/examples/blog/ui/components"
	"github.com/jackielii/structpages/examples/blog/ui/layout"
)

type homePage struct{}

type homeProps struct {
	Posts      []store.Post
	Categories []store.Category
}

// Props loads page data. Returns homeProps directly — gsx now emits method
// components as func (p homePage) Page(props homeProps) gsx.Node without a wrapper struct.
func (homePage) Props(_ *http.Request, s *store.Store) (homeProps, error) {
	posts, _ := s.ListPosts(store.PostFilter{PageSize: 5})
	return homeProps{Posts: posts, Categories: s.ListCategories()}, nil
}

// Page renders the full document. The former Content() method is inlined here:
// gsx's generated-props wrapping makes a separate Content(props) method
// undispatchable by structpages alongside Page(props) (both would need the same
// single Props-return type). See GAP notes.
component (p homePage) Page(props homeProps) {
	<layout.PublicShell title="Home">
		<h1 class="mb-4 text-2xl font-semibold">Recent Posts</h1>
		<div class="space-y-4">
			{ for _, post := range props.Posts {
				<PostCard p={post}/>
			} }
		</div>
		<components.Card title="Browse by category">
			<ul class="flex flex-wrap gap-2">
				{ for _, c := range props.Categories {
					<li>
						<a class="rounded-full border px-3 py-1 text-sm hover:bg-slate-50" href={ categoryPage{} |> url("slug", c.Slug) }>
							{c.Name}
						</a>
					</li>
				} }
			</ul>
		</components.Card>
	</layout.PublicShell>
}
