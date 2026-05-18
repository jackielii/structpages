package main

import "github.com/jackielii/structpages"

// Page tree mirrors design-system/internal/preview/pages.go from
// his-project. The three group parents share `groupIndex` for the
// landing page and `entryPage` for per-entry detail.
type root struct {
	Home        homePage        `route:"/{$} URL Validation Demo"`
	Foundations foundationsRoot `route:"/foundations Foundations"`
	Components  componentsRoot  `route:"/components Components"`
	Patterns    patternsRoot    `route:"/patterns Patterns"`
}

type foundationsRoot struct {
	Index  groupIndex `route:"/{$} Foundations"`
	Detail entryPage  `route:"/{slug} Foundation"`
}

type componentsRoot struct {
	Index  groupIndex `route:"/{$} Components"`
	Detail entryPage  `route:"/{slug} Component"`
}

type patternsRoot struct {
	Index  groupIndex `route:"/{$} Patterns"`
	Detail entryPage  `route:"/{slug} Pattern"`
}

// resolveGroup walks up the page node chain to find which group this
// request belongs to — exact copy of the his-project helper.
func resolveGroup(node *structpages.PageNode) string {
	for n := node; n != nil; n = n.Parent {
		switch n.Route {
		case "/foundations":
			return "foundations"
		case "/components":
			return "components"
		case "/patterns":
			return "patterns"
		}
	}
	return ""
}

// groupParent maps the group string back to its parent struct, used by
// detail-page links inside groupIndex.Page where the parent isn't
// statically known to the renderer. This dispatch is what the
// his-project refactor needed; with the []any chain form, the parent
// type is the only thing URLFor needs to disambiguate the shared leaf.
func groupParent(group string) (any, bool) {
	switch group {
	case "foundations":
		return foundationsRoot{}, true
	case "components":
		return componentsRoot{}, true
	case "patterns":
		return patternsRoot{}, true
	}
	return nil, false
}

// entries is the static fixture content per group. In his-project this
// comes from a Manifest; here we hard-code a few so the URLs have
// something to point at.
var entries = map[string][]string{
	"foundations": {"colors", "typography", "spacing"},
	"components":  {"button", "input", "dialog"},
	"patterns":    {"form-layout", "data-table"},
}
