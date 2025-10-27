package structpages

import (
	"net/http"
	"strings"
)

// HTMXPageConfig is the default component selector for HTMX integration.
// It automatically selects the appropriate component method based on the HX-Target header.
//
// When an HTMX request is detected (via HX-Request header), it converts the HX-Target
// value to a method name. For example:
//   - HX-Target: "content" -> calls Content() method
//   - HX-Target: "index-todo-list" -> calls TodoList() method on index page
//   - No HX-Target or non-HTMX request -> calls Page() method
//
// This is the default component selector for StructPages, making IDFor work
// seamlessly with HTMX out of the box.
func HTMXPageConfig(r *http.Request, pn *PageNode) (string, error) {
	if r.Header.Get("HX-Request") == "true" {
		hxTarget := r.Header.Get("HX-Target")
		if hxTarget != "" {
			// Extract component name from the target ID
			// e.g., "index-todo-list" -> "todo-list" -> "TodoList"
			componentName := extractComponentName(hxTarget, pn)
			if componentName != "" {
				if _, ok := pn.Components[componentName]; ok {
					return componentName, nil
				}
			}
		}
	}
	return "Page", nil
}

// extractComponentName extracts the component method name from an HX-Target ID.
// It handles both formats:
//   - Full ID: "index-todo-list" -> "TodoList" (strips page prefix)
//   - Simple: "content" -> "Content"
func extractComponentName(target string, pn *PageNode) string {
	if target == "" || strings.Contains(target, " ") {
		return ""
	}

	// Remove leading # if present
	target = strings.TrimPrefix(target, "#")

	// Try to strip page prefix (e.g., "index-todo-list" -> "todo-list")
	// This makes IDFor-generated IDs work automatically
	pagePrefix := camelToKebab(pn.Title)
	if pagePrefix != "" && strings.HasPrefix(target, pagePrefix+"-") {
		target = strings.TrimPrefix(target, pagePrefix+"-")
	}

	// Convert kebab-case to PascalCase
	return kebabToPascal(target)
}
