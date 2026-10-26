package structpages

import (
	"net/http"
	"strings"
)

// HTMXRenderTarget is the TargetSelector for htmx 1.x and 2.x. Opt in with
// WithTargetSelector(HTMXRenderTarget).
//
// For an htmx request (HX-Request: true) with an HX-Target, it matches the
// target id against the served page's components:
//   - the id the page generated for a component, via ID or the id filter
//     ("index-todo-list" → TodoList);
//   - the component's name with or without the page prefix ("content",
//     "index-content" → Content), or a suffix of it ("load-more" →
//     EventListLoadMore).
//
// The target Is the matched component. When nothing matches, the target Is
// none of the page's components, but still answers Is for a standalone
// function or another page's component carrying that id, so Props can
// recognise a region it shares. Any other request wants Page.
//
// For htmx 4, use [HTMXv4RenderTarget] instead.
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

			return newHTMXUnmatchedTarget(hxTarget, pn, pcCtx.Value(r.Context())), nil
		}
	}

	// Not a partial request: the request wants Page. On a page without
	// Page, the target Is nothing and Props must use RenderComponent.
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
			return newHTMXUnmatchedTarget(key, pn, pcCtx.Value(r.Context())), nil
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

	// The exact id of another page's component names that component, not
	// a loosely matching one of this page's.
	if pc != nil {
		if _, ok := pc.componentOwner(target); ok {
			return ""
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

// htmxUnmatchedTarget is what the htmx selectors return when HX-Target names
// none of the served page's own components: the request wants something
// this page doesn't have. Is reports whether that something is a given
// standalone function or another page's component, so Props can recognise
// a region it shares on purpose and render it with RenderComponent. It Is
// none of the served page's components, so a Props that returns no error
// gets Page.
type htmxUnmatchedTarget struct {
	key  string        // the HX-Target id, without a leading '#'
	page *PageNode     // the page being served
	pc   *parseContext // nil when the selector runs outside the request pipeline
}

func newHTMXUnmatchedTarget(key string, pn *PageNode, pc *parseContext) RenderTarget {
	return &htmxUnmatchedTarget{key: strings.TrimPrefix(key, "#"), page: pn, pc: pc}
}

// Is reports whether the requested id belongs to component, which is a
// standalone function or a page method (expression or bound value).
//
// A function matches its package-qualified id, the one ID generates
// ("<package>-<func>"), so two packages' same-named components stay
// distinct. Its bare id and the served-page-prefixed form also match, as a
// partial-id convenience that doesn't disambiguate by package.
//
// A method matches only the exact id ID generates for it on a mounted
// page. A method of the served page's own type never matches: a request
// for one of its own components is matched by the selector, so it Is none
// of them. (A type mounted twice can't address its other mount this way.)
func (t *htmxUnmatchedTarget) Is(component any) bool {
	info, err := extractMethodInfo(component)
	if err != nil {
		return false
	}
	if info.isFunction {
		return t.isFunction(info)
	}
	if t.pc == nil || pageNodeHasType(t.page, info) {
		return false
	}
	ref, ok := t.pc.componentOwner(t.key)
	return ok && ref.method == info.methodName && pageNodeHasType(ref.page, info)
}

// pageNodeHasType reports whether pn's value has the receiver type info
// names.
func pageNodeHasType(pn *PageNode, info *methodInfo) bool {
	return info.receiverIs(pn.Value.Type())
}

func (t *htmxUnmatchedTarget) isFunction(info *methodInfo) bool {
	funcKebab := camelToKebab(info.methodName)
	if info.packageName != "" && t.key == camelToKebab(info.packageName)+"-"+funcKebab {
		return true
	}
	return t.key == funcKebab ||
		strings.TrimPrefix(t.key, camelToKebab(t.page.Name)+"-") == funcKebab
}
