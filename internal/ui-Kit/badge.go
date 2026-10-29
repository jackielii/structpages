// Package uikit provides a standalone function component in a package
// whose import path element, "ui-Kit", kebab-cases with a doubled hyphen
// ("ui--kit"), to test that generated ids never contain the "--" that
// separates a keyed id's key.
package uikit

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
func Badge() comp { return comp{s: "uikit.Badge"} }
