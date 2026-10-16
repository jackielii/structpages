package main

import (
	"net/http"

	"ex/orbit"

	"github.com/jackielii/structpages"
)

// Section is an alias of the orbit root.
type Section = orbit.Root

type routes struct {
	Orbit Section `route:"/orbit Orbit"`
}

func main() {
	structpages.Mount(http.NewServeMux(), routes{}, "/", "App")
}
