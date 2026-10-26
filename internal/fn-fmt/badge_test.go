package fnfmt

import (
	"context"
	"strings"
	"testing"
)

func TestBadgeRenders(t *testing.T) {
	var b strings.Builder
	if err := Badge().Render(context.Background(), &b); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := b.String(); got != "fnfmt.Badge" {
		t.Errorf("rendered %q, want %q", got, "fnfmt.Badge")
	}
}
