package blog

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/jackielii/structpages"
	"github.com/jackielii/structpages/examples/blog/store"
)

// commentHandler illustrates the "ServeHTTP that writes, then re-renders a
// sibling component" pattern. For HTMX requests we return the refreshed
// CommentsList directly; for non-HTMX submissions we redirect back to the
// post so the full page reloads with the new comment in place.
type commentHandler struct{}

func (commentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, s *store.Store) error {
	slug := r.PathValue("slug")
	post, err := s.GetPostBySlug(slug)
	if err != nil {
		return err
	}
	author := strings.TrimSpace(r.FormValue("author"))
	body := strings.TrimSpace(r.FormValue("body"))
	if author == "" || body == "" {
		return fmt.Errorf("author and body are required")
	}
	if _, err := s.AddComment(post.ID, author, body); err != nil {
		return err
	}

	if r.Header.Get("HX-Request") == "true" {
		return structpages.RenderComponent(CommentsList(s.ListComments(post.ID)))
	}
	http.Redirect(w, r, "/posts/"+slug, http.StatusSeeOther)
	return nil
}

// resultsCount is a tiny helper used by search.templ.
func resultsCount(n int) string { return fmt.Sprintf("%d result%s", n, plural(n)) }

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
