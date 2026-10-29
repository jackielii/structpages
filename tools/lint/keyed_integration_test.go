package lint

import (
	"fmt"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// keyedID is the finding for a generated id (ID, IDTarget) that attr
// extends with a runtime value; fix is the function to build it with.
func keyedID(file string, line int, attr, fix string) string {
	filter := map[string]string{"Key": "key", "KeyTarget": "keytarget"}[fix]
	return fmt.Sprintf("%s:%d: [keyed] %[3]s appends a runtime value to list.Row's generated id, so it is "+
		"no component's id: the htmx target selectors can't match a request targeting it to list.Row, "+
		"and Props can't read the key back. Build it with structpages.%[4]s (gsx: |> %[5]s(k)), whose "+
		"key Props reads with KeyOf",
		file, line, attr, fix, filter)
}

// keyedKey is the finding for an id Key or KeyTarget generated that
// attr extends with a runtime value.
func keyedKey(file string, line int, attr, fn, filter string) string {
	return fmt.Sprintf("%s:%d: [keyed] %[3]s appends a runtime value to the id %[4]s generates for "+
		"list.Row, so KeyOf reads the appended text back as part of the key, or the id isn't a keyed "+
		"id at all. Pass the whole key to %[4]s (gsx: |> %[5]s(k))",
		file, line, attr, fn, filter)
}

// TestKeyed_Integration covers instance ids built by hand from generated
// ids in testdata/keyed. Only the lines listed here are flagged; the
// fixtures' other elements cover what must stay quiet.
//
// app/pages.gsx: f`…` templates appending a key to the id, target, key
// and keytarget filters, to a hand-written ID and to a *StructPages ID;
// Go concatenation in an expression attribute; a local variable
// assigned once from ID or IDTarget, or from a concatenation; a "#"
// before an ID in an hx-target; an htmx extended selector; in-tag if and
// multi-line attributes. Quiet: a constant suffix (literal or named
// constant, a fixed role name), a role on a keyed id, Key and
// KeyTarget alone, an hx-target whose runtime value is in another
// compound or may start one, other attributes, text before the
// generated id, IDTarget as an id and ID as an hx-target without "#", a
// reassigned variable, a component prop and a suppressed line.
//
// tapp/pages.templ: the same joins through templ's source map, for
// concatenation in an expression attribute.
//
// app.go builds an instance id in Go outside any template: there is no
// attribute to check, so it isn't reported.
func TestKeyed_Integration(t *testing.T) {
	want := []string{
		keyedID("pages.gsx", 32, "id", "Key"),
		keyedID("pages.gsx", 33, "hx-target", "KeyTarget"),
		keyedID("pages.gsx", 34, "hx-target:inherited", "KeyTarget"),
		keyedID("pages.gsx", 35, "data-hx-target", "KeyTarget"),
		keyedID("pages.gsx", 36, "id", "Key"), // key, then a role
		keyedID("pages.gsx", 37, "id", "Key"), // a role, then a key
		keyedID("pages.gsx", 38, "id", "Key"), // no separator
		keyedID("pages.gsx", 39, "id", "Key"), // a call
		keyedID("pages.gsx", 40, "id", "Key"), // hand-written ID
		keyedID("pages.gsx", 41, "id", "Key"), // *StructPages method
		keyedID("pages.gsx", 42, "id", "Key"), // Go concatenation
		keyedID("pages.gsx", 43, "id", "Key"), // variable from ID
		keyedID("pages.gsx", 44, "id", "Key"), // variable holding the concatenation
		keyedID("pages.gsx", 45, "hx-target", "KeyTarget"),
		keyedID("pages.gsx", 46, "hx-target", "KeyTarget"), // "#" and ID
		keyedID("pages.gsx", 47, "hx-target", "KeyTarget"), // extended selector
		keyedKey("pages.gsx", 48, "id", "Key", "key"),
		keyedKey("pages.gsx", 49, "hx-target", "KeyTarget", "keytarget"),
		keyedID("pages.gsx", 52, "id", "Key"), // in-tag if
		keyedID("pages.gsx", 58, "id", "Key"), // multi-line
		keyedID("pages.templ", 13, "id", "Key"),
		keyedID("pages.templ", 14, "hx-target", "KeyTarget"), // "#" and ID
		keyedID("pages.templ", 16, "id", "Key"),              // multi-line
		keyedID("pages.templ", 19, "id", "Key"),              // variable holding the concatenation
	}
	got := categoryDiags(t, "testdata/keyed", keyedCategory)
	slices.Sort(got)
	slices.Sort(want)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("diagnostics (-want +got):\n%s", diff)
	}
}
