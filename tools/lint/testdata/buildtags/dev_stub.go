//go:build !devtools

package main

// devGroup is empty without the devtools tag — the dev routes are not mounted,
// so URLFor(devPage{}) cannot resolve unless the lint loads with -tags=devtools.
type devGroup struct{}
