// Command ex mounts a base route plus a build-tag-gated dev group: the dev
// routes are present only under -tags=devtools. Used to test that
// structpages-lint -tags resolves URLFor call sites against the tagged tree.
package main

import (
	"net/http"

	"github.com/jackielii/structpages"
)

type homePage struct{}

type webPages struct {
	Home homePage `route:"/{$} Home"`
	Dev  devGroup `route:"/"`
}

func main() {
	mux := http.NewServeMux()
	structpages.Mount(mux, webPages{}, "/", "App")
}
