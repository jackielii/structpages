package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackielii/structpages"
)

func TestNavigationGuard(t *testing.T) {
	mux := http.NewServeMux()
	if _, err := structpages.Mount(mux, index{}, "/", "index",
		structpages.WithTargetSelector(structpages.HTMXRenderTarget),
		structpages.WithErrorHandler(errorHandler),
		structpages.WithMiddlewares(NavigationGuard),
	); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		path     string
		headers  map[string]string
		status   int
		location string // Location, or HX-Location when hxLoc
		hxLoc    bool
		body     string
		noStore  bool
	}{
		{
			name: "page, document", path: "/contact",
			status: 200, body: "<html",
		},
		{
			name: "page region, partial", path: "/contact",
			headers: map[string]string{"HX-Request": "true", "HX-Target": "content"},
			status:  200, body: "Contact Page", noStore: true,
		},
		{
			name: "fragment, partial", path: "/contact/details",
			headers: map[string]string{"HX-Request": "true", "HX-Target": "details"},
			status:  200, body: "Email us", noStore: true,
		},
		{
			name: "fragment, plain navigation", path: "/contact/details",
			status: 303, location: "/contact", noStore: true,
		},
		{
			name: "fragment, navigation with stale htmx headers", path: "/contact/details",
			headers: map[string]string{"Sec-Fetch-Mode": "navigate", "HX-Request": "true", "HX-Target": "details"},
			status:  303, location: "/contact", noStore: true,
		},
		{
			name: "fragment, htmx history restore", path: "/contact/details",
			headers: map[string]string{"HX-Request": "true", "HX-History-Restore-Request": "true"},
			status:  303, location: "/contact", noStore: true,
		},
		{
			name: "fragment, htmx 4 swap of the body", path: "/contact/details",
			headers: map[string]string{"HX-Request": "true", "HX-Request-Type": "full", "HX-Target": "body"},
			status:  200, location: "/contact", hxLoc: true, noStore: true,
		},
		{
			name: "fragment, htmx 2 boosted link", path: "/contact/details",
			headers: map[string]string{"HX-Request": "true", "HX-Boosted": "true"},
			status:  200, location: "/contact", hxLoc: true, noStore: true,
		},
		{
			name: "fragment, htmx 4 hx-select swap into an element is a partial", path: "/contact/details",
			headers: map[string]string{"HX-Request": "true", "HX-Request-Type": "full", "HX-Target": "div#details"},
			status:  200, body: "Email us", noStore: true,
		},
		{
			name: "error fragment, navigation lands on root", path: "/throw",
			status: 303, location: "/", noStore: true,
		},
		{
			name: "error fragment, partial", path: "/throw",
			headers: map[string]string{"HX-Request": "true", "HX-Target": "content"},
			status:  200, body: "this is an error", noStore: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d", rec.Code, tt.status)
			}
			loc := rec.Header().Get("Location")
			if tt.hxLoc {
				loc = rec.Header().Get("HX-Location")
			}
			if loc != tt.location {
				t.Errorf("location = %q, want %q", loc, tt.location)
			}
			if !strings.Contains(rec.Body.String(), tt.body) {
				t.Errorf("body does not contain %q:\n%s", tt.body, rec.Body.String())
			}
			if got := rec.Header().Get("Cache-Control") == "private, no-store"; got != tt.noStore {
				t.Errorf("no-store = %v, want %v", got, tt.noStore)
			}
			if vary := rec.Header().Values("Vary"); len(vary) != 1 || strings.Count(vary[0], "HX-Request,") != 1 {
				t.Errorf("Vary = %q, want one header naming HX-Request once", vary)
			}
		})
	}
}

// cachedFragment opts into a private cache for its successful response.
type cachedFragment struct{}

func (cachedFragment) Props(w http.ResponseWriter) error {
	w.Header().Set("Cache-Control", "private, max-age=3600")
	return structpages.RenderComponent(textNode("bell"))
}

type textNode string

func (t textNode) Render(_ context.Context, w io.Writer) error {
	_, err := io.WriteString(w, string(t))
	return err
}

type cachedPages struct {
	bell cachedFragment `route:"GET /bell Bell"`
}

func (cachedPages) Page() textNode { return "home" }

func TestNavigationGuardFragmentCacheOptIn(t *testing.T) {
	mux := http.NewServeMux()
	if _, err := structpages.Mount(mux, cachedPages{}, "/", "App",
		structpages.WithTargetSelector(structpages.HTMXRenderTarget),
		// Another middleware's Vary is kept, and the guard's added once.
		structpages.WithMiddlewares(func(next http.Handler, _ *structpages.PageNode) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Add("Vary", "Accept-Encoding, HX-Request")
				next.ServeHTTP(w, r)
			})
		}, NavigationGuard),
	); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/bell", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", "bell")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if got := rec.Header().Get("Cache-Control"); got != "private, max-age=3600" {
		t.Errorf("partial Cache-Control = %q, want the fragment's own", got)
	}
	if vary := rec.Header().Values("Vary"); len(vary) != 1 ||
		!strings.HasPrefix(vary[0], "Accept-Encoding, HX-Request,") || strings.Count(vary[0], "HX-Request,") != 1 {
		t.Errorf("Vary = %q, want Accept-Encoding kept and each token once", vary)
	}

	// The navigation is redirected before Props runs: no-store, no opt-in.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/bell", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Cache-Control") != "private, no-store" {
		t.Errorf("navigation = %d, Cache-Control %q; want 303 and no-store", rec.Code, rec.Header().Get("Cache-Control"))
	}
}
