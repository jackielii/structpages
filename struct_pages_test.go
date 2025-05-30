package srx

import (
	"context"
	"io"
	"reflect"
	"testing"
)

//	func Test_parsePageItem(t *testing.T) {
//		pc := parsePageTree("/", &TopPages{})
//		printPageItem(t, "", pc.rootNode)
//	}
type noopComponent struct{}

func (noopComponent) Render(ctx context.Context, w io.Writer) error {
	return nil
}

func printPageItem(t *testing.T, indent string, item *PageNode) {
	t.Helper()
	t.Logf("%sName: %s", indent, item.Name)
	t.Logf("%sRoute: %s", indent, item.Route)
	t.Logf("%sPage: %v", indent, formatMethod(item.Page))
	t.Logf("%sPartial: %v", indent, formatMethod(item.Partial))
	t.Logf("%sArgs: %v", indent, formatMethod(item.Args))
	for _, child := range item.Children {
		printPageItem(t, indent+"  ", child)
	}
}

type TestItem1 struct{}

func (i *TestItem1) Page() templComponent {
	return noopComponent{}
}

type TestTop1 struct {
	FirstField  *TestItem1 `route:"first" title:"First Item"`
	SecondField *TestItem1 `route:"second" title:"Second Item"`
}

func TestParseWithFieldName(t *testing.T) {
	pc := parsePageTree("/", &TestTop1{})
	printPageItem(t, "", pc.rootNode)
}

func tt() (context.Context, io.Reader, error) {
	return nil, nil, nil
}

func TestAssignableToError(_ *testing.T) {
	t := reflect.ValueOf(tt)
	numOut := t.Type().NumOut()
	last := t.Type().Out(numOut - 1)
	println("assignable to error:", last.AssignableTo(reflect.TypeOf((*error)(nil)).Elem()))
	println("equal to error:", last == reflect.TypeOf((*error)(nil)).Elem())
}

type TopPages2 struct {
	Level1 struct {
		Level2 struct {
			Level3 struct {
				Level4 struct {
					Level5 struct{}
				} `route:"level5" title:"Level 5"`
			} `route:"level4" title:"Level 4"`
		} `route:"level3" title:"Level 3"`
	} `route:"level1" title:"Level 1"`
}

func TestParseNestedPages(t *testing.T) {
	pc := parsePageTree("/", &TopPages2{})
	printPageItem(t, "", pc.rootNode)
	// println("level5 full route:", pc.rootNode.Children[0].Children[0].Children[0].Children[0].FullRoute())
	// assert.Equal(t, "/level1/level3/level4/level5", pc.rootNode.Children[0].Children[0].Children[0].Children[0].FullRoute())
	// if len(pc.rootNode.Children) != 1 {
	// 	t.Errorf("Expected 1 child, got %d", len(pc.rootNode.Children))
	// }
	// if len(pc.rootNode.Children[0].Children) != 1 {
	// 	t.Errorf("Expected 1 child, got %d", len(pc.rootNode.Children[0].Children))
	// }
	// if len(pc.rootNode.Children[0].Children[0].Children) != 1 {
	// 	t.Errorf("Expected 1 child, got %d", len(pc.rootNode.Children[0].Children[0].Children))
	// }
	// if len(pc.rootNode.Children[0].Children[0].Children[0].Children) != 1 {
	// 	t.Errorf("Expected 1 child, got %d", len(pc.rootNode.Children[0].Children[0].Children[0].Children))
	// }
}
