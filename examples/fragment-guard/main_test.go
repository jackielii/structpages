package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackielii/structpages"
)

func serve(t *testing.T, mux http.Handler, method, target string, headers map[string]string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

var (
	native  = map[string]string{"Sec-Fetch-Mode": "navigate", "Sec-Fetch-Dest": "document"}
	boosted = map[string]string{"HX-Request": "true", "HX-Boosted": "true"}
)

func htmxTo(target string) map[string]string {
	return map[string]string{"HX-Request": "true", "HX-Target": target}
}

func TestLanding(t *testing.T) {
	mux, err := newMux(NewStore())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		path     string
		headers  map[string]string
		status   int
		location string // Location, or HX-Location for a boosted request
		body     string
	}{
		// Default: the ancestor page.
		{name: "task list, new tab", path: "/projects/42/tasks", headers: native, status: 303, location: "/projects/42/"},
		{name: "task list, boosted link", path: "/projects/42/tasks", headers: boosted, status: 200, location: "/projects/42/"},
		{name: "task list, partial", path: "/projects/42/tasks", headers: htmxTo("projects-tasks-list"), status: 200, body: "Write the launch post"},
		{name: "rename dialog, new tab", path: "/projects/42/tasks/7/rename", headers: native, status: 303, location: "/projects/42/"},

		// Landing: built from the query.
		{name: "comments, new tab", path: "/comments?project=42&task=7", headers: native, status: 303, location: "/projects/42/?task=7#comments"},
		{name: "comments, partial", path: "/comments?project=42&task=7", headers: htmxTo("comments"), status: 200, body: "First draft"},

		// SelfLanding: Props looks the mention up.
		{name: "mention, new tab", path: "/mentions/1", headers: native, status: 303, location: "/projects/42/?task=7#comments"},
		{name: "mention, partial", path: "/mentions/1", headers: htmxTo("mention-preview"), status: 200, body: "ship it"},
		{name: "someone else's mention, new tab", path: "/mentions/3", headers: native, status: 404, body: "<html"},
		{name: "someone else's mention, partial", path: "/mentions/3", headers: htmxTo("mention-preview"), status: 404, body: "Not Found"},

		// Access comes after landing: a missing project still lands, and
		// the project page answers 404.
		{name: "unknown project's tasks, new tab", path: "/projects/99/tasks", headers: native, status: 303, location: "/projects/99/"},
		{name: "unknown project's tasks, partial", path: "/projects/99/tasks", headers: htmxTo("projects-tasks-list"), status: 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(t, mux, http.MethodGet, tt.path, tt.headers, nil)
			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d\n%s", rec.Code, tt.status, rec.Body)
			}
			loc := rec.Header().Get("Location")
			if tt.headers["HX-Boosted"] == "true" {
				loc = rec.Header().Get("HX-Location")
			}
			if loc != tt.location {
				t.Errorf("location = %q, want %q", loc, tt.location)
			}
			if !strings.Contains(rec.Body.String(), tt.body) {
				t.Errorf("body does not contain %q:\n%s", tt.body, rec.Body)
			}
			if tt.status == 200 && tt.body != "" && rec.Header().Get("HX-Push-Url") != "false" {
				t.Errorf("fragment partial without HX-Push-Url: false")
			}
		})
	}
}

func TestRename(t *testing.T) {
	mux, err := newMux(NewStore())
	if err != nil {
		t.Fatal(err)
	}
	rec := serve(t, mux, http.MethodPost, "/projects/42/tasks/7/rename", htmxTo("dialog"), url.Values{"title": {""}})
	if !strings.Contains(rec.Body.String(), "A task needs a title.") {
		t.Errorf("empty title: want the dialog with its problem, got\n%s", rec.Body)
	}
	rec = serve(t, mux, http.MethodPost, "/projects/42/tasks/7/rename", htmxTo("dialog"), url.Values{"title": {"Publish the post"}})
	if body := rec.Body.String(); !strings.Contains(body, "Publish the post") || !strings.Contains(body, `hx-swap-oob="true"`) {
		t.Errorf("rename: want the task list swapped out of band, got\n%s", body)
	}
}

// TestGETNavigationsAreDocuments is the route walk: it opens every GET route
// as a document, the way a new tab would, and fails on bare markup, redirect
// loops and off-origin redirects. Lint catches wrong shapes as they're
// written; this catches what it can't see.
func TestGETNavigationsAreDocuments(t *testing.T) {
	var routes []string
	collect := func(next http.Handler, pn *structpages.PageNode) http.Handler {
		if pn.Method == http.MethodGet && (pn.Props != nil || len(pn.Components) > 0) {
			routes = append(routes, pn.FullRoute())
		}
		return next
	}
	mux := http.NewServeMux()
	if _, err := structpages.Mount(mux, pages{}, "/", "Home",
		structpages.WithTargetSelector(structpages.HTMXRenderTarget),
		structpages.WithErrorHandler(errorHandler),
		structpages.WithArgs(NewStore()),
		structpages.WithMiddlewares(collect, NavigationGuard),
	); err != nil {
		t.Fatal(err)
	}
	fixtures := strings.NewReplacer("{project}", "42", "{task}", "7", "{id}", "1", "{$}", "")
	query := map[string]string{"/comments": "?project=42&task=7"}

	if len(routes) < 6 {
		t.Fatalf("collected %d GET routes, want every page and fragment: %q", len(routes), routes)
	}
	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			path := fixtures.Replace(route) + query[route]
			seen := map[string]bool{}
			for hops := 0; ; hops++ {
				if hops == 5 || seen[path] {
					t.Fatalf("redirect loop or limit at %s", path)
				}
				seen[path] = true
				rec := serve(t, mux, http.MethodGet, path, native, nil)
				if rec.Code/100 == 3 {
					to, err := url.Parse(rec.Header().Get("Location"))
					if err != nil || to.Host != "" {
						t.Fatalf("redirect off the origin: %q", rec.Header().Get("Location"))
					}
					to.Fragment = ""
					path = to.String()
					continue
				}
				if !strings.Contains(rec.Body.String(), "<html") || !strings.Contains(rec.Body.String(), `id="content"`) {
					t.Fatalf("%s answered a document request with bare markup:\n%s", path, rec.Body)
				}
				return
			}
		})
	}
}

// The guard here is the htmx example's, unchanged. Keep the copies equal.
func TestGuardMatchesHTMXExample(t *testing.T) {
	ours, err := os.ReadFile("guard.go")
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := os.ReadFile("../htmx/navguard.go")
	if err != nil {
		t.Fatal(err)
	}
	if string(ours) != string(theirs) {
		t.Error("guard.go differs from ../htmx/navguard.go; copy the change across")
	}
}
