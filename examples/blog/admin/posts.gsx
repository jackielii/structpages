package admin

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackielii/structpages"
	"github.com/jackielii/structpages/examples/blog/auth"
	"github.com/jackielii/structpages/examples/blog/store"
	"github.com/jackielii/structpages/examples/blog/ui/components"
	"github.com/jackielii/structpages/examples/blog/ui/layout"
)

// postsPages mounts at /admin/posts. Three GET pages, three POST handlers,
// one route per CRUD verb. The router disambiguates by HTTP method, so list
// (GET /{$}) and create (POST /{$}) coexist on the same path.
// Each non-list route includes a verb in the path so Go's mux can disambiguate
// POSTs without a wildcard catching literal segments like "new".
type postsPages struct {
	postList   postListPage      `route:"GET /{$} All Posts"`
	postNew    postNewPage       `route:"GET /new New Post"`
	postCreate postCreateHandler `route:"POST /create Create"`
	postEdit   postEditPage      `route:"GET /{id}/edit Edit"`
	postUpdate postUpdateHandler `route:"POST /{id}/update Update"`
	postDelete postDeleteHandler `route:"POST /{id}/delete Delete"`
}

// AdminShellWith is a tiny gsx wrapper used by handlers that need to render a
// custom body inside AdminShell from Go code. The body is passed as the
// implicit Children prop — from Go: AdminShellWith(AdminShellWithProps{Title: …,
// User: …, Children: body}).
component AdminShellWith(title string, user store.User) {
	<layout.AdminShell title={title} current={user}>
		{ children }
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
			<a
				class="rounded bg-slate-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-slate-700"
				href={postNewPage{} |> url}
			>
				New post
			</a>
		</header>
		<PostsTable posts={props.Posts}/>
	</layout.AdminShell>
}

type postDeleteHandler struct{}

// Delete supports both styles: the PostsTable form falls back to a full POST
// (visible without HTMX) but also sends hx-post for live refresh of the table.
func (postDeleteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, s *store.Store) error {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		return fmt.Errorf("invalid post id: %w", err)
	}
	if err := s.DeletePost(id); err != nil {
		return fmt.Errorf("delete post: %w", err)
	}
	if r.Header.Get("HX-Request") == "true" {
		posts, _ := s.ListPosts(store.PostFilter{IncludeDraft: true, PageSize: 50})
		return structpages.RenderComponent(<PostsTable posts={posts}/>)
	}
	http.Redirect(w, r, "/admin/posts/", http.StatusSeeOther)
	return nil
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

type postCreateHandler struct{}

func (postCreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, s *store.Store) error {
	user, _ := auth.UserFromContext(r.Context())
	p, errMsg := parsePostForm(r)
	if errMsg != "" {
		return renderPostForm(r.Context(), w, user, "New post", p, s.ListCategories(), errMsg)
	}
	p.AuthorID = user.ID
	if _, err := s.CreatePost(p); err != nil {
		return renderPostForm(r.Context(), w, user, "New post", p, s.ListCategories(), err.Error())
	}
	http.Redirect(w, r, "/admin/posts/", http.StatusSeeOther)
	return nil
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

type postUpdateHandler struct{}

func (postUpdateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, s *store.Store) error {
	user, _ := auth.UserFromContext(r.Context())
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		return fmt.Errorf("invalid post id: %w", err)
	}
	incoming, errMsg := parsePostForm(r)
	incoming.ID = id
	if errMsg != "" {
		return renderPostForm(r.Context(), w, user, "Edit post", incoming, s.ListCategories(), errMsg)
	}
	if _, err := s.UpdatePost(id, func(p *store.Post) {
		p.Title = incoming.Title
		p.Body = incoming.Body
		p.CategoryID = incoming.CategoryID
		p.Published = incoming.Published
		if incoming.Slug != "" {
			p.Slug = incoming.Slug
		}
	}); err != nil {
		return err
	}
	http.Redirect(w, r, "/admin/posts/", http.StatusSeeOther)
	return nil
}

// --- Shared form ---

component PostForm(p store.Post, cats []store.Category, errMsg string) {
	<form method="POST" action={postFormAction(ctx, p)} class="space-y-3">
		<components.Alert kind={components.AlertError} msg={errMsg}/>
		<components.Input name="title" label="Title" value={p.Title} errMsg=""/>
		<components.Input
			name="slug"
			label="Slug (auto if blank)"
			value={p.Slug}
			errMsg=""
		/>
		<label class="block text-sm">
			<span class="mb-1 block font-medium text-slate-700">Category</span>
			<select
				name="category_id"
				class="w-full rounded border border-slate-300 px-2 py-1.5 text-sm"
			>
				<option value="0">— pick one —</option>
				{ for _, c := range cats {
					<option value={c.ID} selected={c.ID == p.CategoryID}>
						{ c.Name }
					</option>
				} }
			</select>
		</label>
		<components.Textarea name="body" label="Body" value={p.Body} errMsg=""/>
		<label class="flex items-center gap-2 text-sm">
			<input type="checkbox" name="published" checked={p.Published}/>
			Publish immediately
		</label>
		<div class="flex items-center gap-2">
			<components.Button label="Save" type="submit"/>
			<a
				class="text-sm text-slate-500 hover:underline"
				href={postListPage{} |> url}
			>
				Cancel
			</a>
		</div>
	</form>
}

// --- Helpers ---

func parsePostForm(r *http.Request) (store.Post, string) {
	catID, _ := strconv.Atoi(r.FormValue("category_id"))
	p := store.Post{
		Title:      strings.TrimSpace(r.FormValue("title")),
		Slug:       strings.TrimSpace(r.FormValue("slug")),
		Body:       strings.TrimSpace(r.FormValue("body")),
		CategoryID: catID,
		Published:  r.FormValue("published") == "on",
	}
	switch {
	case p.Title == "":
		return p, "Title is required."
	case p.Body == "":
		return p, "Body is required."
	case p.CategoryID == 0:
		return p, "Pick a category."
	}
	return p, ""
}

// renderPostForm re-renders the form on validation failure, preserving inputs.
func renderPostForm(ctx context.Context, w http.ResponseWriter, user store.User, title string, p store.Post, cats []store.Category, errMsg string) error {
	body := PostForm(PostFormProps{P: p, Cats: cats, ErrMsg: errMsg})
	return AdminShellWith(AdminShellWithProps{Title: title, User: user, Children: body}).Render(ctx, w)
}

// postFormAction returns the POST URL for the form: create when ID==0,
// update otherwise. Lives in Go code so the markup stays declarative; the
// attribute hole auto-unwraps the (string, error) pair and any error
// propagates through the render instead of being swallowed.
func postFormAction(ctx context.Context, p store.Post) (string, error) {
	if p.ID == 0 {
		return components.URL(ctx, postCreateHandler{})
	}
	return components.URL(ctx, postUpdateHandler{}, "id", p.ID)
}
