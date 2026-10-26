// Package fnfmt provides a standalone function component in a package
// whose import path contains "-fm", the suffix the Go compiler gives bound
// method values, to test that its function is not taken for one.
package fnfmt

import (
	"context"
	"io"
)

type comp struct{ s string }

func (c comp) Render(_ context.Context, w io.Writer) error {
	_, err := io.WriteString(w, c.s)
	return err
}

// Badge is a standalone function component.
func Badge() comp { return comp{s: "fnfmt.Badge"} }
