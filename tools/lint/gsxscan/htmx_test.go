package gsxscan

import (
	"fmt"
	"testing"
)

func TestHTMX(t *testing.T) {
	tests := []struct {
		file string
		want string // "line:col name", or "" for no htmx
	}{
		{"testdata/attrs.gsx", "9:5 hx-get"},
		{"testdata/htmx_partial.gsx", "5:3 hx-partial"},
		{"testdata/htmx_data.gsx", "6:11 DATA-HX-POST"},
		{"testdata/htmx_bag.gsx", "4:46 hx-delete"}, // a bag pair is positioned at its value
		{"testdata/htmx_bool.gsx", "4:8 hx-boost:inherited"},
		{"testdata/suppression.gsx", ""},
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

func TestHTMX_ParseError(t *testing.T) {
	if _, _, err := HTMX("testdata/broken.gsx"); err == nil {
		t.Error("HTMX on a broken file: want a parse error")
	}
}
