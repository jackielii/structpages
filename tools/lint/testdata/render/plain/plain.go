// Package plain is a tree whose template uses no request-issuing htmx
// attribute, so it needs no target selector.
package plain

// Root is the plain tree.
type Root struct {
	Home home `route:"/{$} Home"`
}

type home struct{}
