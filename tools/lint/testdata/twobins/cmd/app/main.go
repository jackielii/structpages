package main

import (
	"net/http"

	"ex/shared"

	"github.com/jackielii/structpages"
)

type webPages struct {
	DesignSystem shared.Root `route:"/design-system DesignSystem"`
}

func main() {
	mux := http.NewServeMux()
	structpages.Mount(mux, webPages{}, "/", "App")
}
