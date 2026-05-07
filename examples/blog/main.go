// Comprehensive blog example demonstrating the structpages "newest patterns"
// with a per-feature package layout (React-style modules).
//
// See README.md for the pattern catalog and the verification curl recipes.
package main

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/jackielii/structpages"
	"github.com/jackielii/structpages/examples/blog/admin"
	"github.com/jackielii/structpages/examples/blog/auth"
	blogpkg "github.com/jackielii/structpages/examples/blog/blog"
	"github.com/jackielii/structpages/examples/blog/store"
	"github.com/jackielii/structpages/examples/blog/ui/components"
)

// rootPages composes the feature packages. Each field's name becomes the
// structpages.Ref key for its subtree (so layout uses Ref("home"),
// Ref("loginPage"), Ref("dashboard") to link across packages without
// importing them).
//
// Note loginPage is a sibling of admin (not a child) — admin.Pages.Middlewares
// returns RequireAdmin, which would otherwise lock unauthenticated users
// out of the very form they need to sign in with.
type rootPages struct {
	blog      blogpkg.Pages   `route:"/ Blog"`
	loginPage admin.LoginPage `route:"/admin/login Admin Login"`
	admin     admin.Pages     `route:"/admin Admin"`
}

func main() {
	s := store.New()
	store.SeedDemo(s)
	authSvc := auth.New(s)

	mux := http.NewServeMux()
	if _, err := structpages.Mount(mux, &rootPages{}, "/", "structpages blog",
		structpages.WithArgs(s, authSvc),
		structpages.WithErrorHandler(errorHandler),
	); err != nil {
		log.Fatalf("mount: %v", err)
	}

	log.Println("listening on http://localhost:8080  (admin: admin/admin)")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

// errorHandler renders our own error component instead of net/http's plain
// 500 page. Recognises store.ErrNotFound as a 404; anything else is 500.
// HTMX requests get the inner block (so it slots into the current target);
// regular requests get a full document.
func errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	msg := err.Error()
	switch {
	case errors.Is(err, store.ErrNotFound):
		status = http.StatusNotFound
		msg = "We couldn't find what you were looking for."
	case errors.Is(err, store.ErrDuplicate):
		status = http.StatusConflict
	}
	if status >= 500 {
		log.Printf("server error: %v", err)
		msg = "Something went wrong on our end."
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if strings.EqualFold(r.Header.Get("HX-Request"), "true") {
		_ = components.ErrorBlock(status, msg).Render(r.Context(), w)
		return
	}
	_ = components.ErrorPage(status, msg).Render(r.Context(), w)
}
