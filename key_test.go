package structpages

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	uikit "github.com/jackielii/structpages/internal/ui-Kit"
)

// keyedList renders one Row per item. A request for one row's keyed id
// renders that row; Props reads which one with KeyOf.
type keyedList struct{}

func (p keyedList) Props(t RenderTarget) (string, error) {
	switch {
	case t.Is(p.Row):
		return KeyOf(t), nil
	case t.Is(keyedPeople.Card):
		return "", RenderComponent(testComponent{"SHARED-CARD:" + KeyOf(t)})
	case t.Is(keyedBadge):
		return "", RenderComponent(testComponent{"BADGE:" + KeyOf(t)})
	}
	return "", nil
}

func (keyedList) Page() component          { return testComponent{"LIST-PAGE"} }
func (keyedList) Content() component       { return testComponent{"LIST-CONTENT"} }
func (keyedList) Row(key string) component { return testComponent{"ROW:" + key} }

type keyedPeople struct{}

func (keyedPeople) Page() component { return testComponent{"PEOPLE-PAGE"} }
func (keyedPeople) Card() component { return testComponent{"PEOPLE-CARD"} }

func keyedBadge() component { return testComponent{"BADGE"} }

type keyedRoot struct {
	List   keyedList   `route:"/list List"`
	People keyedPeople `route:"/people People"`
}

func mountKeyed(t *testing.T, sel TargetSelector) (*http.ServeMux, *StructPages) {
	t.Helper()
	mux := http.NewServeMux()
	sp, err := Mount(mux, keyedRoot{}, "/", "App", WithTargetSelector(sel))
	if err != nil {
		t.Fatalf("Mount: %v", err)
	}
	return mux, sp
}

func TestKey_appendsEscapedKeyToID(t *testing.T) {
	_, sp := mountKeyed(t, HTMXv4RenderTarget)
	ctx := pcCtx.WithValue(context.Background(), sp.pc)
	base, err := sp.ID(keyedList.Row)
	if err != nil {
		t.Fatalf("sp.ID: %v", err)
	}
	for key, escaped := range map[string]string{
		"s3":                                   "s3",
		"550e8400-e29b-41d4-a716-446655440000": "550e8400-e29b-41d4-a716-446655440000",
		"a b":                                  "a_20b",
		"x_y":                                  "x_5fy",
		"é":                                    "_c3_a9",
		"a--b":                                 "a--b",
	} {
		want := base + "--" + escaped
		for name, got := range map[string]func() (string, error){
			"Key":    func() (string, error) { return Key(ctx, keyedList.Row, key) },
			"sp.Key": func() (string, error) { return sp.Key(keyedList.Row, key) },
		} {
			if id, err := got(); err != nil || id != want {
				t.Errorf("%s(Row, %q) = %q, %v; want %q", name, key, id, err, want)
			}
		}
		for name, got := range map[string]func() (string, error){
			"KeyTarget":    func() (string, error) { return KeyTarget(ctx, keyedList.Row, key) },
			"sp.KeyTarget": func() (string, error) { return sp.KeyTarget(keyedList.Row, key) },
		} {
			if sel, err := got(); err != nil || sel != "#"+want {
				t.Errorf("%s(Row, %q) = %q, %v; want %q", name, key, sel, err, "#"+want)
			}
		}
	}
}

// Key takes every component form ID takes.
func TestKey_componentForms(t *testing.T) {
	_, sp := mountKeyed(t, HTMXv4RenderTarget)
	ctx := pcCtx.WithValue(context.Background(), sp.pc)
	for name, v := range map[string]any{
		"method expression": keyedList.Row,
		"bound method":      keyedList{}.Row,
		"chain":             []any{keyedList{}, "Row"},
		"Ref":               Ref("List.Row"),
		"function":          keyedBadge,
	} {
		id, err := ID(ctx, v)
		if err != nil {
			t.Fatalf("%s: ID: %v", name, err)
		}
		if got, err := Key(ctx, v, "k"); err != nil || got != id+"--k" {
			t.Errorf("%s: Key = %q, %v; want %q", name, got, err, id+"--k")
		}
	}
}

func TestKey_errors(t *testing.T) {
	_, sp := mountKeyed(t, HTMXv4RenderTarget)
	ctx := pcCtx.WithValue(context.Background(), sp.pc)
	for name, call := range map[string]func() (string, error){
		"Key empty key":          func() (string, error) { return Key(ctx, keyedList.Row, "") },
		"KeyTarget empty key":    func() (string, error) { return KeyTarget(ctx, keyedList.Row, "") },
		"sp.Key empty key":       func() (string, error) { return sp.Key(keyedList.Row, "") },
		"sp.KeyTarget empty key": func() (string, error) { return sp.KeyTarget(keyedList.Row, "") },
		"Key not a component":    func() (string, error) { return Key(ctx, 42, "k") },
		"sp.Key not a component": func() (string, error) { return sp.Key(42, "k") },
		"Key without context":    func() (string, error) { return Key(context.Background(), keyedList.Row, "k") },
		"Key plain string":       func() (string, error) { return Key(ctx, "rows", "k") },
		"sp.Key plain string":    func() (string, error) { return sp.Key("rows", "k") },
	} {
		if got, err := call(); err == nil {
			t.Errorf("%s = %q, nil; want an error", name, got)
		}
	}
}

// Every byte string survives the round trip through an id, and the id
// part is a CSS identifier needing no escaping.
func TestEscapeKey_roundTrip(t *testing.T) {
	var all strings.Builder
	for b := range 256 {
		all.WriteByte(byte(b))
	}
	for _, key := range []string{all.String(), "s3", "_", "-", "--", "a_5fb", "ü"} {
		esc := escapeKey(key)
		if i := strings.IndexFunc(esc, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_')
		}); i >= 0 {
			t.Errorf("escapeKey(%q) = %q: byte %q outside [A-Za-z0-9_-]", key, esc, esc[i])
		}
		if got, ok := unescapeKey(esc); !ok || got != key {
			t.Errorf("unescapeKey(escapeKey(%q)) = %q, %v", key, got, ok)
		}
	}
}

// Only what escapeKey produces unescapes, so one key has one id.
func TestUnescapeKey_rejectsNonCanonical(t *testing.T) {
	// "_61" and "_2d" escape bytes escapeKey keeps ('a', '-').
	for _, s := range []string{"", "a_4A", "a_4", "a_", "a_zz", "a.b", "a b", "a#b", "_61", "a_2db"} {
		if got, ok := unescapeKey(s); ok {
			t.Errorf("unescapeKey(%q) = %q, true; want false", s, got)
		}
	}
}

// A keyed id selects its component and carries its key, with either
// htmx selector, whatever the key holds.
func TestKeyedTarget_selectsInstance(t *testing.T) {
	for _, tc := range []struct {
		name   string
		sel    TargetSelector
		header func(id string) string
	}{
		{"htmx 2", HTMXRenderTarget, func(id string) string { return id }},
		{"htmx 4", HTMXv4RenderTarget, func(id string) string { return "tr#" + id }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mux, sp := mountKeyed(t, tc.sel)
			for _, key := range []string{"s3", "a b/ü_--", "content", "-", "--"} {
				id, err := sp.Key(keyedList.Row, key)
				if err != nil {
					t.Fatalf("sp.Key: %v", err)
				}
				if got := serveHX(mux, "/list", tc.header(id)); got != "ROW:"+key {
					t.Errorf("HX-Target %q rendered %q, want %q", id, got, "ROW:"+key)
				}
			}
		})
	}
}

// The unkeyed id of a keyed component still selects it, with no key.
func TestKeyedTarget_unkeyedID(t *testing.T) {
	mux, sp := mountKeyed(t, HTMXv4RenderTarget)
	id, err := sp.ID(keyedList.Row)
	if err != nil {
		t.Fatalf("sp.ID: %v", err)
	}
	if got := serveHX(mux, "/list", "tr#"+id); got != "ROW:" {
		t.Errorf("unkeyed Row id rendered %q, want %q", got, "ROW:")
	}
}

// A keyed id of another page's component or of a standalone function
// leaves the target unmatched, but it Is that component and carries the
// key, so Props can render the shared region's instance.
func TestKeyedTarget_sharedRegions(t *testing.T) {
	mux, sp := mountKeyed(t, HTMXv4RenderTarget)
	card, err := sp.Key(keyedPeople.Card, "p1")
	if err != nil {
		t.Fatalf("sp.Key(Card): %v", err)
	}
	badge, err := sp.Key(keyedBadge, "b 1")
	if err != nil {
		t.Fatalf("sp.Key(keyedBadge): %v", err)
	}
	for target, want := range map[string]string{
		card:  "SHARED-CARD:p1",
		badge: "BADGE:b 1",
	} {
		if got := serveHX(mux, "/list", "div#"+target); got != want {
			t.Errorf("HX-Target %q rendered %q, want %q", target, got, want)
		}
	}
}

// Keys come only from ids Key generated, so a keyed id matches exactly:
// never by the loose name and suffix rules that serve hand-written ids.
func TestKeyedTarget_matchesExactly(t *testing.T) {
	mux, _ := mountKeyed(t, HTMXv4RenderTarget)
	for _, target := range []string{
		"list-wrapper--list-content", // ends with Content's id
		"keyed-badge--b1",            // keyedBadge's bare, unqualified id
		"list--row",                  // the page name, then a component name
	} {
		if got := serveHX(mux, "/list", "div#"+target); got != "LIST-PAGE" {
			t.Errorf("HX-Target %q rendered %q, want LIST-PAGE", target, got)
		}
	}
}

// A malformed key part is no key: the id is matched as an unkeyed id.
func TestKeyedTarget_malformedKeyIsUnkeyed(t *testing.T) {
	mux, sp := mountKeyed(t, HTMXv4RenderTarget)
	id, err := sp.ID(keyedList.Row)
	if err != nil {
		t.Fatalf("sp.ID: %v", err)
	}
	for _, target := range []string{id + "--", id + "--a_4A", id + "--a.b"} {
		if got := serveHX(mux, "/list", "tr#"+target); got != "LIST-PAGE" {
			t.Errorf("HX-Target %q rendered %q, want LIST-PAGE", target, got)
		}
	}
}

type keyedCustomTarget struct{ key string }

func (keyedCustomTarget) Is(any) bool   { return false }
func (t keyedCustomTarget) Key() string { return t.key }

type unkeyedCustomTarget struct{}

func (unkeyedCustomTarget) Is(any) bool { return false }

func TestKeyOf(t *testing.T) {
	page := newMethodRenderTarget("Page", &reflect.Method{})
	for name, tc := range map[string]struct {
		target RenderTarget
		want   string
	}{
		"nil":                     {nil, ""},
		"unkeyed method target":   {page, ""},
		"custom target with Key":  {keyedCustomTarget{"x"}, "x"},
		"custom target, no Key":   {unkeyedCustomTarget{}, ""},
		"unmatched without a key": {newHTMXUnmatchedTarget("elsewhere", &PageNode{Name: "List"}, nil), ""},
	} {
		if got := KeyOf(tc.target); got != tc.want {
			t.Errorf("%s: KeyOf = %q, want %q", name, got, tc.want)
		}
	}
}

// serveHX serves an htmx request for path targeting hxTarget.
func serveHX(mux *http.ServeMux, path, hxTarget string) string {
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", hxTarget)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec.Body.String()
}

// A function in a package whose path element kebab-cases with a doubled
// hyphen still gets an id without "--", so its keyed and unkeyed ids both
// select it.
func TestKey_functionIDNeverContainsSeparator(t *testing.T) {
	_, sp := mountKeyed(t, HTMXv4RenderTarget)
	id, err := sp.ID(uikit.Badge)
	if err != nil {
		t.Fatalf("sp.ID: %v", err)
	}
	if id != "ui-kit-badge" {
		t.Errorf("sp.ID(uikit.Badge) = %q, want %q", id, "ui-kit-badge")
	}
	keyed, err := sp.Key(uikit.Badge, "k--1")
	if err != nil {
		t.Fatalf("sp.Key: %v", err)
	}
	list := nodeByRoute(sp.pc, "/list")
	for target, wantKey := range map[string]string{id: "", keyed: "k--1"} {
		tgt := htmxTarget(target, list, sp.pc)
		if !tgt.Is(uikit.Badge) || KeyOf(tgt) != wantKey {
			t.Errorf("target %q: Is(uikit.Badge) = %v, KeyOf = %q; want true, %q",
				target, tgt.Is(uikit.Badge), KeyOf(tgt), wantKey)
		}
	}
}

func ptr[T any](v T) *T { return &v }

type keyer interface{ Key() string }

type tenant struct{}

func (tenant) Key() string { return "acme" }

type tenantPage struct{}

func (tenantPage) Props(k keyer) (string, error) { return k.Key(), nil }
func (tenantPage) Page(name string) component    { return testComponent{"tenant=" + name} }
func (tenantPage) Content(name string) component { return testComponent{"content=" + name} }

// The built-in targets are injected as before: a Props parameter of an
// interface asking only for Key() string is filled from WithArgs, not
// with the render target.
func TestKeyOf_builtInTargetsInjectAsBefore(t *testing.T) {
	for sel, hxTarget := range map[*TargetSelector]string{
		ptr(TargetSelector(HTMXRenderTarget)):   "tenant-page-content--k",
		ptr(TargetSelector(HTMXv4RenderTarget)): "div#tenant-page-content--k",
	} {
		mux := http.NewServeMux()
		if _, err := Mount(mux, struct {
			tenantPage `route:"/{$} Tenant"`
		}{}, "/", "App", WithTargetSelector(*sel), WithArgs(tenant{})); err != nil {
			t.Fatalf("Mount: %v", err)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
		if got := rec.Body.String(); got != "tenant=acme" {
			t.Errorf("GET / rendered %q, want %q", got, "tenant=acme")
		}
		if got := serveHX(mux, "/", hxTarget); got != "content=acme" {
			t.Errorf("keyed htmx request rendered %q, want %q", got, "content=acme")
		}
	}
}

func TestIDSegment(t *testing.T) {
	for in, want := range map[string]string{
		"UserList": "user-list",
		"HTMLPage": "html-page",
		"fnfmt":    "fnfmt",
		"fn-fmt":   "fn-fmt",
		"ui-Kit":   "ui-kit",
		"my--lib":  "my-lib",
		"lib-":     "lib",
		"-lib":     "lib",
		"":         "",
	} {
		if got := idSegment(in); got != want {
			t.Errorf("idSegment(%q) = %q, want %q", in, got, want)
		}
	}
}
