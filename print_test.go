package srx

import (
	"testing"

	"github.com/a-h/templ"
)

type (
	TopPage struct {
		Level1 *Level1 `route:"level1" title:"Level 1"`
		Level2 *Level2 `route:"level2" title:"Level 2"`
	}
	Level1 struct{}
	Level2 struct{}
)

func (l *Level1) Page() templ.Component    { return templ.NopComponent }
func (l *Level1) Partial() templ.Component { return templ.NopComponent }
func (l *Level2) Page() templ.Component    { return templ.NopComponent }
func (l *Level2) Partial() templ.Component { return templ.NopComponent }

func TestPrint(t *testing.T) {
	NewStructPages().MountPages(&printeRouter{}, "/", &TopPage{})
}
