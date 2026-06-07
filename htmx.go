package structpages

import (
	"net/http"
	"strings"
)

// HTMXRenderTarget is the default TargetSelector for HTMX integration.
// It automatically selects the appropriate component based on the HX-Target header.
//
// When an HTMX request is detected (via HX-Request header), it matches the HX-Target
// value against all available component IDs. For example:
//   - HX-Target: "content" -> returns methodRenderTarget for Content() method
//   - HX-Target: "index-page-todo-list" -> returns methodRenderTarget for TodoList() method
//   - HX-Target: "user-stats-widget" (no method match) -> returns functionRenderTarget for lazy evaluation
//   - No HX-Target or non-HTMX request -> returns methodRenderTarget for Page() method
//
// This selector works with htmx 1.x and 2.x, where HX-Target carries the bare
// element id. For htmx 4, use [HTMXv4RenderTarget] instead.
//
// This is the default TargetSelector for StructPages, making IDFor work
// seamlessly with HTMX out of the box.
func HTMXRenderTarget(r *http.Request, pn *PageNode) (RenderTarget, error) {
	if r.Header.Get("HX-Request") == "true" {
		hxTarget := r.Header.Get("HX-Target")
		if hxTarget != "" {
			// Try to match against registered method components
			componentName := matchComponentByTarget(hxTarget, pn, pcCtx.Value(r.Context()))
			if componentName != "" {
				method := pn.Components[componentName]
				return newMethodRenderTarget(componentName, &method), nil
			}

			// No method match - assume it's a standalone function
			// Store raw hxTarget for lazy evaluation in Is()
			return newFunctionRenderTarget(hxTarget, pn.Name), nil
		}
	}

	// Default: render "Page" component
	// If no Page method exists (Props-only page), methodRenderTarget.Is() will return false,
	// forcing Props to use RenderComponent
	pageMethod := pn.Components["Page"]
	return newMethodRenderTarget("Page", &pageMethod), nil
}

// HTMXv4RenderTarget is the htmx 4 variant of [HTMXRenderTarget].
//
// htmx 4 reshaped two request headers we care about:
//
//   - HX-Target now carries "<tag>#<id>" (or just "<tag>" for unidentified
//     elements) instead of a bare id.
//   - HX-Request-Type was added: "full" when the swap target is <body> or
//     hx-select is in play, "partial" otherwise. The server is expected to
//     honor it.
//
// This selector treats HX-Request-Type=full as a hard hint to render the Page
// component. Otherwise it picks the matching key from HX-Target — preferring
// the id portion of "<tag>#<id>" when present, falling back to the tag for
// id-less targets — and applies the same matching rules as
// [HTMXRenderTarget]. A component named e.g. Form matches both
// `hx-target="#some-form"` (sent as "form#some-form") and `hx-target="form"`
// (sent as "form").
//
// HX-Source (htmx 4's HX-Trigger replacement) identifies the trigger element
// rather than the swap target, so it is not used for component routing. Read
// it from the request directly if you need it.
//
// See https://four.htmx.org/reference/#headers.
//
// Wire it via [WithTargetSelector] when serving an htmx 4 frontend:
//
//	sp, err := structpages.Mount(mux, root{}, "/", "App",
//	    structpages.WithTargetSelector(structpages.HTMXv4RenderTarget))
func HTMXv4RenderTarget(r *http.Request, pn *PageNode) (RenderTarget, error) {
	if r.Header.Get("HX-Request") == "true" &&
		r.Header.Get("HX-Request-Type") != "full" {
		if key := htmxv4TargetKey(r.Header.Get("HX-Target")); key != "" {
			if componentName := matchComponentByTarget(key, pn, pcCtx.Value(r.Context())); componentName != "" {
				method := pn.Components[componentName]
				return newMethodRenderTarget(componentName, &method), nil
			}
			return newFunctionRenderTarget(key, pn.Name), nil
		}
	}

	pageMethod := pn.Components["Page"]
	return newMethodRenderTarget("Page", &pageMethod), nil
}

// htmxv4TargetKey extracts the matching key from an htmx 4 HX-Target value.
// The header format is "<tag>#<id>" when the swap target has an id, or just
// "<tag>" when it doesn't. Prefer the id when available (more specific);
// otherwise use the tag so components like Form can match `hx-target="form"`.
func htmxv4TargetKey(target string) string {
	tag, id, ok := strings.Cut(target, "#")
	if ok && id != "" {
		return id
	}
	return tag
}

// matchComponentByTarget finds a component that matches the given HX-Target ID.
// It prioritizes matches from most specific to least specific:
//
//  0. Authoritative match against each component's real generated id
//     (requires pc): pc.componentID(pn, name) == target.
//  1. Exact match with page prefix: "index-page-todo-list" → TodoList
//  2. Exact match without page prefix: "todo-list" → TodoList
//  3. Suffix match (best overlap): "load-more" → EventListLoadMore
//
// Pass 0 is the true inverse of ID()/IDTarget(): it reproduces the id the
// page actually emitted, accounting for the full field-path prefix, the
// maxIDLen budget, and the compact "<leaf>-<method>-<hash>" degradation a
// long id collapses to. The field-name heuristic in passes 1–3 reconstructs
// ids from pn.Name alone and so cannot regenerate that hash suffix — before
// pass 0, any id that compacted past maxIDLen was unroutable and the request
// silently fell back to rendering Page (the full layout) into the swap target.
// Passes 1–3 remain for partial-id ergonomics and for callers with no pc
// (pc == nil — e.g. a selector invoked outside the request pipeline).
//
// This flexible matching allows HTMX targets to work even with partial IDs.
func matchComponentByTarget(target string, pn *PageNode, pc *parseContext) string {
	if target == "" || strings.Contains(target, " ") {
		return ""
	}

	// Remove leading # if present
	target = strings.TrimPrefix(target, "#")

	// Pass 0: authoritative match against the real generated id. Ids are
	// unique per (node, method) (checkIDUniqueness), so the first equal id
	// is the only match and iteration order is irrelevant.
	if pc != nil {
		for componentName := range pn.Components {
			if pc.componentID(pn, componentName, true) == target {
				return componentName
			}
		}
	}

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
		// e.g., target="wrapper-index-page-load-more" ends with fullID="index-page-load-more"
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
