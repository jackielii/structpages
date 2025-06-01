package structpages

import (
	"errors"
	"testing"
)

func Test_formatPathSegments(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		path string
		args []any
		want string
		err  error
	}{
		{
			name: "Empty path",
			path: "",
			args: []any{},
			want: "",
		},
		{
			name: "static path",
			path: "/static/path",
			args: []any{},
			want: "/static/path",
		},
		{
			name: "path with args",
			path: "/path/{arg1}/{arg2}",
			args: []any{"value1", "value2"},
			want: "/path/value1/value2",
		},
		{
			name: "path with fewer args",
			path: "/path/{arg1}/{arg2}",
			args: []any{"value1"},
			want: "/path/{arg1}/{arg2}",
			err:  errors.New("pattern /path/{arg1}/{arg2}: use map[string]any for single arg or provide the full args"),
		},
		// {
		// 	name: "path with more args",
		// 	path: "/path/{arg1}/{arg2}",
		// 	args: []any{"value1", "value2", "extra"},
		// 	want: "/path/{arg1}/{arg2}",
		// 	err:  errors.New("pattern /path/{arg1}/{arg2}: too many arguments provided for segment: {arg2}"),
		// },
		{
			name: "path with no args",
			path: "/path/{arg1}/{arg2}",
			args: []any{},
			want: "/path/{arg1}/{arg2}",
			err:  errors.New("pattern /path/{arg1}/{arg2}: not enough arguments provided for segment: [arg1 arg2]"),
		},
		{
			name: "path with map args",
			path: "/path/{arg1}/{arg2}",
			args: []any{map[string]any{"arg1": "value1", "arg2": "value2"}},
			want: "/path/value1/value2",
			err:  nil,
		},
		{
			name: "path with map args missing key",
			path: "/path/{arg1}/{arg2}",
			args: []any{map[string]any{"arg1": "value1"}},
			want: "/path/{arg1}/{arg2}",
			err:  errors.New("pattern /path/{arg1}/{arg2}: argument arg2 not found in provided args"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatPathSegments(tt.path, tt.args...)
			if tt.err != nil {
				if err == nil || err.Error() != tt.err.Error() {
					t.Errorf("formatPathSegments() error = %v, want %v", err, tt.err)
				}
			}
			if got != tt.want {
				t.Errorf("formatPathSegments() = %v, want %v", got, tt.want)
			}
		})
	}
}
