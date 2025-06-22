package structpages

import (
	"testing"
)

func Test_walk(t *testing.T) {
	testNode := &PageNode{
		Name: "Top",
		Children: []*PageNode{
			{
				Name: "Child1",
				Children: []*PageNode{
					{Name: "GrandChild1"},
					{Name: "GrandChild2"},
				},
			},
			{
				Name: "Child2",
				Children: []*PageNode{
					{Name: "GrandChild3"},
					{Name: "GrandChild4"},
				},
			},
		},
	}
	expected := []string{"Top", "Child1", "GrandChild1", "GrandChild2", "Child2", "GrandChild3", "GrandChild4"}
	t.Run("walk test", func(t *testing.T) {
		items := make([]string, 0)
		walk(testNode, func(p *PageNode) bool {
			items = append(items, p.Name)
			return true
		})
		if len(items) != 7 {
			t.Errorf("Expected 7 items, got %d", len(items))
		}
		for i, name := range expected {
			if items[i] != name {
				t.Errorf("Expected item %d to be %s, got %s", i, name, items[i])
			}
		}
	})
	t.Run("walk iter all", func(t *testing.T) {
		items := make([]string, 0)
		for n := range testNode.All() {
			items = append(items, n.Name)
		}
		for i, name := range expected {
			if items[i] != name {
				t.Errorf("Expected item %d to be %s, got %s", i, name, items[i])
			}
		}
	})
	t.Run("walk with break", func(t *testing.T) {
		items := make([]string, 0)
		i := 0
		for n := range testNode.All() {
			if i == 3 {
				break
			}
			items = append(items, n.Name)
			i++
		}
		if len(items) != 3 {
			t.Errorf("Expected 3 items, got %d", len(items))
		}
		for i, name := range expected[:3] {
			if items[i] != name {
				t.Errorf("Expected item %d to be %s, got %s", i, name, items[i])
			}
		}
	})
}
