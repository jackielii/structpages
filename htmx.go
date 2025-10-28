package structpages

import (
	"net/http"
	"strings"
)

// HTMXPageConfig is the default component selector for HTMX integration.
// It automatically selects the appropriate component method based on the HX-Target header.
//
// When an HTMX request is detected (via HX-Request header), it matches the HX-Target
// value against all available component IDs. For example:
//   - HX-Target: "content" -> calls Content() method
//   - HX-Target: "index-page-todo-list" -> calls TodoList() method on IndexPage
//   - No HX-Target or non-HTMX request -> calls Page() method
//
// This is the default component selector for StructPages, making IDFor work
// seamlessly with HTMX out of the box.
func HTMXPageConfig(r *http.Request, pn *PageNode) (string, error) {
	if r.Header.Get("HX-Request") == "true" {
		hxTarget := r.Header.Get("HX-Target")
		if hxTarget != "" {
			// Match target against all available components
			// This is more flexible than trying to extract/transform the component name
			componentName := matchComponentByTarget(hxTarget, pn)
			if componentName != "" {
				return componentName, nil
			}
		}
	}
	return "Page", nil
}

// matchComponentByTarget finds a component that matches the given HX-Target ID.
// It prioritizes matches from most specific to least specific:
//
//  1. Exact match with page prefix: "index-page-todo-list" → TodoList
//  2. Exact match without page prefix: "todo-list" → TodoList
//  3. Suffix match (best overlap): "load-more" → EventListLoadMore
//
// This flexible matching allows HTMX targets to work even with partial IDs.
func matchComponentByTarget(target string, pn *PageNode) string {
	if target == "" || strings.Contains(target, " ") {
		return ""
	}

	// Remove leading # if present
	target = strings.TrimPrefix(target, "#")

	pagePrefix := camelToKebab(pn.Name)

	// First pass: look for exact matches (highest priority)
	for componentName := range pn.Components {
		componentID := camelToKebab(componentName)
		fullID := pagePrefix + "-" + componentID

		// Exact match with page prefix (highest priority)
		if target == fullID {
			return componentName
		}

		// Exact match without page prefix
		if target == componentID {
			return componentName
		}
	}

	// Second pass: look for suffix matches if no exact match found
	// Track the best suffix match (longest match wins)
	bestMatch := ""
	bestMatchLen := 0
	for componentName := range pn.Components {
		componentID := camelToKebab(componentName)
		fullID := pagePrefix + "-" + componentID

		// Check if fullID ends with target
		// e.g., fullID="index-page-event-list-load-more" ends with target="list-load-more"
		if strings.HasSuffix(fullID, target) && len(target) > bestMatchLen {
			bestMatch = componentName
			bestMatchLen = len(target)
		}

		// Check if target ends with fullID
		// e.g., target="index-page-event-list-load-more" ends with fullID="index-page-load-more"
		if strings.HasSuffix(target, fullID) && len(fullID) > bestMatchLen {
			bestMatch = componentName
			bestMatchLen = len(fullID)
		}

		// Check if target ends with componentID (without page prefix)
		// BUT only if target starts with the correct page prefix
		// e.g., target="index-page-event-list-load-more" starts with "index-page-" and ends with "load-more"
		// This prevents "home-content" from matching when the page name is "IndexPage"
		if strings.HasPrefix(target, pagePrefix+"-") &&
			strings.HasSuffix(target, componentID) &&
			len(componentID) > bestMatchLen {
			bestMatch = componentName
			bestMatchLen = len(componentID)
		}
	}

	return bestMatch
}
