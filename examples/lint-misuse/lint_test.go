package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestLintMisuse builds tools/lint/cmd/structpages-lint, runs it against this
// fixture, and snapshot-compares the output. It pins the exact
// diagnostic wording so any drift in messages requires an
// intentional update to the want literal below.
//
// Run with: go test ./examples/lint-misuse/ -run TestLintMisuse
func TestLintMisuse(t *testing.T) {
	bin := buildLinter(t)

	cmd := exec.Command(bin, "./...")
	cmd.Dir = "."
	out, _ := cmd.CombinedOutput()
	got := normaliseOutput(string(out))

	want := strings.TrimSpace(`
pages.go:LINE:COL: [ref] Ref "Items.NoSuch": segment 1 ("NoSuch") not found as child of "Items"; available: Detail, Index
pages.go:LINE:COL: [ref] Ref "/missing": no page with this route. Did you rename a route tag? Known routes: /, /items, /items/{$}, /items/{slug}, /{$}
pages.go:LINE:COL: [ref] Ref "Ghost": no page with this name; known names include: Detail, Home, Index, Items, itemsRoot, root
pages.go:LINE:COL: [urlfor] URLFor chain: parent Items has no child of type homePage; available: Index (itemsIndex), Detail (itemDetail)
pages.go:LINE:COL: [urlfor] URLFor: typed value at slice position 2 follows a string fragment; chain steps must all come before any string fragment
pages.go:LINE:COL: [params] URLFor: param "wrong" does not appear in pattern "/items/{slug}" (known: slug)
pages.go:LINE:COL: [idfor] IDTarget: method expression unmountedPage.Title: receiver type unmountedPage is not mounted as a page
pages.go:LINE:COL: [ref] Ref "Detail.Index": segment 1 ("Index") not found as child of "Detail"; available:
pages.go:LINE:COL: [idfor] IDTarget: method "Nope" not found on chain leaf "Detail"; available: Page, Stats
pages.go:LINE:COL: [params] URLFor: param "wrongTab" does not appear in pattern "/items/{slug}?tab={tab}" (known: slug, tab)
pages.go:LINE:COL: [ref] Ref "Items.NoSuch": segment 1 ("NoSuch") not found as child of "Items"; available: Detail, Index
pages.templ:LINE:COL: [url-attr] href value "/login" is a hard-coded internal URL; use structpages.URLFor instead
pages.templ:LINE:COL: [url-attr] href value "/admin" is a hard-coded internal URL; use structpages.URLFor instead
pages.templ:LINE:COL: [url-attr] href value ` + "`" + `"/items/" + strconv.Itoa(id)` + "`" + ` builds an internal URL by string concatenation; use structpages.URLFor instead
pages.templ:LINE:COL: [url-attr] href value ` + "`" + `fmt.Sprintf("/users/%s", name)` + "`" + ` builds an internal URL via fmt.Sprint*; use structpages.URLFor instead
pages.templ:LINE:COL: [url-attr] hx-get value "/api/items" is a hard-coded internal URL; use structpages.URLFor instead
pages.templ:LINE:COL: [url-attr] action value "/submit" is a hard-coded internal URL; use structpages.URLFor instead
`)
	if got != want {
		t.Errorf("linter output mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// buildLinter compiles cmd/structpages-lint into a temp file and
// returns the path. The build runs in the tools/lint sub-module
// (../../tools/lint) because the linter and the example live in
// separate modules.
func buildLinter(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "structpages-lint")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", out, "./cmd/structpages-lint")
	cmd.Dir = filepath.Join("..", "..", "tools", "lint")
	if buf, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, buf)
	}
	return out
}

// normaliseOutput strips the absolute path prefix, the line:col
// positions, and trailing whitespace so the snapshot only pins the
// stable parts of each diagnostic. We deliberately discard column
// positions because they shift when the fixture is reformatted.
func normaliseOutput(s string) string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimRight(line, "\r ")
		if line == "" {
			continue
		}
		// Drop absolute path prefix up to "pages.go" / "pages.templ".
		if i := strings.Index(line, "pages.go"); i > 0 {
			line = line[i:]
		} else if i := strings.Index(line, "pages.templ"); i > 0 {
			line = line[i:]
		}
		// Replace ":N:M:" with ":LINE:COL:".
		line = stripPos(line)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// stripPos rewrites the first ":N:M:" run after the filename to a
// stable placeholder.
func stripPos(line string) string {
	parts := strings.SplitN(line, ":", 4)
	if len(parts) < 4 {
		return line
	}
	return fmt.Sprintf("%s:LINE:COL:%s", parts[0], parts[3])
}
