package structpages

import (
	"context"
	"reflect"
	"testing"

	fnfmt "github.com/jackielii/structpages/internal/fn-fmt"
	"github.com/jackielii/structpages/internal/idconflicta"
	"github.com/jackielii/structpages/internal/idconflictb"
)

// widgetNodes returns the idConflictRoot tree and its two Widget nodes,
// one per package.
func widgetNodes(t *testing.T) (pc *parseContext, a, b *PageNode) {
	t.Helper()
	pc, err := parsePageTree("/", &idConflictRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	for n := range pc.root.All() {
		switch pointerType(n.Value.Type()) {
		case reflect.TypeFor[*idconflicta.Widget]():
			a = n
		case reflect.TypeFor[*idconflictb.Widget]():
			b = n
		}
	}
	if a == nil || b == nil {
		t.Fatal("Widget nodes not found")
	}
	return pc, a, b
}

// A bound method value (b.List) names its receiver's package as well as
// its type, so it resolves as exactly as the method expression: never to
// a same-named type from another package (#39).
func TestID_boundMethodValueCrossPackage(t *testing.T) {
	pc, aNode, bNode := widgetNodes(t)
	base := pcCtx.WithValue(context.Background(), pc)
	var a idconflicta.Widget
	var b idconflictb.Widget

	for _, fn := range []struct {
		name string
		id   func(context.Context, any) (string, error)
	}{
		{"ID", ID},
		{"IDTarget", IDTarget},
	} {
		wantA, err := fn.id(base, idconflicta.Widget.List)
		if err != nil {
			t.Fatalf("%s(idconflicta.Widget.List): %v", fn.name, err)
		}
		wantB, err := fn.id(base, idconflictb.Widget.List)
		if err != nil {
			t.Fatalf("%s(idconflictb.Widget.List): %v", fn.name, err)
		}
		for _, cur := range []struct {
			name string
			page *PageNode
		}{
			{"no current page", nil},
			{"current page idconflicta.Widget", aNode},
			{"current page idconflictb.Widget", bNode},
		} {
			ctx := base
			if cur.page != nil {
				ctx = currentPageCtx.WithValue(base, cur.page)
			}
			if got, err := fn.id(ctx, b.List); err != nil || got != wantB {
				t.Errorf("%s(b.List), %s: got %q, %v; want %q", fn.name, cur.name, got, err, wantB)
			}
			if got, err := fn.id(ctx, a.List); err != nil || got != wantA {
				t.Errorf("%s(a.List), %s: got %q, %v; want %q", fn.name, cur.name, got, err, wantA)
			}
		}
	}
}

// A request for a page's own component Is that component's bound value,
// and not the bound value of a same-named type from another package.
func TestMethodRenderTarget_boundMethodValueCrossPackage(t *testing.T) {
	m, ok := reflect.TypeFor[idconflicta.Widget]().MethodByName("List")
	if !ok {
		t.Fatal("idconflicta.Widget.List not found")
	}
	target := newMethodRenderTarget("List", &m)
	var a idconflicta.Widget
	var b idconflictb.Widget
	if !target.Is(a.List) {
		t.Error("Is(a.List) = false, want true")
	}
	if target.Is(b.List) {
		t.Error("Is(b.List) = true, want false: b is idconflictb.Widget")
	}
}

// With htmx, a request served by one Widget page that targets the other
// package's Widget.List region Is that region's bound value, and not the
// served page's own.
func TestHTMXUnmatchedTarget_boundMethodValueCrossPackage(t *testing.T) {
	pc, aNode, bNode := widgetNodes(t)
	var a idconflicta.Widget
	var b idconflictb.Widget
	for _, tt := range []struct {
		name         string
		served, want *PageNode
		is, isNot    any
	}{
		{"served a, target b", aNode, bNode, b.List, a.List},
		{"served b, target a", bNode, aNode, a.List, b.List},
	} {
		t.Run(tt.name, func(t *testing.T) {
			target := newHTMXUnmatchedTarget(pc.componentID(tt.want, "List", true), tt.served, pc)
			if !target.Is(tt.is) {
				t.Error("Is(other package's Widget.List) = false, want true")
			}
			if target.Is(tt.isNot) {
				t.Error("Is(served page's own Widget.List) = true, want false")
			}
		})
	}
}

// A bound method value's function name ends in "-fm"; a standalone
// function in a package whose path merely contains "-fm" is not one.
func TestID_functionInPackagePathWithFm(t *testing.T) {
	pc, err := parsePageTree("/", &idConflictRoot{})
	if err != nil {
		t.Fatalf("parsePageTree: %v", err)
	}
	ctx := pcCtx.WithValue(context.Background(), pc)
	if got, err := ID(ctx, fnfmt.Badge); err != nil || got != "fn-fmt-badge" {
		t.Errorf("ID(fnfmt.Badge) = %q, %v; want %q", got, err, "fn-fmt-badge")
	}
}
