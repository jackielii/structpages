// Package pages is the mounted tree.
package pages

// Root is the app's page tree.
type Root struct {
	About about `route:"/about About"`
}

type about struct{}
