package lint

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestEscape_Integration covers values escaped with net/url before URLFor,
// which escapes them again: in every argument form, through a literal map,
// and not for raw values, a pre-encoded template, a spread slice or a
// suppressed line.
func TestEscape_Integration(t *testing.T) {
	got := categoryDiags(t, "testdata/escape", "escape")
	for i, d := range got {
		got[i], _, _ = strings.Cut(d, " is escaped again")
	}
	want := []string{
		"main.go:25: [escape] URLFor: url.QueryEscape value",
		"main.go:26: [escape] URLFor: url.QueryEscape value",
		"main.go:27: [escape] URLFor: url.PathEscape value",
		"main.go:28: [escape] URLFor: url.Values.Encode() as a value",
		"main.go:29: [escape] URLFor: url.QueryEscape value",
		"main.go:32: [escape] URLFor: url.QueryEscape value",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("diagnostics (-want +got):\n%s", diff)
	}
}
