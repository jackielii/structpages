package main

import (
	"log"
	"net/http"

	"github.com/jackielii/structpages"
)

func main() {
	sp := structpages.New(
		structpages.WithDefaultPageConfig(structpages.HTMXPageConfig),
	)
	router := structpages.NewRouter(http.DefaultServeMux)
	sp.MountPages(router, "/", index{})
	log.Printf("Registered pages:\n%s", structpages.PrintRoutes("/", &index{}))
	log.Println("Starting server on :8080")
	http.ListenAndServe(":8080", router)
}
