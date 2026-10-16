package structpages

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// Chain form parallels URLFor's []any{Parent{}, Leaf{}, ...} shape.
// Leading typed values are chain steps; the trailing element is the
// method spec — either a string method name or a method expression
// whose receiver type IS the leaf.

type chainAdmin struct {
	Dashboard chainDashboard `route:"/dashboard Admin Dashboard"`
}

type chainUser struct {
	Dashboard chainDashboard `route:"/dashboard User Dashboard"`
}

type chainDashboard struct{}

func (chainDashboard) Header() component { return testComponent{"Header"} }

type chainRoot struct {
	Admin chainAdmin `route:"/admin Admin"`
	User  chainUser  `route:"/user User"`
}

func TestID_ChainForm(t *testing.T) {
	pc, err := parsePageTree("/", &chainRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	cases := []struct {
		name  string
		input []any
		want  string
	}{
		{
			name:  "chain with string method name",
			input: []any{chainAdmin{}, chainDashboard{}, "Header"},
			want:  "#admin-dashboard-header",
		},
		{
			name:  "chain with method expression terminal",
			input: []any{chainAdmin{}, chainDashboard.Header},
			want:  "#admin-dashboard-header",
		},
		{
			name:  "chain targeting user's dashboard (different parent, explicit descent)",
			input: []any{chainUser{}, chainDashboard{}, "Header"},
			want:  "#user-dashboard-header",
		},
		{
			name:  "chain leaf type matches method expr receiver (collapse duplicate descend)",
			input: []any{chainAdmin{}, chainDashboard{}, chainDashboard.Header},
			want:  "#admin-dashboard-header",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := IDTarget(ctx, tc.input)
			if err != nil {
				t.Fatalf("IDTarget: %v", err)
			}
			if got != tc.want {
				t.Errorf("IDTarget = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestID_ChainFormErrors(t *testing.T) {
	pc, err := parsePageTree("/", &chainRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)

	cases := []struct {
		name      string
		input     []any
		wantInErr string
	}{
		{
			name:      "empty chain",
			input:     []any{},
			wantInErr: "empty",
		},
		{
			name:      "chain with no method spec (only typed values)",
			input:     []any{chainAdmin{}, chainDashboard{}},
			wantInErr: "method",
		},
		{
			name:      "chain step does not descend correctly",
			input:     []any{chainAdmin{}, chainUser{}, "Header"},
			wantInErr: "child",
		},
		{
			name:      "method expression receiver type conflicts with prior chain step",
			input:     []any{chainAdmin{}, chainUser{}, chainDashboard.Header},
			wantInErr: "child",
		},
		{
			name:      "method name not on leaf type",
			input:     []any{chainAdmin{}, chainDashboard{}, "Nonexistent"},
			wantInErr: "method",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := IDTarget(ctx, tc.input)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantInErr)
			}
			if !strings.Contains(err.Error(), tc.wantInErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantInErr)
			}
		})
	}
}

// TestID_ChainFormLeavesCallerSliceIntact pins that resolving a chain
// ending in a method expression doesn't write the implicit receiver step
// into the caller's slice, so a stored chain can be reused.
func TestID_ChainFormLeavesCallerSliceIntact(t *testing.T) {
	sp, err := Mount(http.NewServeMux(), &degRoot{}, "/", "App")
	if err != nil {
		t.Fatalf("Mount: %v", err)
	}
	ref := []any{degAlpha{}, degComp.Box}
	first, err := sp.IDTarget(ref)
	if err != nil {
		t.Fatalf("first IDTarget: %v", err)
	}
	second, err := sp.IDTarget(ref)
	if err != nil {
		t.Fatalf("second IDTarget on the same slice: %v", err)
	}
	if first != second {
		t.Errorf("same chain gave %q then %q", first, second)
	}
}
