package templscan

import (
	"strings"
	"testing"
)

func TestScan_ParsesFixture(t *testing.T) {
	diags, err := Scan("testdata/empty.templ", nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected zero diagnostics, got %d: %v", len(diags), diags)
	}
}

func TestScan_ConstantAttribute_FlagsInternalLiteral(t *testing.T) {
	diags, err := Scan("testdata/constant_attr.templ", nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("want 1 diagnostic, got %d: %#v", len(diags), diags)
	}
	d := diags[0]
	if d.Category != "url-attr" {
		t.Errorf("category = %q, want url-attr", d.Category)
	}
	if d.Line != 4 {
		t.Errorf("line = %d, want 4", d.Line)
	}
	if !strings.Contains(d.Message, `"/login"`) {
		t.Errorf("message %q does not mention the literal", d.Message)
	}
	if !strings.Contains(d.Message, "hard-coded internal URL") {
		t.Errorf("message %q does not describe the rule", d.Message)
	}
}

func TestScan_ExpressionAttribute_FlagsInternalLiteral(t *testing.T) {
	diags, err := Scan("testdata/expr_literal.templ", nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("want 1 diagnostic, got %d: %#v", len(diags), diags)
	}
	d := diags[0]
	if d.Line != 4 {
		t.Errorf("line = %d, want 4", d.Line)
	}
	if !strings.Contains(d.Message, `"/login"`) {
		t.Errorf("message %q does not mention the literal", d.Message)
	}
}

func TestScan_ExpressionAttribute_FlagsConcat(t *testing.T) {
	diags, err := Scan("testdata/expr_concat.templ", nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(diags) != 3 {
		t.Fatalf("want 3 diagnostics, got %d: %#v", len(diags), diags)
	}
	for _, d := range diags {
		if !strings.Contains(d.Message, "string concatenation") {
			t.Errorf("message %q does not describe concat", d.Message)
		}
	}
}

func TestScan_ExpressionAttribute_FlagsSprintf(t *testing.T) {
	diags, err := Scan("testdata/expr_sprintf.templ", nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(diags) != 3 {
		t.Fatalf("want 3 diagnostics, got %d: %#v", len(diags), diags)
	}
	for _, d := range diags {
		if !strings.Contains(d.Message, "fmt.Sprint") {
			t.Errorf("message %q does not describe sprintf", d.Message)
		}
	}
}
