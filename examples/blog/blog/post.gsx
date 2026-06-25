package blog

import (
	"net/http"

	"github.com/gsxhq/gsx"
	"github.com/jackielii/structpages/examples/blog/store"
	"github.com/jackielii/structpages/examples/blog/ui/components"
	"github.com/jackielii/structpages/examples/blog/ui/layout"
)

type postPage struct{}

type postProps struct {
	Post     store.Post
	Author   store.User
	Category store.Category
	Comments []store.Comment
}

func (postPage) Props(r *http.Request, s *store.Store) (postProps, error) {
	slug := r.PathValue("slug")
	p, err := s.GetPostBySlug(slug)
	if err != nil {
		return postProps{}, err
	}
	author, _ := s.GetUser(p.AuthorID)
	category, _ := s.GetCategory(p.CategoryID)
	return postProps{
		Post:     p,
		Author:   author,
		Category: category,
		Comments: s.ListComments(p.ID),
	}, nil
}

component (p postPage) Page(props postProps) {
	<layout.PublicShell title={props.Post.Title}>
		<article class="space-y-3">
			<h1 class="text-2xl font-semibold">{props.Post.Title}</h1>
			<p class="text-sm text-slate-500">
				by {props.Author.Username}
				{ if props.Category.Slug != "" {
					· <a class="hover:underline" href={ categoryPage{} |> url("slug", props.Category.Slug) }>{props.Category.Name}</a>
				} }
			</p>
			<div class="prose max-w-none text-slate-800">
				<p>{props.Post.Body}</p>
			</div>
		</article>
		<section class="mt-10 space-y-4">
			<h2 class="text-lg font-semibold">Comments</h2>
			<CommentsList comments={props.Comments}/>
			<form
				class="space-y-2 rounded border bg-white p-4"
				hx-post={ commentHandler{} |> url("slug", props.Post.Slug) }
				hx-target={ CommentsList |> target }
				hx-swap="outerHTML"
				hx-on:htmx:after-request="this.reset()"
			>
				<h3 class="text-sm font-semibold">Add a comment</h3>
				<components.Input name="author" label="Name" value="" errMsg=""/>
				<components.Textarea name="body" label="Comment" value="" errMsg=""/>
				<components.Button label="Post comment" { gsx.Attrs{"type": "submit"}... }/>
			</form>
		</section>
	</layout.PublicShell>
}
