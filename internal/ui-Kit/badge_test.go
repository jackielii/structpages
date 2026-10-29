package uikit

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
	if got := b.String(); got != "uikit.Badge" {
		t.Errorf("rendered %q, want %q", got, "uikit.Badge")
	}
}
