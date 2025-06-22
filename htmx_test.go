package structpages

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_mixedCase(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		s    string
		want string
	}{
		{
			name: "Empty string",
			s:    "",
			want: "",
		},
		{
			name: "Single word",
			s:    "hello",
			want: "Hello",
		},
		{
			name: "Hyphenated words",
			s:    "hello-world",
			want: "HelloWorld",
		},
		{
			name: "Mixed case with hyphens",
			s:    "hello-World",
			want: "HelloWorld",
		},
		{
			name: "Multiple hyphenated words",
			s:    "hello-world-example",
			want: "HelloWorldExample",
		},
		{
			name: "No hyphens, just spaces",
			s:    "hello world",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mixedCase(tt.s)
			// Compare the result with expected value
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("mixedCase() mismatch (-got +want):\n%s", diff)
			}
		})
	}
}
