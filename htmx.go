package structpages

import (
	"net/http"
	"strings"
)

const (
	htmxRequestHxTarget    = "HX-Target"
	htmxResponseHxRetarget = "HX-Retarget"
)

// HTMXPageConfig is a page configuration function designed for HTMX integration.
// It automatically selects the appropriate component method based on the HX-Target header.
//
// When an HTMX request is detected (via HX-Request header), it converts the HX-Target
// value to a method name. For example:
//   - HX-Target: "content" -> calls Content() method
//   - HX-Target: "todo-list" -> calls TodoList() method
//   - No HX-Target or non-HTMX request -> calls Page() method
//
// This function can be used with WithDefaultPageConfig to enable HTMX partial
// rendering across all pages:
//
//	sp := structpages.New(
//	    structpages.WithDefaultPageConfig(structpages.HTMXPageConfig),
//	)
func HTMXPageConfig(r *http.Request, pn *PageNode) (string, error) {
	if isHTMX(r) {
		hxTarget := r.Header.Get(htmxRequestHxTarget)
		if hxTarget != "" {
			page := mixedCase(hxTarget)
			if _, ok := pn.Components[page]; !ok {
				return "Page", nil
			}
			return page, nil
		}
	}
	return "Page", nil
}

// HTMXPageRetargetMiddleware is a middleware that automatically retargets HTMX responses
// to the body element when a partial component target would result in a full page render.
//
// IMPORTANT: This middleware implements a heuristic based on common HTMX usage patterns.
// It is designed for a specific use case where:
//  1. You're using HTMXPageConfig (or similar) as the default page config
//  2. Your pages have partial component methods (Content, Sidebar, etc.)
//  3. You want automatic retargeting when a requested partial doesn't exist
//
// The heuristic works as follows:
//   - If an HTMX request targets a specific element (via HX-Target)
//   - AND the page doesn't have a custom PageConfig method (pn.Config == nil)
//   - AND the provided pageConfig returns "Page" (full page render)
//   - THEN it adds HX-Retarget: body to ensure the full page replaces the body
//
// This middleware will NOT retarget if:
//   - The page has its own PageConfig method (respecting custom behavior)
//   - The target is already "body"
//   - The pageConfig returns a partial component name
//
// This is useful for scenarios where:
//   - A partial update is requested but the server needs to render the full page
//   - The requested component doesn't exist and falls back to the full page
//   - Error conditions require showing the complete page instead of a partial
//
// Usage:
//
//	sp := structpages.New(
//	    structpages.WithDefaultPageConfig(structpages.HTMXPageConfig),
//	    structpages.WithMiddleware(
//	        structpages.HTMXPageRetargetMiddleware(structpages.HTMXPageConfig),
//	    ),
//	)
func HTMXPageRetargetMiddleware(pageConfig func(r *http.Request, pn *PageNode) (string, error)) MiddlewareFunc {
	return func(next http.Handler, pn *PageNode) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isHTMX(r) {
				hxTarget := r.Header.Get(htmxRequestHxTarget)
				if hxTarget != "" {
					// Skip retargeting if the page has its own PageConfig method
					// This respects custom page-specific behavior
					if pn.Config != nil {
						next.ServeHTTP(w, r)
						return
					}
					page, err := pageConfig(r, pn)
					if err == nil && hxTarget != "body" && strings.EqualFold(page, "Page") {
						w.Header().Set(htmxResponseHxRetarget, "body")
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MixedCase
func mixedCase(s string) string {
	if s == "" {
		return s
	}
	if strings.Contains(s, " ") {
		// hx-target values cannot contain spaces according to HTMX spec
		// Return empty string to indicate invalid target
		return ""
	}
	parts := strings.Split(s, "-")
	for i, part := range parts {
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("Hx-Request") == "true"
}
