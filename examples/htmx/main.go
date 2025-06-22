package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/jackielii/structpages"
)

// PrintRoutes is a simple middleware that prints all routes to stdout
func PrintRoutes() structpages.MiddlewareFunc {
	fmt.Println("\nRoutes:")
	fmt.Println("Method\tPattern\t\tTitle")
	fmt.Println("------\t-------\t\t-----")
	return func(h http.Handler, pn *structpages.PageNode) http.Handler {
		fmt.Printf("%s\t%s\t\t%s\n", pn.Method, pn.FullRoute(), pn.Title)
		return h
	}
}

func main() {
	sp := structpages.New(
		structpages.WithDefaultPageConfig(structpages.HTMXPageConfig),
		structpages.WithErrorHandler(errorHandler),
		structpages.WithMiddlewares(PrintRoutes()),
	)
	router := structpages.NewRouter(http.DefaultServeMux)
	sp.MountPages(router, index{}, "/", "index")
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
