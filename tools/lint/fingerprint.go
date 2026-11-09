package lint

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

// Fingerprint writes to w a description of every input a run over g
// reads, so that two runs whose fingerprints match produce the same
// findings:
//
//   - the package graph itself: every package variant and its imports;
//   - for packages in a module on disk (the main module, or one
//     replaced by a local directory): the path and content hash of each
//     Go file, of each .gsx and .templ file beside them, and of the
//     module's go.mod;
//   - for packages in a versioned module: the module path and version,
//     whose content the module cache keeps immutable;
//   - for standard-library packages: their directory, which names the
//     toolchain.
//
// Files are identified by content, not modification time, so a checkout
// or copy that preserves timestamps can't reuse a stale result; hashing
// a large module's sources (thousands of files) takes a fraction of a
// second, in parallel. Paths are absolute, so two checkouts of a module
// never share a fingerprint.
func (g *Graph) Fingerprint(w io.Writer) error {
	var lines []string
	add := func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) }
	type hashed struct{ kind, name string }
	var files []hashed
	seen := map[string]bool{} // a package's variants share its files
	hashFile := func(kind, name string) {
		if !seen[name] {
			seen[name] = true
			files = append(files, hashed{kind, name})
		}
	}
	modules := map[string]bool{}
	dirs := map[string]bool{}
	packages.Visit(g.Roots, nil, func(p *packages.Package) {
		add("pkg %s %s", p.ID, p.Name)
		for _, imp := range p.Imports {
			add("imp %s %s", p.ID, imp.ID)
		}
		m := p.Module
		if m != nil && m.Replace != nil {
			m = m.Replace
		}
		switch {
		case m == nil:
			// The standard library (or a package outside any module):
			// its directory names the toolchain that compiles it.
			if len(p.GoFiles) > 0 {
				add("std %s %s", p.PkgPath, filepath.Dir(p.GoFiles[0]))
			}
			return
		case m.Version != "":
			if !modules[m.Path] {
				modules[m.Path] = true
				add("mod %s %s", m.Path, m.Version)
			}
			return
		}
		if !modules[m.Path] {
			modules[m.Path] = true
			if m.GoMod != "" {
				hashFile("gomod", m.GoMod)
			}
		}
		for _, files := range [][]string{p.GoFiles, p.OtherFiles, p.EmbedFiles} {
			for _, f := range files {
				hashFile("file", f)
			}
		}
		dir := p.Dir
		if dir == "" && len(p.GoFiles) > 0 {
			dir = filepath.Dir(p.GoFiles[0])
		}
		if dir == "" || dirs[dir] {
			return
		}
		dirs[dir] = true
		entries, err := os.ReadDir(dir)
		if err != nil {
			add("dir %s unreadable", dir)
			return
		}
		for _, e := range entries {
			if name := e.Name(); !e.IsDir() && (strings.HasSuffix(name, ".gsx") || strings.HasSuffix(name, ".templ")) {
				hashFile("template", filepath.Join(dir, name))
			}
		}
	})
	sums := make([]string, len(files))
	parallelDo(len(files), func(i int) {
		sums[i] = "missing"
		if f, err := os.Open(files[i].name); err == nil {
			h := sha256.New()
			if _, err := io.Copy(h, f); err == nil {
				sums[i] = hex.EncodeToString(h.Sum(nil))
			}
			f.Close()
		}
	})
	for i, f := range files {
		add("%s %s %s", f.kind, f.name, sums[i])
	}
	slices.Sort(lines)
	lines = slices.Compact(lines)
	for _, l := range lines {
		if _, err := io.WriteString(w, l+"\n"); err != nil {
			return err
		}
	}
	return nil
}

// parallelDo calls f(0) … f(n-1) on GOMAXPROCS goroutines and waits.
func parallelDo(n int, f func(int)) {
	next := make(chan int)
	var wg sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				f(i)
			}
		}()
	}
	for i := range n {
		next <- i
	}
	close(next)
	wg.Wait()
}
