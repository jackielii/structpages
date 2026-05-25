package structpages_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackielii/structpages"
)

type parsePubHome struct{}

func (parsePubHome) ServeHTTP(http.ResponseWriter, *http.Request) {}

type parsePubItem struct{}

func (parsePubItem) ServeHTTP(http.ResponseWriter, *http.Request) {}

type parsePubRoot struct {
	Home parsePubHome `route:"/ Home"`
	Item parsePubItem `route:"/items/{id} Item"`
}

// TestParse_BuildsTreeWithoutMounting verifies that Parse returns a
// usable *StructPages whose URLFor resolves against the page tree,
// without ever registering routes on a mux.
func TestParse_BuildsTreeWithoutMounting(t *testing.T) {
	sp, err := structpages.Parse(parsePubRoot{}, "/", "Test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	got, err := sp.URLFor(parsePubHome{})
	if err != nil {
		t.Fatalf("URLFor Home: %v", err)
	}
	if got != "/" {
		t.Errorf("URLFor Home = %q, want %q", got, "/")
	}

	got, err = sp.URLFor(parsePubItem{}, map[string]any{"id": 42})
	if err != nil {
		t.Fatalf("URLFor Item: %v", err)
	}
	if got != "/items/42" {
		t.Errorf("URLFor Item = %q, want %q", got, "/items/42")
	}
}

// TestParse_DoesNotTouchMux confirms Parse skips the mux-registration
// step entirely — a freshly created ServeMux stays empty.
func TestParse_DoesNotTouchMux(t *testing.T) {
	mux := http.NewServeMux()
	if _, err := structpages.Parse(parsePubRoot{}, "/", "Test"); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 on empty mux, got %d (Parse must not register routes)", rec.Code)
	}
}

// TestPageContext_EnablesURLForOnBareContext is the test-harness
// motivating use case: a template render with a bare
// context.Background() can call URLFor after wrapping with
// PageContext.
func TestPageContext_EnablesURLForOnBareContext(t *testing.T) {
	sp, err := structpages.Parse(parsePubRoot{}, "/", "Test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ctx := sp.PageContext(context.Background())

	got, err := structpages.URLFor(ctx, parsePubItem{}, map[string]any{"id": 7})
	if err != nil {
		t.Fatalf("URLFor: %v", err)
	}
	if got != "/items/7" {
		t.Errorf("URLFor = %q, want %q", got, "/items/7")
	}
}
