package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// cacheEntries counts the results stored in the cache directory.
func cacheEntries(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir(cacheDir())
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "tmp-") {
			n++
		}
	}
	return n
}

// TestRun_tagSetsAndCache lints testdata/buildtags, whose finding only
// exists in the default build, under the default and devtools builds in
// one run, then checks the cache: an unchanged rerun reuses both
// results, and touching a source file makes the next run analyse again.
func TestRun_tagSetsAndCache(t *testing.T) {
	t.Setenv("STRUCTPAGES_LINT_CACHE", t.TempDir())
	src, err := filepath.Abs("../../testdata/buildtags")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(src)
	const want = "dev.go:16:33: [urlfor] URLFor: no page mounted for type devPage\n"

	lint := func() (string, int) {
		var out strings.Builder
		code := run(&out, []string{"", "devtools"}, true, nil)
		return out.String(), code
	}
	out, code := lint()
	if code != 1 || !strings.HasSuffix(out, want) || strings.Count(out, "\n") != 1 {
		t.Fatalf("first run: exit %d, output %q; want exit 1 and one finding ending %q", code, out, want)
	}
	if n := cacheEntries(t); n != 2 {
		t.Fatalf("after the first run the cache holds %d results, want one per build (2)", n)
	}

	if out2, code2 := lint(); out2 != out || code2 != code {
		t.Fatalf("cached run: exit %d, output %q; want the first run's", code2, out2)
	}
	if n := cacheEntries(t); n != 2 {
		t.Fatalf("an unchanged rerun stored new results: cache holds %d, want 2", n)
	}

	// Touching a file changes nothing; changing its content does. The
	// added comment leaves the finding's position as it was.
	dev := filepath.Join(src, "dev.go")
	orig, err := os.ReadFile(dev)
	if err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(dev, later, later); err != nil {
		t.Fatal(err)
	}
	if _, _ = lint(); cacheEntries(t) != 2 {
		t.Fatalf("touching a file invalidated the cache: it holds %d results, want 2", cacheEntries(t))
	}
	if err := os.WriteFile(dev, append(orig, []byte("\n// edited\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.WriteFile(dev, orig, 0o644) })
	if out3, code3 := lint(); out3 != out || code3 != code {
		t.Fatalf("rerun after a change: exit %d, output %q; want the first run's", code3, out3)
	}
	if n := cacheEntries(t); n != 4 {
		t.Fatalf("after a change the cache holds %d results, want 4 (two new)", n)
	}

	// -cache=false neither reads nor writes the cache.
	var out4 strings.Builder
	if code4 := run(&out4, []string{""}, false, nil); code4 != 1 || out4.String() != out {
		t.Fatalf("uncached run: exit %d, output %q", code4, out4.String())
	}
	if n := cacheEntries(t); n != 4 {
		t.Fatalf("-cache=false wrote to the cache: it holds %d, want 4", n)
	}
}

// TestSortedFindings pins the output order: by file, then numerically
// by line and column, then by text.
func TestSortedFindings(t *testing.T) {
	in := []string{
		"b.go:2:1: [x] one\n",
		"a.go:105:3: [x] late\n",
		"a.go:23:9: [x] second\n",
		"a.go:23:10: [x] third\n",
		"a.go:23:9: [a] first\n",
		`C:\src\a.go:1:1: [x] drive` + "\n",
	}
	lines := map[string]bool{}
	for _, l := range in {
		lines[l] = true
	}
	want := []string{
		`C:\src\a.go:1:1: [x] drive` + "\n",
		"a.go:23:9: [a] first\n",
		"a.go:23:9: [x] second\n",
		"a.go:23:10: [x] third\n",
		"a.go:105:3: [x] late\n",
		"b.go:2:1: [x] one\n",
	}
	if got := sortedFindings(lines); !slices.Equal(got, want) {
		t.Errorf("sortedFindings:\n got %q\nwant %q", got, want)
	}
}
