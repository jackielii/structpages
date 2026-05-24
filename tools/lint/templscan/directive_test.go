package templscan

import (
	"strings"
	"testing"
)

func TestScan_HonoursHTMLCommentSuppression(t *testing.T) {
	diags, err := Scan("testdata/suppression.templ", nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(diags) != 1 {
		for _, d := range diags {
			t.Logf("diag: %s:%d:%d %s", d.Filename, d.Line, d.Col, d.Message)
		}
		t.Fatalf("want 1 unsuppressed diagnostic, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Message, `"/dashboard"`) {
		t.Errorf("the surviving diagnostic should be for /dashboard, got %q", diags[0].Message)
	}
}
