package structpages

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

type degComp struct{}

func (degComp) Box() component { return testComponent{"Box"} }

type degAlpha struct {
	Item degComp `route:"/item A"`
}

type degBeta struct {
	Item degComp `route:"/item B"`
}

// Two mounts of the same component under different parents share the leaf
// name "Item", so the compact (leaf-only) id form needs a disambiguating
// hash suffix.
type degRoot struct {
	Alpha degAlpha `route:"/alpha Alpha"`
	Beta  degBeta  `route:"/beta Beta"`
}

// TestID_LengthDegradation exercises the full-prefix vs compact-fallback
// switch driven by maxIDLen, including the stable hash suffix applied to a
// non-unique leaf name in the compact regime.
func TestID_LengthDegradation(t *testing.T) {
	pc, err := parsePageTree("/", &degRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	// Generous budget → readable full-path form, no hash.
	pc.maxIDLen = 40
	alphaFull, err := IDTarget(ctx, []any{degAlpha{}, degComp.Box})
	if err != nil {
		t.Fatalf("alpha full: %v", err)
	}
	betaFull, err := IDTarget(ctx, []any{degBeta{}, degComp.Box})
	if err != nil {
		t.Fatalf("beta full: %v", err)
	}
	if alphaFull != "#alpha-item-box" {
		t.Errorf("alpha full id = %q, want %q", alphaFull, "#alpha-item-box")
	}
	if betaFull != "#beta-item-box" {
		t.Errorf("beta full id = %q, want %q", betaFull, "#beta-item-box")
	}

	// Tight budget → compact leaf-only form. The leaf "item" is shared, so
	// each carries a stable 4-hex hash suffix and they stay distinct.
	pc.maxIDLen = 8
	alphaC, err := IDTarget(ctx, []any{degAlpha{}, degComp.Box})
	if err != nil {
		t.Fatalf("alpha compact: %v", err)
	}
	betaC, err := IDTarget(ctx, []any{degBeta{}, degComp.Box})
	if err != nil {
		t.Fatalf("beta compact: %v", err)
	}
	for _, got := range []string{alphaC, betaC} {
		base := strings.TrimPrefix(got, "#")
		if !strings.HasPrefix(base, "item-box-") {
			t.Errorf("compact id %q does not use the leaf-only form 'item-box-<hash>'", got)
		}
		if suffix := strings.TrimPrefix(base, "item-box-"); len(suffix) != 4 {
			t.Errorf("compact id %q hash suffix = %q, want 4 hex chars", got, suffix)
		}
	}
	if alphaC == betaC {
		t.Fatalf("compact ids collide despite distinct paths: %q", alphaC)
	}

	// Stability: recomputing yields the identical hash.
	again, err := IDTarget(ctx, []any{degAlpha{}, degComp.Box})
	if err != nil {
		t.Fatalf("alpha compact again: %v", err)
	}
	if again != alphaC {
		t.Errorf("hash not stable across calls: %q then %q", alphaC, again)
	}
}

// TestWithMaxIDLength verifies the option threads through Mount and drives
// the same full-vs-compact decision via the public API.
func TestWithMaxIDLength(t *testing.T) {
	mux := http.NewServeMux()
	sp, err := Mount(mux, &degRoot{}, "/", "App", WithMaxIDLength(8))
	if err != nil {
		t.Fatalf("Mount: %v", err)
	}
	got, err := sp.IDTarget([]any{degAlpha{}, degComp.Box})
	if err != nil {
		t.Fatalf("IDTarget: %v", err)
	}
	if !strings.HasPrefix(strings.TrimPrefix(got, "#"), "item-box-") {
		t.Errorf("with maxIDLen=8, id = %q, want compact 'item-box-<hash>' form", got)
	}
}
