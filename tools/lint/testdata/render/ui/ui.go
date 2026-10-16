// Package ui holds components that aren't pages: a standalone function
// component and a type no tree mounts.
package ui

import (
	"context"
	"io"
)

// Comp is a renderable stand-in.
type Comp struct{}

// Render implements the structpages component contract.
func (Comp) Render(context.Context, io.Writer) error { return nil }

// Toast is a standalone function component.
func Toast() Comp { return Comp{} }

// Widget has component methods but is mounted nowhere.
type Widget struct{}

// Body is a component method of an unmounted type.
func (Widget) Body() Comp { return Comp{} }
