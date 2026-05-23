package lint

import "testing"

func TestParseDirective(t *testing.T) {
	tests := []struct {
		in       string
		wantOK   bool
		wantCats map[string]bool
	}{
		{"//structpages:lint:ignore", true, map[string]bool{"": true}},
		{"// structpages:lint:ignore", true, map[string]bool{"": true}},
		{"//structpages:lint:ignore ref", true, map[string]bool{"ref": true}},
		{"//structpages:lint:ignore ref,params", true, map[string]bool{"ref": true, "params": true}},
		{"// structpages:lint:ignore  ref , params ", true, map[string]bool{"ref": true, "params": true}},
		{"/* structpages:lint:ignore ref */", true, map[string]bool{"ref": true}},
		{"// something else", false, nil},
		{"// structpages: not a directive", false, nil},
		{"plain text", false, nil},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := parseDirective(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if len(got) != len(tc.wantCats) {
				t.Fatalf("got %v, want %v", got, tc.wantCats)
			}
			for k := range tc.wantCats {
				if !got[k] {
					t.Fatalf("missing category %q in %v", k, got)
				}
			}
		})
	}
}

func TestPatternSegments(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"/", nil},
		{"/items", nil},
		{"/items/{id}", []string{"id"}},
		{"/items/{id}/sub/{kind}", []string{"id", "kind"}},
		{"/files/{path...}", []string{"path"}},
		{"/x/{$}", nil},
		{"/x/{a}/{$}", []string{"a"}},
		{"unterminated/{x", nil},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got := parsePatternSegments(tc.in)
			if !equalStrings(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
