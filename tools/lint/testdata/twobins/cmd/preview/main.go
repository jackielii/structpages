package main

import (
	"net/http"

	"ex/shared"

	"github.com/jackielii/structpages"
)

func main() {
	mux := http.NewServeMux()
	structpages.Mount(mux, shared.Root{}, "/", "Preview")
}
