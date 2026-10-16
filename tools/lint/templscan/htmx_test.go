package templscan

import (
	"fmt"
	"testing"
)

func TestHTMX(t *testing.T) {
	tests := []struct {
		file string
		want string // "line:col name", or "" for no htmx
	}{
		{"testdata/htmx.templ", "6:11 data-hx-get"},
		{"testdata/htmx_partial.templ", "4:3 hx-partial"},
		{"testdata/htmx_bool.templ", "4:8 hx-boost"},
		{"testdata/constant_attr.templ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			use, ok, err := HTMX(tt.file)
			if err != nil {
				t.Fatalf("HTMX: %v", err)
			}
			got := ""
			if ok {
				got = fmt.Sprintf("%d:%d %s", use.Line, use.Col, use.Name)
			}
			if got != tt.want {
				t.Errorf("HTMX = %q, want %q", got, tt.want)
			}
		})
	}
}
