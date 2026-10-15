package gsxscan

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func scanLines(t *testing.T, filename string) []string {
	t.Helper()
	diags, err := Scan(filename)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	out := make([]string, len(diags))
	for i, d := range diags {
		if d.Category != "url-attr" {
			t.Errorf("category = %q, want url-attr", d.Category)
		}
		out[i] = fmt.Sprintf("%d:%d %s", d.Line, d.Col, d.Message)
	}
	return out
}

func TestScan_Attributes(t *testing.T) {
	got := scanLines(t, "testdata/attrs.gsx")
	want := []string{
		`6:11 [url-attr] href value "/login" is a hard-coded internal URL; use structpages.URLFor instead`,
		"7:11 [url-attr] href value `\"/\" + \"admin\"` builds an internal URL by string concatenation; " +
			"use structpages.URLFor instead",
		"8:11 [url-attr] href value `fmt.Sprintf(\"/users/%s\", name)` builds an internal URL via fmt.Sprint*; " +
			"use structpages.URLFor instead",
		`9:13 [url-attr] hx-get value "/api/items" is a hard-coded internal URL; use structpages.URLFor instead`,
		"10:15 [url-attr] hx-post value `f`/items/@{id}`` builds an internal URL by string interpolation; " +
			"use structpages.URLFor instead",
		`11:16 [url-attr] action value "/submit" is a hard-coded internal URL; use structpages.URLFor instead`,
		`17:12 [url-attr] href value "/in-if" is a hard-coded internal URL; use structpages.URLFor instead`,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("diagnostics mismatch (-want +got):\n%s", diff)
	}
}

func TestScan_HonoursSuppressionDirectives(t *testing.T) {
	got := scanLines(t, "testdata/suppression.gsx")
	if len(got) != 2 {
		t.Fatalf("want 2 unsuppressed diagnostics, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	for i, lit := range []string{`"/other-category"`, `"/dashboard"`} {
		if !strings.Contains(got[i], lit) {
			t.Errorf("diagnostic %d = %q, want it to mention %s", i, got[i], lit)
		}
	}
}

func TestScan_ParseErrorIsADiagnostic(t *testing.T) {
	got := scanLines(t, "testdata/broken.gsx")
	if len(got) != 1 || !strings.Contains(got[0], "gsx parse error") {
		t.Fatalf("want one parse-error diagnostic, got %q", got)
	}
}
