// Package htmxattr holds the rule the template scanners (templscan for
// .templ, gsxscan for .gsx) share for recognising htmx markup: which
// attribute names and element tags make a template issue htmx partial
// requests. The [selector] check uses it to tell whether a page tree is
// served to htmx.
package htmxattr

import "strings"

// Use is one piece of htmx markup found in a template. Line/Col are
// 1-indexed positions in Filename.
type Use struct {
	Filename string
	Line     int
	Col      int
	// Name is the attribute name or element tag as authored.
	Name string
}

// attrs are the htmx attributes that issue a request (hx-get …
// hx-delete, hx-boost) or shape which element a request swaps into
// (hx-target, hx-swap, hx-select); each sets the HX-Target /
// HX-Request headers a target selector reads.
var attrs = map[string]bool{
	"hx-get":    true,
	"hx-post":   true,
	"hx-put":    true,
	"hx-patch":  true,
	"hx-delete": true,
	"hx-target": true,
	"hx-swap":   true,
	"hx-boost":  true,
	"hx-select": true,
}

// IsAttr reports whether name is an htmx attribute in the set above.
// HTML attribute names are case-insensitive. htmx also reads every
// attribute with a "data-" prefix (data-hx-get), and htmx 4 spells
// inheritance and other modifiers as a ":" suffix on the name
// (hx-target:inherited), so both are stripped before the lookup.
func IsAttr(name string) bool {
	n := strings.ToLower(name)
	n = strings.TrimPrefix(n, "data-")
	n, _, _ = strings.Cut(n, ":")
	return attrs[n]
}

// IsElement reports whether tag is an htmx element: htmx 4's
// <hx-partial>, which targets a swap by id from a response.
func IsElement(tag string) bool {
	return strings.EqualFold(tag, "hx-partial")
}
