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
// This middleware works in conjunction with a page config function to determine when
// retargeting is necessary. When an HTMX request targets a specific element (via HX-Target),
// but the page config returns "Page" (indicating a full page render), this middleware
// adds the HX-Retarget header to redirect the response to the body element.
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
