// Package tpl is a templ tree using htmx 4's inherited attributes.
package tpl

// Root is the templ tree.
type Root struct {
	Home home `route:"/{$} Home"`
}

type home struct{}
