package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestLintMisuse builds cmd/structpages-lint, runs it against this
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
pages.go:LINE:COL: [ref] Ref "Ghost": no page with this name; known names include: Detail, Home, Index, Items, root
pages.go:LINE:COL: [urlfor] URLFor chain: parent Items has no child of type homePage; available: Index (itemsIndex), Detail (itemDetail)
pages.go:LINE:COL: [urlfor] URLFor: typed value at slice position 2 follows a string fragment; chain steps must all come before any string fragment
pages.go:LINE:COL: [params] URLFor: param "wrong" does not appear in pattern "/items/{slug}" (known: slug)
pages.go:LINE:COL: [idfor] IDTarget: method expression unmountedPage.Title: receiver type unmountedPage is not mounted as a page
`)
	if got != want {
		t.Errorf("linter output mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// buildLinter compiles cmd/structpages-lint into a temp file and
// returns the path. The build runs in the main structpages module
// (../..) because the example's own go.sum does not have x/tools
// entries — they're not used by example code, only by the linter.
func buildLinter(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "structpages-lint")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", out, "./cmd/structpages-lint")
	cmd.Dir = filepath.Join("..", "..")
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
		// Drop absolute path prefix up to "pages.go".
		if i := strings.Index(line, "pages.go"); i > 0 {
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
