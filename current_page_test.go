package structpages

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fnComponent adapts a function to the component interface so a test page can
// inspect the render-time context.
type fnComponent func(context.Context, io.Writer) error

func (f fnComponent) Render(ctx context.Context, w io.Writer) error { return f(ctx, w) }

// cpLeaf renders the current page's FullRoute plus its parent's route, proving
// CurrentPage returns the matched leaf (not an ancestor) and that Parent walks
// the mount chain.
type cpLeaf struct{}

func (cpLeaf) Page() component {
	return fnComponent(func(ctx context.Context, w io.Writer) error {
		pn := CurrentPage(ctx)
		if pn == nil {
			_, err := io.WriteString(w, "nil")
			return err
		}
		out := pn.FullRoute()
		if pn.Parent != nil {
			out += "|parent=" + pn.Parent.Route
		}
		_, err := io.WriteString(w, out)
		return err
	})
}

type cpGroup struct {
	Leaf cpLeaf `route:"/leaf Leaf"`
}

type cpRoot struct {
	Group cpGroup `route:"/group Group"`
}

func TestCurrentPage_ReturnsMatchedLeafDuringRequest(t *testing.T) {
	mux := http.NewServeMux()
	if _, err := Mount(mux, &cpRoot{}, "/", "App"); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/group/leaf", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	const want = "/group/leaf|parent=/group"
	if got := rec.Body.String(); got != want {
		t.Errorf("CurrentPage in render: body = %q, want %q", got, want)
	}
}

func TestCurrentPage_NilOutsideRequest(t *testing.T) {
	if pn := CurrentPage(context.Background()); pn != nil {
		t.Errorf("bare context: CurrentPage = %v, want nil", pn)
	}

	// PageContext attaches the parse tree (for URLFor/ID) but not a matched
	// page, so CurrentPage is still nil there.
	sp, err := Parse(&cpRoot{}, "/", "App")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if pn := CurrentPage(sp.PageContext(context.Background())); pn != nil {
		t.Errorf("PageContext: CurrentPage = %v, want nil", pn)
	}
}
