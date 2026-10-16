package lint

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestRegion_Integration covers pages borrowing another page's region
// identity in testdata/region/app/pages.gsx: an id or hx-target that is
// another mounted page's component id. Only the lines listed here are
// flagged; the file's other elements cover the cases that must stay
// quiet (out-of-band hx-partial, other attributes, mismatched filters,
// standalone functions, the page's own components, component props,
// suppression, a Props that recognises the region, and a shared
// component type that isn't a page). app_test.go mounts a stub tree
// with ace but not detail, as eos's section tests do: a tree that
// mounts only one of the two pages must not clear the findings.
func TestRegion_Integration(t *testing.T) {
	msg := func(p, attr string) string {
		return fmt.Sprintf("[region] %[1]s renders an element whose %[2]s is detail.Content's id: "+
			"requests from that element reach %[1]s targeting detail.Content's id, which %[1]s has no "+
			"component for, so %[1]s renders its Page into the region. Give the region a component of "+
			"%[1]s's own, or recognise it in %[1]s's Props with t.Is(detail.Content) and RenderComponent", p, attr)
	}
	want := []string{
		"pages.gsx:11: " + msg("ace", "id"),                  // []any chain
		"pages.gsx:12: " + msg("ace", "hx-target"),           // method expression
		"pages.gsx:13: " + msg("ace", "hx-target:inherited"), // bound value
		"pages.gsx:14: " + msg("ace", "data-hx-target"),
		"pages.gsx:15: " + msg("ace", "id"), // hand-written structpages.ID
		"pages.gsx:26: " + msg("ace", "id"), // inside control flow
	}
	if diff := cmp.Diff(want, moduleDiags(t, "testdata/region", regionCategory)); diff != "" {
		t.Errorf("diagnostics (-want +got):\n%s", diff)
	}
}

// TestRegion_NoOtherFindings pins that testdata/region is otherwise
// clean, so the region cases aren't masking idfor/urlfor problems.
func TestRegion_NoOtherFindings(t *testing.T) {
	got := moduleDiags(t, "testdata/region", "idfor", "urlfor", "ref", "params", selectorCategory)
	if len(got) != 0 {
		t.Errorf("unexpected diagnostics:\n%v", got)
	}
}
