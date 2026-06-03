//go:build devtools

package main

// devGroup mounts the dev-only routes; compiled only with -tags=devtools.
type devGroup struct {
	Dev devPage `route:"/_dev Dev"`
}
