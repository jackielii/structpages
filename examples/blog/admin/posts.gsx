package admin

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gsxhq/gsx"
	"github.com/jackielii/structpages/examples/blog/auth"
	"github.com/jackielii/structpages/examples/blog/store"
	"github.com/jackielii/structpages/examples/blog/ui/components"
	"github.com/jackielii/structpages/examples/blog/ui/layout"
)

// AdminShellWith is a tiny gsx wrapper used by handlers that need to render a
// custom body inside AdminShell from Go code. The body is passed as the
// implicit Children prop — from Go: AdminShellWith(AdminShellWithProps{Title: …,
// User: …, Children: body}).
component AdminShellWith(title string, user store.User) {
	<layout.AdminShell title={title} current={user}>
		{children}
	</layout.AdminShell>
}

// --- List ---

type postListPage struct{}

type postListProps struct {
	User  store.User
	Posts []store.Post
}

func (postListPage) Props(r *http.Request, s *store.Store) (postListProps, error) {
	user, _ := auth.UserFromContext(r.Context())
	posts, _ := s.ListPosts(store.PostFilter{IncludeDraft: true, PageSize: 50})
	return postListProps{User: user, Posts: posts}, nil
}

component (p postListPage) Page(props postListProps) {
	<layout.AdminShell title="Posts" current={props.User}>
		<header class="mb-4 flex items-center justify-between">
			<h1 class="text-2xl font-semibold">All posts</h1>
			<a class="rounded bg-slate-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-slate-700" href={ postNewPage{} |> url }>New post</a>
		</header>
		<PostsTable posts={props.Posts}/>
	</layout.AdminShell>
}

// --- New ---

type postNewPage struct{}

type postFormViewProps struct {
	User       store.User
	Categories []store.Category
	Post       store.Post
}

func (postNewPage) Props(r *http.Request, s *store.Store) (postFormViewProps, error) {
	user, _ := auth.UserFromContext(r.Context())
	return postFormViewProps{User: user, Categories: s.ListCategories()}, nil
}

component (p postNewPage) Page(props postFormViewProps) {
	<layout.AdminShell title="New post" current={props.User}>
		<h1 class="mb-4 text-2xl font-semibold">New post</h1>
		<PostForm p={props.Post} cats={props.Categories} errMsg=""/>
	</layout.AdminShell>
}

// --- Edit ---

type postEditPage struct{}

func (postEditPage) Props(r *http.Request, s *store.Store) (postFormViewProps, error) {
	user, _ := auth.UserFromContext(r.Context())
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		return postFormViewProps{}, fmt.Errorf("invalid post id: %w", err)
	}
	p, err := s.GetPost(id)
	if err != nil {
		return postFormViewProps{}, err
	}
	return postFormViewProps{User: user, Categories: s.ListCategories(), Post: p}, nil
}

component (p postEditPage) Page(props postFormViewProps) {
	<layout.AdminShell title="Edit post" current={props.User}>
		<h1 class="mb-4 text-2xl font-semibold">Edit post</h1>
		<PostForm p={props.Post} cats={props.Categories} errMsg=""/>
	</layout.AdminShell>
}

// --- Shared form ---

component PostForm(p store.Post, cats []store.Category, errMsg string) {
	<form method="POST" action={ postFormAction(ctx, p) } class="space-y-3">
		<components.Alert kind={components.AlertError} msg={errMsg}/>
		<components.Input name="title" label="Title" value={p.Title} errMsg=""/>
		<components.Input name="slug" label="Slug (auto if blank)" value={p.Slug} errMsg=""/>
		<label class="block text-sm">
			<span class="mb-1 block font-medium text-slate-700">Category</span>
			<select name="category_id" class="w-full rounded border border-slate-300 px-2 py-1.5 text-sm">
				<option value="0">— pick one —</option>
				{ for _, c := range cats {
					<option value={c.ID} selected={ c.ID == p.CategoryID }>{c.Name}</option>
				} }
			</select>
		</label>
		<components.Textarea name="body" label="Body" value={p.Body} errMsg=""/>
		<label class="flex items-center gap-2 text-sm">
			<input type="checkbox" name="published" checked={p.Published}/>
			Publish immediately
		</label>
		<div class="flex items-center gap-2">
			<components.Button label="Save" { gsx.Attrs{"type": "submit"}... }/>
			<a class="text-sm text-slate-500 hover:underline" href={ postListPage{} |> url }>Cancel</a>
		</div>
	</form>
}
