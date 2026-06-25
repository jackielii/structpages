package blog

import (
	"net/http"
	"strconv"

	"github.com/jackielii/structpages/examples/blog/store"
	"github.com/jackielii/structpages/examples/blog/ui/components"
	"github.com/jackielii/structpages/examples/blog/ui/layout"
)

type categoryPage struct{}

type categoryProps struct {
	Category   store.Category
	Posts      []store.Post
	Pagination components.PageNav
}

func (categoryPage) Props(r *http.Request, s *store.Store) (categoryProps, error) {
	slug := r.PathValue("slug")
	cat, err := s.GetCategoryBySlug(slug)
	if err != nil {
		return categoryProps{}, err
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	posts, total := s.ListPosts(store.PostFilter{
		CategoryID: cat.ID,
		Page:       page,
		PageSize:   store.DefaultPageSize,
	})

	ctx := r.Context()
	return categoryProps{
		Category: cat,
		Posts:    posts,
		Pagination: components.PageNav{
			Page:     page,
			PageSize: store.DefaultPageSize,
			Total:    total,
			URL: func(target int) (string, error) {
				return components.URL(ctx,
					[]any{categoryPage{}, "?page={page}"},
					"page", target,
				)
			},
		},
	}, nil
}

component (p categoryPage) Page(props categoryProps) {
	<layout.PublicShell title={props.Category.Name}>
		<h1 class="mb-1 text-2xl font-semibold">{props.Category.Name}</h1>
		<p class="mb-6 text-sm text-slate-500">Posts filed under this category.</p>
		<div class="space-y-4">
			{ if len(props.Posts) == 0 {
				<p class="text-sm text-slate-500">Nothing here yet.</p>
			} }
			{ for _, post := range props.Posts {
				<PostCard p={post}/>
			} }
		</div>
		<div class="mt-6">
			{ components.Pagination(props.Pagination) }
		</div>
	</layout.PublicShell>
}
