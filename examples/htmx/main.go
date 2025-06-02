package main

import (
	"log"
	"net/http"

	"github.com/jackielii/structpages"
)

func main() {
	sp := structpages.New(
		structpages.WithDefaultPageConfig(structpages.HTMXPageConfig),
		structpages.WithErrorHandler(errorHandler),
	)
	router := structpages.NewRouter(http.DefaultServeMux)
	sp.MountPages(router, index{}, "/", "index")
	log.Printf("Registered pages:\n%s", structpages.PrintRoutes(index{}, "/", "index"))
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
