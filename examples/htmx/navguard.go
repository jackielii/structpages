package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackielii/structpages"
)

// NavigationGuard keeps fragment URLs from being opened as documents.
//
// Under the route convention a page with Page is a document, and a page
// whose Props returns RenderComponent (and that has no Page) is a fragment:
// bare markup meant to be swapped into a document. A browser can still
// navigate to a fragment URL — a new tab, a bookmark, a reload — and would
// get that markup with no layout. For a document navigation to a fragment,
// the guard redirects to the page that hosts it without running Props.
//
// It also marks responses that differ by htmx headers as such, so a cache
// never answers a navigation with a partial.
func NavigationGuard(next http.Handler, pn *structpages.PageNode) http.Handler {
	_, hasPage := pn.Components["Page"]
	_, hasProps := pn.Props["Props"]
	fragment := !hasPage && hasProps
	if !hasPage && !fragment {
		return next // ServeHTTP pages: actions, redirects, downloads, JSON
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "HX-Request, HX-Target, HX-Request-Type, HX-History-Restore-Request, Sec-Fetch-Mode, Sec-Fetch-Dest")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		intent := requestIntent(r)
		if intent == partial || fragment {
			w.Header().Set("Cache-Control", "private, no-store")
		}
		if !fragment || intent == partial {
			next.ServeHTTP(w, r)
			return
		}
		to, err := landingURL(r, pn)
		if err != nil {
			http.NotFound(w, r) // render your app's error page here
			return
		}
		if intent == htmxFull {
			w.Header().Set("HX-Location", to)
			return
		}
		http.Redirect(w, r, to, http.StatusSeeOther)
	})
}

type intent int

const (
	document intent = iota // a browser navigation, curl, a test: wants a document
	htmxFull               // an htmx request for a whole page (htmx 4 HX-Request-Type: full)
	partial                // an htmx request for part of a page
)

// requestIntent reads what kind of response the request is for. Anything
// that is not explicitly an htmx partial counts as a document.
func requestIntent(r *http.Request) intent {
	switch r.Header.Get("Sec-Fetch-Dest") {
	case "document", "iframe", "frame":
		return document
	}
	// htmx 2 restores history on a cache miss with its own request, which
	// follows redirects but reads no response headers: send it a 303.
	if r.Header.Get("Sec-Fetch-Mode") == "navigate" ||
		r.Header.Get("HX-History-Restore-Request") == "true" ||
		r.Header.Get("HX-Request") != "true" {
		return document
	}
	if r.Header.Get("HX-Request-Type") == "full" {
		return htmxFull
	}
	return partial
}

// Landing is implemented by a fragment whose hosting page is not an
// ancestor, to name where a document navigation should go instead.
type Landing interface {
	Landing(r *http.Request) (string, error)
}

var errNoLanding = errors.New("no page hosts this fragment")

// landingURL returns where a document navigation to fragment pn goes: its
// Landing, else the nearest ancestor that is a page (or a group's index
// page) whose URL the request's path parameters can fill.
func landingURL(r *http.Request, pn *structpages.PageNode) (string, error) {
	if l, ok := pn.Value.Interface().(Landing); ok {
		return l.Landing(r)
	}
	for a := pn.Parent; a != nil; a = a.Parent {
		for _, n := range append([]*structpages.PageNode{a}, a.Children...) {
			if n != a && strings.Trim(n.Route, "/") != "{$}" {
				continue // only a itself, or its index child
			}
			if !isGETPage(n) {
				continue
			}
			to, err := structpages.URLFor(r.Context(), func(c *structpages.PageNode) bool { return c == n })
			if err == nil {
				return to, nil
			}
		}
	}
	return "", errNoLanding
}

func isGETPage(pn *structpages.PageNode) bool {
	_, hasPage := pn.Components["Page"]
	return hasPage && (pn.Method == http.MethodGet || pn.Method == "ALL")
}
