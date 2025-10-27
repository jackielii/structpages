package structpages

import (
	"reflect"
	"strings"
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

// Test type for methods
type testPage struct{}

func (t *testPage) String() string { return "test" }

// Test PageNode.String with components and props
func TestPageNode_String_componentsAndProps(t *testing.T) {
	// Get a real method for testing
	method, _ := reflect.TypeOf(&testPage{}).MethodByName("String")

	pn := &PageNode{
		Name:  "test",
		Title: "Test Page",
		Route: "/test",
		Value: reflect.ValueOf(&testPage{}),
		Components: map[string]reflect.Method{
			"Page": method,
		},
		Props: map[string]reflect.Method{
			"Props": method,
		},
	}

	str := pn.String()
	if str == "" {
		t.Error("Expected non-empty string")
	}

	// The string method has branches for empty/non-empty components
	// This tests the non-empty path
}

// Test Page node String method edge cases
func TestPageNode_String_edgeCases(t *testing.T) {
	// Test with zero Value (which we fixed to handle properly)
	pn := &PageNode{
		Name:        "test",
		Title:       "Test Page",
		Route:       "/test",
		Value:       reflect.Value{}, // Zero value
		Middlewares: nil,
		Components:  make(map[string]reflect.Method),
		Props:       make(map[string]reflect.Method),
	}

	str := pn.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}
	if strings.Contains(str, "is http.Handler: true") {
		t.Error("Should not show http.Handler for zero Value")
	}

	// Test with valid value
	type testPage2 struct{}
	pn.Value = reflect.ValueOf(&testPage2{})

	// Test with children
	child := &PageNode{
		Name:  "child",
		Title: "Child Page",
		Route: "/child",
		Value: reflect.ValueOf(&testPage2{}),
	}
	pn.Children = []*PageNode{child}

	str2 := pn.String()
	if !strings.Contains(str2, "child") {
		t.Error("Expected string to contain child information")
	}
}
