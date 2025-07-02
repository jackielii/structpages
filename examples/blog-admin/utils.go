package main

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
)

// Render helper for templ components
func render(w http.ResponseWriter, r *http.Request, component templ.Component) error {
	templ.Handler(component).ServeHTTP(w, r)
	return nil
}

// Ternary helper for strings
func ternary(condition bool, ifTrue, ifFalse string) string {
	if condition {
		return ifTrue
	}
	return ifFalse
}

// Ternary helper for URLs
func ternaryURL(condition bool, ifTrue, ifFalse templ.SafeURL) templ.SafeURL {
	if condition {
		return ifTrue
	}
	return ifFalse
}

// Helper for conditional URLs with error handling
func ternaryURLWithError(condition bool, ctx context.Context, pageTrue, pageFalse any, args ...any) templ.SafeURL {
	if condition {
		url, _ := urlFor(ctx, pageTrue)
		return url
	}
	url, _ := urlFor(ctx, pageFalse, args...)
	return url
}

// Helper to get URL or empty string
func urlForOrEmpty(ctx context.Context, page any, args ...any) templ.SafeURL {
	url, _ := urlFor(ctx, page, args...)
	return url
}
