package main

import (
	"net/http"

	"ex/client"
	"ex/pages"

	"github.com/jackielii/structpages"
)

func main() {
	if _, err := structpages.Mount(http.NewServeMux(), pages.Root{}, "/", "App"); err != nil {
		panic(err)
	}
	_ = client.AboutURL()
}
