package templscan

import "testing"

func TestScan_ParsesFixture(t *testing.T) {
	diags, err := Scan("testdata/empty.templ", nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected zero diagnostics, got %d: %v", len(diags), diags)
	}
}
