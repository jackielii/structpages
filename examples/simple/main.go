package main

import (
	"log"
	"net/http"

	"github.com/jackielii/structpages"
)

func main() {
	mux := http.NewServeMux()
	sp, err := structpages.Mount(mux, index{}, "/", "index")
	if err != nil {
		log.Fatalf("Failed to mount pages: %v", err)
	}
	_ = sp // sp provides URLFor and IDFor methods
	log.Println("Starting server on :8080")
	http.ListenAndServe(":8080", mux)
}
