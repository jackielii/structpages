package main

import (
	"log"
	"net/http"

	"github.com/jackielii/structpages"
)

var sp = structpages.New()

func main() {
	r := structpages.NewRouter(nil)
	if err := sp.MountPages(r, index{}, "/", "index"); err != nil {
		log.Fatalf("Failed to mount pages: %v", err)
	}
	log.Println("Starting server on :8080")
	http.ListenAndServe(":8080", nil)
}
