package main

import (
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
		location string // Location, or HX-Location for an htmx full request
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
			name: "fragment, htmx full request", path: "/contact/details",
			headers: map[string]string{"HX-Request": "true", "HX-Request-Type": "full"},
			status:  200, location: "/contact", noStore: true,
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
			if tt.headers["HX-Request-Type"] == "full" {
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
			if !strings.Contains(rec.Header().Get("Vary"), "HX-Request") {
				t.Errorf("Vary = %q, want it to name HX-Request", rec.Header().Get("Vary"))
			}
		})
	}
}
