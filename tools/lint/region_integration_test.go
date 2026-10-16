package lint

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// regionContainer is the finding for an element without requests of
// its own: requests from inside it come back to p.
func regionContainer(file string, line int, p, attr, q string) string {
	return fmt.Sprintf("%s:%d: [region] %[3]s renders an element whose %[4]s is %[5]s's id: "+
		"requests from inside that element that come back to %[3]s target %[5]s's id, which %[3]s has no "+
		"component for, so %[3]s renders its Page into the region. Give the region a component of "+
		"%[3]s's own, or recognise it in %[3]s's Props with t.Is(%[5]s) and RenderComponent",
		file, line, p, attr, q)
}

// regionOwn is the finding for an element whose own request reaches p.
func regionOwn(file string, line int, p, attr, q, req string) string {
	return fmt.Sprintf("%s:%d: [region] %[3]s renders an element whose %[4]s is %[5]s's id, and the "+
		"element's %[6]s request reaches %[3]s targeting that id, which %[3]s has no component for, so "+
		"%[3]s renders its Page into the region. Give the region a component of %[3]s's own, or "+
		"recognise it in %[3]s's Props with t.Is(%[5]s) and RenderComponent",
		file, line, p, attr, q, req)
}

// TestRegion_Integration covers pages borrowing another page's region
// identity in testdata/region: an id or hx-target that is another
// mounted page's component id. Only the lines listed here are flagged;
// the fixture's other elements cover the cases that must stay quiet.
//
// app/pages.gsx and tapp/pages.templ: out-of-band hx-partial, other
// attributes, mismatched filters, standalone functions, the page's own
// components, component props, suppression, a Props that recognises
// the region, and a shared component type that isn't a page.
//
// app/requests.gsx and tapp/requests.templ: elements with requests of
// their own are flagged only when every one is a URLFor resolving back
// to the page (a lazy load or a form posting to another page, a literal
// URL, and one request elsewhere stay quiet); multi-line values,
// comments, explicit semicolons in templ Go code; a spread; promoted
// methods, owned as structpages.ID owns them at runtime.
//
// app_test.go mounts a stub tree with ace but not detail, as eos's
// section tests do: a tree that mounts only one of the two pages must
// not clear the findings. tapp/pages_templ.go carries another templ
// version in its header and tapp/noversion_templ.go none at all, which
// doesn't stop the templ join; tapp/stale_templ.go differs from this
// templ version's output elsewhere, so its borrowed region isn't
// reported.
func TestRegion_Integration(t *testing.T) {
	const dc = "detail.Content"
	want := []string{
		regionContainer("noversion.templ", 6, "ace", "id", dc),
		regionContainer("pages.gsx", 11, "ace", "id", dc),                  // []any chain
		regionContainer("pages.gsx", 12, "ace", "hx-target", dc),           // method expression
		regionContainer("pages.gsx", 13, "ace", "hx-target:inherited", dc), // bound value
		regionContainer("pages.gsx", 14, "ace", "data-hx-target", dc),
		regionContainer("pages.gsx", 15, "ace", "id", dc), // hand-written structpages.ID
		regionContainer("pages.gsx", 26, "ace", "id", dc), // inside control flow
		regionContainer("pages.templ", 10, "ace", "data-hx-target", dc),
		regionContainer("pages.templ", 19, "ace", "id", dc), // inside control flow
		regionContainer("pages.templ", 7, "ace", "id", dc),  // []any chain
		regionContainer("pages.templ", 8, "ace", "hx-target", dc),
		regionContainer("pages.templ", 9, "ace", "hx-target:inherited", dc), // bound value
		regionOwn("requests.gsx", 10, "ace", "hx-target", dc, "hx-get"),     // hand-written URLFor
		regionContainer("requests.gsx", 15, "ace", "hx-target", dc),         // multi-line staged
		regionContainer("requests.gsx", 17, "ace", "id", dc),                // multi-line hand-written
		regionContainer("requests.gsx", 19, "ace", "id", dc),                // with a comment
		regionContainer("requests.gsx", 20, "ace", "id", "wrap.Content"),    // promoted method expression
		regionContainer("requests.gsx", 25, "wrap", "id", dc),               // promoted method value
		regionOwn("requests.gsx", 9, "ace", "id", dc, "hx-get"),             // reloads from ace
		regionContainer("requests.templ", 14, "ace", "hx-target", dc),       // multi-line
		regionOwn("requests.templ", 9, "ace", "id", dc, "hx-get"),           // reloads from ace
	}
	if diff := cmp.Diff(want, moduleDiags(t, "testdata/region", regionCategory)); diff != "" {
		t.Errorf("diagnostics (-want +got):\n%s", diff)
	}
}

// TestRegion_Columns pins that a gsx finding points at the pipeline
// stage (or hand-written call) itself: gsx writes a blank between its
// /*line*/ directive and the call, so the call's own mapped column is
// one too far right.
func TestRegion_Columns(t *testing.T) {
	got := moduleDiagsAt(t, "testdata/region", regionCategory)
	for _, want := range []string{"pages.gsx:12:37", "pages.gsx:15:12", "requests.gsx:9:29"} {
		if !got[want] {
			t.Errorf("no region finding at %s; have %v", want, got)
		}
	}
}

// TestRegion_NoOtherFindings pins that testdata/region is otherwise
// clean, so the region cases aren't masking other problems. That
// includes route-literal, which used to report Mount's own route
// argument.
func TestRegion_NoOtherFindings(t *testing.T) {
	got := moduleDiags(t, "testdata/region", "idfor", "urlfor", "ref", "params", selectorCategory, "route-literal")
	if len(got) != 0 {
		t.Errorf("unexpected diagnostics:\n%v", got)
	}
}
