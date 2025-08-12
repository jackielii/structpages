package structpages

import (
	"net/http"
	"strings"
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
		hxTarget := r.Header.Get("Hx-Target")
		if hxTarget != "" {
			return mixedCase(hxTarget), nil
		}
	}
	return "Page", nil
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
