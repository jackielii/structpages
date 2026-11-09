// Command fragment-guard shows a navigation guard keeping fragment URLs from
// being opened as bare markup. See docs/fragment-guard.md.
package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/jackielii/structpages"
)

func newMux(store *Store) (*http.ServeMux, error) {
	mux := http.NewServeMux()
	_, err := structpages.Mount(mux, pages{}, "/", "Home",
		structpages.WithTargetSelector(structpages.HTMXRenderTarget),
		structpages.WithErrorHandler(errorHandler),
		structpages.WithArgs(store),
		structpages.WithMiddlewares(NavigationGuard),
	)
	return mux, err
}

func main() {
	mux, err := newMux(NewStore())
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

// errorHandler answers a partial with a message to swap in, and anything
// else with a full page.
func errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, errNotFound) {
		status = http.StatusNotFound
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	if Partial(r) {
		_ = ErrorMessage(status).Render(r.Context(), w)
		return
	}
	_ = ErrorPage(status).Render(r.Context(), w)
}
