package templscan

import (
	"errors"
	"path/filepath"
	"testing"
)

// The linter's templ fixtures are joined to their .templ only when the
// _templ.go is exactly what templ generates, so a formatter run over one
// (goimports regroups its imports) silently drops its findings.
// stale.templ's _templ.go differs on purpose.
func TestExprAttrs_fixturesAreTemplOutput(t *testing.T) {
	files, err := filepath.Glob("../testdata/*/tapp/*.templ")
	if err != nil || len(files) == 0 {
		t.Fatalf("no .templ fixtures: %v", err)
	}
	for _, f := range files {
		_, err := ExprAttrs(f)
		switch {
		case filepath.Base(f) == "stale.templ":
			if !errors.Is(err, ErrStale) {
				t.Errorf("%s: err = %v, want ErrStale", f, err)
			}
		case err != nil:
			t.Errorf("%s: %v (commit a _templ.go exactly as `templ generate` writes it: "+
				"no goimports, no gofmt -s)", f, err)
		}
	}
}
