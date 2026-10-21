package structpages

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// decodedURL is a generated URL taken apart and decoded, so tests compare
// what a server reads back rather than one choice of encoding.
type decodedURL struct {
	Segments []string // decoded path segments, split on the raw path's slashes
	Query    url.Values
	Fragment string
}

func decodeURL(t *testing.T, s string) decodedURL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", s, err)
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		t.Fatalf("url.ParseQuery(%q): %v", u.RawQuery, err)
	}
	raws := strings.Split(strings.TrimPrefix(u.EscapedPath(), "/"), "/")
	segs := make([]string, 0, len(raws))
	for _, raw := range raws {
		seg, err := url.PathUnescape(raw)
		if err != nil {
			t.Fatalf("url.PathUnescape(%q): %v", raw, err)
		}
		segs = append(segs, seg)
	}
	return decodedURL{Segments: segs, Query: q, Fragment: u.Fragment}
}

func TestFormatPathSegments_queryValuesRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		args    []any
		query   url.Values
	}{
		{"ampersand", "/search?q={q}", []any{"A&B"}, url.Values{"q": {"A&B"}}},
		{"equals", "/search?q={q}", []any{"x=1"}, url.Values{"q": {"x=1"}}},
		{"plus", "/search?q={q}", []any{"c+d"}, url.Values{"q": {"c+d"}}},
		{"percent", "/search?q={q}", []any{"50%"}, url.Values{"q": {"50%"}}},
		{"hash", "/search?q={q}", []any{"a#b"}, url.Values{"q": {"a#b"}}},
		{"space", "/search?q={q}", []any{"Gas Lift"}, url.Values{"q": {"Gas Lift"}}},
		{"non-ascii", "/search?q={q}", []any{"مرحبا é"}, url.Values{"q": {"مرحبا é"}}},
		{"two values", "/search?q={q}&filter={f}", []any{map[string]any{"q": "foo&bar", "f": "a=b"}},
			url.Values{"q": {"foo&bar"}, "filter": {"a=b"}}},
		{"inside a longer value", "/search?q=pre-{x}", []any{"a&b"}, url.Values{"q": {"pre-a&b"}}},
		{"as a key", "/search?{k}=1", []any{"a&b"}, url.Values{"a&b": {"1"}}},
		{"empty", "/search?q={q}", []any{""}, url.Values{"q": {""}}},
		{"not a string", "/search?q={q}", []any{42}, url.Values{"q": {"42"}}},
		{"wildcard in query", "/search?p={p...}", []any{"a/b&c"}, url.Values{"p": {"a/b&c"}}},
		{"after {$}", "/{$}?q={q}", []any{"a b&c"}, url.Values{"q": {"a b&c"}}},
		{"caller pre-escaped", "/alerts?category={c}", []any{url.QueryEscape("Health & Safety")},
			url.Values{"category": {"Health+%26+Safety"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatPathSegments(context.Background(), tt.pattern, tt.args...)
			if err != nil {
				t.Fatalf("formatPathSegments: %v", err)
			}
			d := decodeURL(t, got)
			if diff := cmp.Diff(tt.query, d.Query); diff != "" {
				t.Errorf("%s: query mismatch (-want +got):\n%s", got, diff)
			}
			if d.Fragment != "" {
				t.Errorf("%s: fragment = %q, want none", got, d.Fragment)
			}
		})
	}
}

func TestFormatPathSegments_spaceEncodedAlikeInEveryPart(t *testing.T) {
	got, err := formatPathSegments(context.Background(), "/p/{a}?q={b}#{c}", "a b", "a b", "a b")
	if err != nil {
		t.Fatalf("formatPathSegments: %v", err)
	}
	if want := "/p/a%20b?q=a%20b#a%20b"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatPathSegments_pathAndFragmentRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		args    []any
		want    decodedURL
		raw     string // exact output, where the path encoding is pinned
	}{
		{name: "path space", pattern: "/users/{id}", args: []any{"a b"},
			raw: "/users/a%20b"},
		{name: "path slash stays one segment", pattern: "/users/{id}", args: []any{"a/b"},
			raw: "/users/a%2Fb"},
		{name: "path question mark", pattern: "/users/{id}", args: []any{"a?b"},
			want: decodedURL{Segments: []string{"users", "a?b"}, Query: url.Values{}}},
		{name: "path hash", pattern: "/users/{id}", args: []any{"a#b"},
			want: decodedURL{Segments: []string{"users", "a#b"}, Query: url.Values{}}},
		{name: "path plus is literal", pattern: "/users/{id}", args: []any{"c+d"},
			raw: "/users/c+d"},
		{name: "wildcard keeps slashes", pattern: "/files/{path...}", args: []any{"a b/c&d"},
			want: decodedURL{Segments: []string{"files", "a b", "c&d"}, Query: url.Values{}}},
		{name: "path and query", pattern: "/users/{id}?tab={t}", args: []any{"a&b", "x&y"},
			want: decodedURL{Segments: []string{"users", "a&b"}, Query: url.Values{"tab": {"x&y"}}}},
		{name: "fragment", pattern: "/docs/{s}#{a}", args: []any{"api", "a b"},
			want: decodedURL{Segments: []string{"docs", "api"}, Query: url.Values{}, Fragment: "a b"}},
		{name: "path, query and fragment", pattern: "/docs/{s}?q={q}#{a}", args: []any{"api", "a#b", "c&d"},
			want: decodedURL{Segments: []string{"docs", "api"}, Query: url.Values{"q": {"a#b"}}, Fragment: "c&d"}},
		{name: "question mark inside the fragment", pattern: "/docs#x?{a}", args: []any{"c&d"},
			want: decodedURL{Segments: []string{"docs"}, Query: url.Values{}, Fragment: "x?c&d"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatPathSegments(context.Background(), tt.pattern, tt.args...)
			if err != nil {
				t.Fatalf("formatPathSegments: %v", err)
			}
			if tt.raw != "" {
				if got != tt.raw {
					t.Errorf("got %q, want %q", got, tt.raw)
				}
				return
			}
			if diff := cmp.Diff(tt.want, decodeURL(t, got)); diff != "" {
				t.Errorf("%s: mismatch (-want +got):\n%s", got, diff)
			}
		})
	}
}

func TestFormatPathSegments_queryValueEveryArgForm(t *testing.T) {
	for name, args := range map[string][]any{
		"map":        {map[string]any{"q": "A&B"}},
		"positional": {"A&B"},
		"pairs":      {"q", "A&B"},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := formatPathSegments(context.Background(), "/search?q={q}", args...)
			if err != nil {
				t.Fatalf("formatPathSegments: %v", err)
			}
			if diff := cmp.Diff(url.Values{"q": {"A&B"}}, decodeURL(t, got).Query); diff != "" {
				t.Errorf("%s: query mismatch (-want +got):\n%s", got, diff)
			}
		})
	}
}

// Values filled from the current request come from r.PathValue, already
// decoded, so URLFor must escape them like values the caller passes.
func TestFormatPathSegments_requestValuesEscaped(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		params  map[string]string
		args    []any
		want    decodedURL
	}{
		{name: "path value with reserved characters", pattern: "/items/{id}",
			params: map[string]string{"id": "a/b?c#d"},
			want:   decodedURL{Segments: []string{"items", "a/b?c#d"}, Query: url.Values{}}},
		{name: "wildcard keeps slashes", pattern: "/files/{path...}",
			params: map[string]string{"path": "a b/c?d"},
			want:   decodedURL{Segments: []string{"files", "a b", "c?d"}, Query: url.Values{}}},
		{name: "mixed with a passed query value", pattern: "/items/{id}?q={q}",
			params: map[string]string{"id": "a&b?"}, args: []any{map[string]any{"q": "x&y"}},
			want: decodedURL{Segments: []string{"items", "a&b?"}, Query: url.Values{"q": {"x&y"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := urlParamsCtx.WithValue(context.Background(), tt.params)
			got, err := formatPathSegments(ctx, tt.pattern, tt.args...)
			if err != nil {
				t.Fatalf("formatPathSegments: %v", err)
			}
			if diff := cmp.Diff(tt.want, decodeURL(t, got)); diff != "" {
				t.Errorf("%s: mismatch (-want +got):\n%s", got, diff)
			}
		})
	}
}

type escItemPage struct{}

func (escItemPage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, err := URLFor(r.Context(), []any{escItemPage{}, "?q={q}"}, map[string]any{"q": "A&B"})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	_, _ = w.Write([]byte(u))
}

func TestURLFor_requestValuesRoundTrip(t *testing.T) {
	type pages struct {
		item escItemPage `route:"/items/{id} Item"`
	}
	for _, tt := range []struct {
		name     string
		opts     []Option
		path     string
		wantPath string
		want     decodedURL
	}{
		{name: "escaped request value", path: "/items/a%26b%2Fc%3Fd",
			want: decodedURL{Segments: []string{"items", "a&b/c?d"}, Query: url.Values{"q": {"A&B"}}}},
		{name: "under WithURLPrefix", opts: []Option{WithURLPrefix("/admin")}, path: "/items/a%20b",
			wantPath: "/admin/items/a%20b",
			want:     decodedURL{Segments: []string{"admin", "items", "a b"}, Query: url.Values{"q": {"A&B"}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			if _, err := Mount(mux, &pages{}, "/", "Test", tt.opts...); err != nil {
				t.Fatalf("Mount: %v", err)
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, http.NoBody))
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			got := rec.Body.String()
			if tt.wantPath != "" && !strings.HasPrefix(got, tt.wantPath+"?") {
				t.Errorf("got %q, want path %q", got, tt.wantPath)
			}
			if diff := cmp.Diff(tt.want, decodeURL(t, got)); diff != "" {
				t.Errorf("%s: mismatch (-want +got):\n%s", got, diff)
			}
		})
	}
}

// A template composed per request, such as []any{page, "?" + v.Encode()},
// is distinct per value set, so URLFor must not cache it: only Mount adds
// to the segment cache, once per route.
func TestURLFor_composedTemplatesNotCached(t *testing.T) {
	type pages struct {
		item escItemPage `route:"/items/{id} Item"`
	}
	sp, err := Mount(http.NewServeMux(), &pages{}, "/", "Test")
	if err != nil {
		t.Fatalf("Mount: %v", err)
	}
	pc := sp.pc
	before := len(pc.segmentCache)
	ctx := pcCtx.WithValue(context.Background(), pc)
	for i := range 100 {
		v := url.Values{"n": {strings.Repeat("x", i)}}
		if _, err := URLFor(ctx, []any{escItemPage{}, "?" + v.Encode()}, "1"); err != nil {
			t.Fatalf("URLFor: %v", err)
		}
	}
	if got, err := URLFor(ctx, escItemPage{}, "a b"); err != nil || got != "/items/a%20b" {
		t.Fatalf("URLFor = %q, %v; want /items/a%%20b", got, err)
	}
	if after := len(pc.segmentCache); after != before {
		t.Errorf("segment cache grew from %d to %d entries", before, after)
	}
}
