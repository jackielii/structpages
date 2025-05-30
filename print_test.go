package srx

import (
	"testing"
)

type (
	TopPage struct {
		Level1 *Level1 `route:"level1" title:"Level 1"`
		Level2 *Level2 `route:"level2" title:"Level 2"`
	}
	Level1 struct{}
	Level2 struct{}
)

func (l *Level1) Page() templComponent    { return noopComponent{} }
func (l *Level1) Partial() templComponent { return noopComponent{} }
func (l *Level2) Page() templComponent    { return noopComponent{} }
func (l *Level2) Partial() templComponent { return noopComponent{} }

func TestPrint(t *testing.T) {
	NewStructPages().MountPages(&printRouter{}, "/", &TopPage{})
}
