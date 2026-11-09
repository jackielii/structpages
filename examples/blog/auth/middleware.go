package auth

import (
	"context"
	"net/http"

	"github.com/jackielii/structpages"
	"github.com/jackielii/structpages/examples/blog/store"
)

type ctxUserKey struct{}

// UserFromContext returns the user RequireAdmin attached to the request, if any.
func UserFromContext(ctx context.Context) (store.User, bool) {
	u, ok := ctx.Value(ctxUserKey{}).(store.User)
	return u, ok
}

// RequireAdmin gates descendant pages. It redirects unauthenticated visitors
// to the login page and forbids non-admin users. The authenticated user is
// stashed in the request context so admin pages can read it without re-querying.
func RequireAdmin(a *Service) structpages.MiddlewareFunc {
	return func(next http.Handler, _ *structpages.PageNode) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := a.Current(r)
			if !ok {
				// Ref, not LoginPage{}: admin imports auth, so auth can't import admin.
				login, err := structpages.URLFor(r.Context(), structpages.Ref("loginPage"))
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				http.Redirect(w, r, login, http.StatusSeeOther)
				return
			}
			if !u.IsAdmin {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), ctxUserKey{}, u))
			next.ServeHTTP(w, r)
		})
	}
}
