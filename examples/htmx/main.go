package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/jackielii/structpages"
)

// PrintRoutes is a simple middleware that prints all routes to stdout
func PrintRoutes(sb *strings.Builder) structpages.MiddlewareFunc {
	fmt.Fprintln(sb, "\nRoutes:")
	fmt.Fprintln(sb, "Method\tPattern\t\tTitle")
	fmt.Fprintln(sb, "------\t-------\t\t-----")
	return func(h http.Handler, pn *structpages.PageNode) http.Handler {
		fmt.Fprintf(sb, "%s\t%- 12s\t%s\n", pn.Method, pn.FullRoute(), pn.Title)
		return h
	}
}

func main() {
	var routes strings.Builder
	sp := structpages.New(
		structpages.WithErrorHandler(errorHandler),
		structpages.WithMiddlewares(PrintRoutes(&routes)),
	)
	router := structpages.NewRouter(http.DefaultServeMux)
	if err := sp.MountPages(router, index{}, "/", "index"); err != nil {
		log.Fatalf("Failed to mount pages: %v", err)
	}
	fmt.Println("Available routes:\n", routes.String())
	log.Println("Starting server on :8080")
	http.ListenAndServe(":8080", router)
}

func errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("Error: %v", err)
	if r.Header.Get("Hx-Request") == "true" {
		errorComp(err).Render(r.Context(), w)
		return
	}
	errorPage(err).Render(r.Context(), w)
}
