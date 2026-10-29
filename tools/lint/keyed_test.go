package lint

import "testing"

func TestRuntimeInToken(t *testing.T) {
	lit := func(s string) idPart { return idPart{kind: constPart, text: s} }
	val := idPart{kind: runtimePart}
	for _, tc := range []struct {
		name  string
		parts []idPart
		want  bool
	}{
		{"separator then value", []idPart{lit("-"), val}, true},
		{"role then value", []idPart{lit("-picker-"), val}, true},
		{"two literals then value", []idPart{lit("-a"), lit("b"), val}, true},
		{"underscore and non-ASCII", []idPart{lit("_é"), val}, true},
		{"value right after the target", []idPart{val}, false},
		{"space ends the token", []idPart{lit("-doc "), val}, false},
		{"attribute selector ends it", []idPart{lit("-doc[data-id=\""), val}, false},
		{"pseudo-class ends it", []idPart{lit(":has(> "), val}, false},
		{"class ends it", []idPart{lit("-doc."), val}, false},
		{"literal only", []idPart{lit("-document")}, false},
		{"escaped dot continues it", []idPart{lit(`-a\.b`), val}, true},
		{"hex escape and its space continue it", []idPart{lit(`-\31 `), val}, true},
		{"trailing backslash escapes the value", []idPart{lit(`-\`), val}, true},
		{"backslash-newline is no escape", []idPart{lit("-\\\n"), val}, false},
	} {
		if got := runtimeInToken(tc.parts); got != tc.want {
			t.Errorf("%s: runtimeInToken = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestEndsWithHash(t *testing.T) {
	for s, want := range map[string]bool{
		"#":          true,
		"closest #":  true,
		`\\#`:        true, // an escaped backslash, then "#"
		`\#`:         false,
		"#a":         false,
		"":           false,
		"[data-x=#]": false,
	} {
		if got := endsWithHash(s); got != want {
			t.Errorf("endsWithHash(%q) = %v, want %v", s, got, want)
		}
	}
}
