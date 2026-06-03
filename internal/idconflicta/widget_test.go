package idconflicta

import (
	"context"
	"strings"
	"testing"
)

func TestComponentsRender(t *testing.T) {
	cases := map[string]comp{
		"Widget.List": Widget{}.List(),
		"StatsWidget": StatsWidget(),
	}
	want := map[string]string{
		"Widget.List": "a.Widget.List",
		"StatsWidget": "a.StatsWidget",
	}
	for name, c := range cases {
		var b strings.Builder
		if err := c.Render(context.Background(), &b); err != nil {
			t.Fatalf("%s: Render: %v", name, err)
		}
		if got := b.String(); got != want[name] {
			t.Errorf("%s: rendered %q, want %q", name, got, want[name])
		}
	}
}
