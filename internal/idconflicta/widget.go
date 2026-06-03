// Package idconflicta provides a Widget page type used to demonstrate
// cross-package ID collisions in tests.
package idconflicta

import (
	"context"
	"io"
)

type comp struct{ s string }

func (c comp) Render(_ context.Context, w io.Writer) error {
	_, err := io.WriteString(w, c.s)
	return err
}

// Widget is a page type. Note: another package defines a type with the
// same name "Widget" and the same method name "List".
type Widget struct{}

func (Widget) List() comp { return comp{s: "a.Widget.List"} }

// StatsWidget is a standalone function component. Another package
// defines a function with the same name.
func StatsWidget() comp { return comp{s: "a.StatsWidget"} }
