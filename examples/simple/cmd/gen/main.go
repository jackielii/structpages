// Command gen runs gsx codegen for the simple example with the structpages
// pipeline filters registered, so .gsx templates can write `{ page{} |> url }`,
// `{ x |> id }`, and `{ x |> target }` instead of the ctx-threading,
// error-returning structpages.URLFor / ID / IDTarget calls.
//
// Generate with:  go run ./cmd/gen generate .
package main

import (
	"github.com/gsxhq/gsx/gen"

	"github.com/jackielii/structpages"
)

func main() {
	gen.Main(
		gen.WithFilter("url", structpages.URLFor),
		gen.WithFilter("id", structpages.ID),
		gen.WithFilter("target", structpages.IDTarget),
	)
}
