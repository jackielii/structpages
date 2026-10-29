package structpages

import (
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Underscores and hyphens separate words, so an id holds single hyphens
// between words and none at its ends; letters lowercase whatever their
// script.
func TestCamelToKebab_separatorsAndUnicode(t *testing.T) {
	for in, want := range map[string]string{
		"Row_Edit":    "row-edit",
		"row_edit":    "row-edit",
		"HTTP_Status": "http-status",
		"A__B":        "a-b",
		"Foo_":        "foo",
		"_Foo":        "foo",
		"Page2_Edit":  "page2-edit",
		"ui-Kit":      "ui-kit", // an import path element
		"my--lib":     "my-lib",
		"lib-":        "lib",
		"-lib":        "lib",
		"fn-fmt":      "fn-fmt",
		"Größe":       "größe",
		"ÜberPage":    "über-page",
		"pageÜber":    "page-über",
		"ÉTÉPage":     "été-page",
	} {
		if got := camelToKebab(in); got != want {
			t.Errorf("camelToKebab(%q) = %q, want %q", in, got, want)
		}
	}
}

// legacyCamelToKebab is camelToKebab before underscores, hyphens and
// non-ASCII letters were handled. It is correct for ASCII letters and
// digits, so ids of such names must not change.
func legacyCamelToKebab(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			if s[i-1] >= 'a' && s[i-1] <= 'z' {
				result.WriteByte('-')
			} else if i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z' {
				result.WriteByte('-')
			}
		}
		result.WriteRune(r | ('a' - 'A'))
	}
	return result.String()
}

func TestCamelToKebab_asciiAlphanumericUnchanged(t *testing.T) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	rng := rand.New(rand.NewPCG(1, 2))
	for range 100000 {
		b := make([]byte, 1+rng.IntN(12))
		for i := range b {
			b[i] = alphabet[rng.IntN(len(alphabet))]
		}
		s := string(b)
		if got, want := camelToKebab(s), legacyCamelToKebab(s); got != want {
			t.Fatalf("camelToKebab(%q) = %q, was %q", s, got, want)
		}
	}
}

type snakePage struct{}

func (snakePage) Page() component     { return testComponent{"SNAKE-PAGE"} }
func (snakePage) Row_Edit() component { return testComponent{"SNAKE-ROW-EDIT"} }

type snakeRoot struct {
	Snake_Page snakePage `route:"/snake Snake"`
}

// A page or component named with an underscore gets a readable id, and a
// request for it selects the component.
func TestID_underscoreNames(t *testing.T) {
	mux := http.NewServeMux()
	sp, err := Mount(mux, snakeRoot{}, "/", "App", WithTargetSelector(HTMXRenderTarget))
	if err != nil {
		t.Fatalf("Mount: %v", err)
	}
	id, err := sp.ID(snakePage.Row_Edit)
	if err != nil {
		t.Fatalf("sp.ID: %v", err)
	}
	if id != "snake-page-row-edit" {
		t.Errorf("sp.ID(snakePage.Row_Edit) = %q, want %q", id, "snake-page-row-edit")
	}
	req := httptest.NewRequest(http.MethodGet, "/snake", http.NoBody)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", id)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if got := rec.Body.String(); got != "SNAKE-ROW-EDIT" {
		t.Errorf("HX-Target %q rendered %q, want SNAKE-ROW-EDIT", id, got)
	}
}

type snakeTwinPage struct{}

func (snakeTwinPage) Page() component     { return testComponent{"TWIN-PAGE"} }
func (snakeTwinPage) RowEdit() component  { return testComponent{"ROW-EDIT"} }
func (snakeTwinPage) Row_Edit() component { return testComponent{"ROW_EDIT"} }

// Names that kebab-case alike can't share a page: Mount reports the id
// both would get instead of rendering duplicate ids.
func TestID_namesKebabCasingAlikeAreAnError(t *testing.T) {
	_, err := Mount(http.NewServeMux(), struct {
		Twin snakeTwinPage `route:"/twin Twin"`
	}{}, "/", "App")
	if err == nil || !strings.Contains(err.Error(), `"twin-row-edit"`) {
		t.Errorf("Mount error = %v, want one naming the id \"twin-row-edit\"", err)
	}
}
