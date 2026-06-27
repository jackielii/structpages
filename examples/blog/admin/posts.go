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

// --- Handlers ---

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
		return structpages.RenderComponent(PostsTable(PostsTableProps{Posts: posts}))
	}
	http.Redirect(w, r, "/admin/posts/", http.StatusSeeOther)
	return nil
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
// update otherwise. Lives outside the gsx file so the markup stays declarative.
// gsx auto-sanitizes URL attributes, so a plain string is enough.
func postFormAction(ctx context.Context, p store.Post) string {
	if p.ID == 0 {
		return must(components.URL(ctx, postCreateHandler{}))
	}
	return must(components.URL(ctx, postUpdateHandler{}, "id", p.ID))
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
