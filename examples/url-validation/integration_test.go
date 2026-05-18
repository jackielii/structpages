package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackielii/structpages"
)

// Integration-style "URL inventory" test: mount the full app, render
// each page, and assert the response body contains every URL we expect
// it to emit. This is the "no dangling URLs" guard — it catches:
//
//   - A renamed field on a parent struct (chain step fails to resolve,
//     helper returns an error, body contains "error: ..." instead of
//     the expected URL).
//   - A renamed route in a route tag (the rendered URL drifts; we
//     assert against the new path explicitly so the diff is visible).
//   - A new page type accidentally introducing ambiguity (strict URLFor
//     errors, the rendering path surfaces it).
//   - A helper added without a call site, or a call site bypassing
//     helpers (the body simply lacks the URL we expected).
//
// Run as part of CI (`go test ./...`) to keep URL drift out of
// production traffic.
func TestRenderedURLsResolve(t *testing.T) {
	mux := http.NewServeMux()
	if _, err := structpages.Mount(mux, &root{}, "/", "url-validation demo"); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	cases := []struct {
		name string
		path string
		// wantContains is every literal substring that must appear in
		// the rendered body. URLs go here, not page-text fragments.
		wantContains []string
	}{
		{
			name: "home lists all three group indexes (chain, composition, and Ref sections)",
			path: "/",
			wantContains: []string{
				// chain form
				`href="/foundations/"`,
				`href="/components/"`,
				`href="/patterns/"`,
				// composition form (chain + query fragment)
				`href="/foundations/?tab=overview"`,
				`href="/components/?tab=overview"`,
				`href="/patterns/?tab=overview"`,
				// Ref form caption — confirms the Ref code path ran
				`Ref "Foundations.Index"`,
			},
		},
		{
			name: "foundations index lists entries + home link",
			path: "/foundations/",
			wantContains: []string{
				`href="/"`,
				`href="/foundations/colors"`,
				`href="/foundations/typography"`,
				`href="/foundations/spacing"`,
			},
		},
		{
			name: "components index lists entries + home link",
			path: "/components/",
			wantContains: []string{
				`href="/"`,
				`href="/components/button"`,
				`href="/components/input"`,
				`href="/components/dialog"`,
			},
		},
		{
			name: "patterns index lists entries + home link",
			path: "/patterns/",
			wantContains: []string{
				`href="/"`,
				`href="/patterns/form-layout"`,
				`href="/patterns/data-table"`,
			},
		},
		{
			name: "components detail back-links to its own group index",
			path: "/components/button",
			wantContains: []string{
				`href="/components/"`,
			},
		},
		{
			name: "patterns detail back-links to its own group index (NOT foundations)",
			path: "/patterns/form-layout",
			wantContains: []string{
				`href="/patterns/"`,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d, body: %s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			// A helper that fails returns the error inline as
			// "error: ...". If we see that string, something drifted.
			if strings.Contains(body, "error:") {
				t.Errorf("body contains helper error: %s", body)
			}
			for _, want := range tc.wantContains {
				if !strings.Contains(body, want) {
					t.Errorf("body missing %q\n--- full body ---\n%s", want, body)
				}
			}
		})
	}
}

// TestValidateURLs runs the same validator main.go runs at startup,
// so CI catches any dangling URL in the validator's inventory even
// without booting the binary. The integration test above is the
// end-to-end signal; this one is the cheap, fast guard.
func TestValidateURLs(t *testing.T) {
	sp, err := structpages.Mount(http.NewServeMux(), &root{}, "/", "url-validation demo")
	if err != nil {
		t.Fatalf("Mount: %v", err)
	}
	if err := validateURLs(sp); err != nil {
		t.Errorf("validateURLs reported dangling URLs:\n%v", err)
	}
}

// TestPatternsDetailDoesNotLeakToOtherGroups is the regression guard
// for the exact bug the chain form was built to prevent: a detail page
// in one group must NOT generate links pointing at another group's
// URL space. Would have failed loudly against the pre-strict-mode
// framework.
func TestPatternsDetailDoesNotLeakToOtherGroups(t *testing.T) {
	mux := http.NewServeMux()
	if _, err := structpages.Mount(mux, &root{}, "/", "url-validation demo"); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/patterns/form-layout", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, wrongPrefix := range []string{`href="/foundations/`, `href="/components/`} {
		if strings.Contains(body, wrongPrefix) {
			t.Errorf("patterns detail leaked a %s link:\n%s", wrongPrefix, body)
		}
	}
}
