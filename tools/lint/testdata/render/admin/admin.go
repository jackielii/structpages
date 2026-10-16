// Package admin declares a page mounted in pages.Root's tree.
package admin

import "ex/ui"

// Panel is mounted under /admin.
type Panel struct{}

// Page renders the panel.
func (Panel) Page() ui.Comp { return ui.Comp{} }

// Stats is a partial of the panel.
func (Panel) Stats() ui.Comp { return ui.Comp{} }
