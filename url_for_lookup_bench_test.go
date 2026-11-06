package structpages

import (
	"context"
	"net/http"
	"testing"
)

// A page tree large enough for lookup cost to show: a root, one app
// wrapper, 8 sections of 8 leaves, a region page and a type mounted
// twice (79 nodes). The wrapper keeps the sections off the top level, as
// an authed subtree would, so qualified Refs anchor on a non-top-level
// node. Each leaf is a distinct type, since lookupLeaf is instantiated
// per (section, leaf) marker pair.
type (
	lkS0 struct{}
	lkS1 struct{}
	lkS2 struct{}
	lkS3 struct{}
	lkS4 struct{}
	lkS5 struct{}
	lkS6 struct{}
	lkS7 struct{}
	lkL0 struct{}
	lkL1 struct{}
	lkL2 struct{}
	lkL3 struct{}
	lkL4 struct{}
	lkL5 struct{}
	lkL6 struct{}
	lkL7 struct{}
)

type lookupLeaf[S, L any] struct{}

func (lookupLeaf[S, L]) Page() component { return testComponent{} }

type lookupSection[S any] struct {
	A lookupLeaf[S, lkL0] `route:"/a A"`
	B lookupLeaf[S, lkL1] `route:"/b B"`
	C lookupLeaf[S, lkL2] `route:"/c C"`
	D lookupLeaf[S, lkL3] `route:"/d D"`
	E lookupLeaf[S, lkL4] `route:"/e E"`
	F lookupLeaf[S, lkL5] `route:"/f F"`
	G lookupLeaf[S, lkL6] `route:"/g G"`
	H lookupLeaf[S, lkL7] `route:"/{id} H"`
}

type lookupApp struct {
	S0 lookupSection[lkS0] `route:"/s0/ S0"`
	S1 lookupSection[lkS1] `route:"/s1/ S1"`
	S2 lookupSection[lkS2] `route:"/s2/ S2"`
	S3 lookupSection[lkS3] `route:"/s3/ S3"`
	S4 lookupSection[lkS4] `route:"/s4/ S4"`
	S5 lookupSection[lkS5] `route:"/s5/ S5"`
	S6 lookupSection[lkS6] `route:"/s6/ S6"`
	S7 lookupSection[lkS7] `route:"/s7/ S7"`
}

// lkRegion is a page with a partial that other pages' rows target.
type lkRegion struct{}

func (lkRegion) Page() component { return testComponent{} }
func (lkRegion) Rows() component { return testComponent{} }

// lkTwice is mounted under two parents, so its ids differ per mount and
// only the current page can pick one.
type lkTwice struct{}

func (lkTwice) Page() component { return testComponent{} }
func (lkTwice) Rows() component { return testComponent{} }

type lkHolderX struct {
	Shared lkTwice `route:"/shared Shared"`
}

type lkHolderY struct {
	Shared lkTwice `route:"/shared Shared"`
}

type lookupRoot struct {
	App    lookupApp `route:"/app/ App"`
	Region lkRegion  `route:"/region Region"`
	X      lkHolderX `route:"/x/ X"`
	Y      lkHolderY `route:"/y/ Y"`
}

// BenchmarkURLForLookup measures resolving a page in a 79-node tree by
// each identifier form, with the target deep in walk order.
func BenchmarkURLForLookup(b *testing.B) {
	mux := http.NewServeMux()
	sp, err := Mount(mux, lookupRoot{}, "/", "Root")
	if err != nil {
		b.Fatal(err)
	}
	ctx := pcCtx.WithValue(context.Background(), sp.pc)

	cases := []struct {
		name string
		page any
	}{
		{"Type", lookupLeaf[lkS7, lkL6]{}},
		{"Chain", []any{lookupSection[lkS7]{}, lookupLeaf[lkS7, lkL6]{}}},
		{"RefQualified", Ref("S7.G")},
		{"RefName", Ref("S7")},
		{"RefRoute", Ref("/app/s7/g")},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			if _, err := URLFor(ctx, c.page); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				_, _ = URLFor(ctx, c.page)
			}
		})
	}
	b.Run("NavOf16", func(b *testing.B) {
		// One navigation fragment: 16 qualified-Ref links.
		refs := make([]Ref, 0, 16)
		for _, s := range []string{"S0", "S3", "S5", "S7"} {
			for _, l := range []string{"A", "C", "E", "G"} {
				refs = append(refs, Ref(s+"."+l))
			}
		}
		b.ReportAllocs()
		for b.Loop() {
			for _, r := range refs {
				_, _ = URLFor(ctx, r)
			}
		}
	})
}

// BenchmarkIDTargetLookup measures resolving a component id from a
// method expression or method value in a 79-node tree: from another
// page (cross-page, the receiver is found by searching the tree) and
// from a page rendering its own partial (current-page fast path).
func BenchmarkIDTargetLookup(b *testing.B) {
	mux := http.NewServeMux()
	sp, err := Mount(mux, lookupRoot{}, "/", "Root")
	if err != nil {
		b.Fatal(err)
	}
	ctx := pcCtx.WithValue(context.Background(), sp.pc)
	var leafPage, twiceY *PageNode
	for pn := range sp.pc.root.All() {
		switch {
		case pn.Name == "G" && pn.Parent.Name == "S7":
			leafPage = pn
		case pn.Name == "Shared" && pn.Parent.Name == "Y":
			twiceY = pn
		}
	}
	fromLeaf := currentPageCtx.WithValue(ctx, leafPage)
	fromTwice := currentPageCtx.WithValue(ctx, twiceY)

	cases := []struct {
		name string
		ctx  context.Context
		v    any
	}{
		{"Unbound", fromLeaf, lkRegion.Rows},
		{"Bound", fromLeaf, lkRegion{}.Rows},
		{"TwiceMountedCurrent", fromTwice, lkTwice.Rows},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			if _, err := IDTarget(c.ctx, c.v); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				_, _ = IDTarget(c.ctx, c.v)
			}
		})
	}
	b.Run("KeyTarget", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_, _ = KeyTarget(fromLeaf, lkRegion.Rows, "row-42")
		}
	})
	b.Run("RowsOf50", func(b *testing.B) {
		// One fragment rendering 50 rows, each targeting the region.
		b.ReportAllocs()
		for b.Loop() {
			for range 50 {
				_, _ = IDTarget(fromLeaf, lkRegion.Rows)
			}
		}
	})
}
