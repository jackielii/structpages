// Package idconflictb provides a Widget page type used to demonstrate
// cross-package ID collisions in tests.
package idconflictb

import (
	"context"
	"io"
)

type comp struct{ s string }

func (c comp) Render(_ context.Context, w io.Writer) error {
	_, err := io.WriteString(w, c.s)
	return err
}

// Widget is a page type with the same name as idconflicta.Widget and the
// same method name "List".
type Widget struct{}

func (Widget) List() comp { return comp{s: "b.Widget.List"} }

// StatsWidget is a standalone function component with the same name as
// idconflicta.StatsWidget.
func StatsWidget() comp { return comp{s: "b.StatsWidget"} }
