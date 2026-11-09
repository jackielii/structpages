package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackielii/structpages/tools/lint"
)

// The result cache lets a rerun over unchanged inputs skip the analysis,
// which dominates a run: the findings of a run are stored under a hash
// of everything that determines them, and printed again on a match.
//
// The key covers:
//   - every input the graph names (see lint.Graph.Fingerprint);
//   - this binary, by path, size and modification time, so a rebuilt or
//     upgraded linter never reuses a result;
//   - the working directory, the -tags flag and the patterns;
//   - the environment the go command reads to resolve packages.
//
// Load failures (exit code 2) are never cached.

// cacheVersion changes when the cache's format or meaning does.
const cacheVersion = "structpages-lint cache v1"

// cacheEnv are the environment variables that change which files the go
// command loads or how it type-checks them.
var cacheEnv = []string{"GOFLAGS", "GOOS", "GOARCH", "CGO_ENABLED", "GOEXPERIMENT", "GOWORK", "GOTOOLCHAIN", "GOROOT", "GOPATH"}

// cacheKey hashes the inputs of a run over graph; "" when they can't all
// be read, which disables the cache for this run.
func cacheKey(graph *lint.Graph, tags string, patterns []string) string {
	h := sha256.New()
	fmt.Fprintln(h, cacheVersion)
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	fi, err := os.Stat(exe)
	if err != nil {
		return ""
	}
	fmt.Fprintln(h, "exe", exe, fi.Size(), fi.ModTime().UnixNano())
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	fmt.Fprintln(h, "wd", wd)
	fmt.Fprintf(h, "tags %q\n", tags)
	fmt.Fprintf(h, "patterns %q\n", patterns)
	for _, name := range cacheEnv {
		fmt.Fprintf(h, "env %s=%q\n", name, os.Getenv(name))
	}
	if err := graph.Fingerprint(h); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// cacheDir is where results are stored: $STRUCTPAGES_LINT_CACHE, else
// structpages-lint in the user cache directory; "" when there is none.
func cacheDir() string {
	if dir := os.Getenv("STRUCTPAGES_LINT_CACHE"); dir != "" {
		return dir
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "structpages-lint")
}

// cacheGet returns the findings and exit code stored under key.
func cacheGet(key string) (out string, code int, ok bool) {
	dir := cacheDir()
	if dir == "" {
		return "", 0, false
	}
	name := filepath.Join(dir, key)
	b, err := os.ReadFile(name)
	if err != nil {
		return "", 0, false
	}
	// Mark it used, so cachePut's pruning keeps results in regular use.
	now := time.Now()
	os.Chtimes(name, now, now)
	first, rest, found := strings.Cut(string(b), "\n")
	code, err = strconv.Atoi(first)
	if !found || err != nil {
		return "", 0, false
	}
	return rest, code, true
}

// cacheMaxAge is how long an unused result is kept.
const cacheMaxAge = 7 * 24 * time.Hour

// cachePut stores the findings and exit code under key, best effort,
// and removes results older than cacheMaxAge.
func cachePut(key, out string, code int) {
	dir := cacheDir()
	if dir == "" || os.MkdirAll(dir, 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, "tmp-")
	if err != nil {
		return
	}
	_, werr := io.WriteString(tmp, strconv.Itoa(code)+"\n"+out)
	if cerr := tmp.Close(); werr != nil || cerr != nil || os.Rename(tmp.Name(), filepath.Join(dir, key)) != nil {
		os.Remove(tmp.Name())
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > cacheMaxAge {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
