// Package client doesn't depend on structpages, so it is parsed without
// types, yet a route literal in it is still reported.
package client

// AboutURL hardcodes a mounted route.
func AboutURL() string {
	return "/about" // want route-literal
}

// Matches compares against a route, which is not URL generation.
func Matches(path string) bool {
	return path == "/about"
}
