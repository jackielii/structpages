package main

import (
	"log"
	"net/http"

	"github.com/jackielii/structpages"
)

var sp = structpages.New()

func main() {
	r := structpages.NewRouter(nil)
	sp.MountPages(r, index{}, "/", "index")
	log.Println("Starting server on :8080")
	http.ListenAndServe(":8080", nil)
}
