package stringsutil

import "testing"

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{name: "no values", values: nil, want: ""},
		{name: "all empty", values: []string{"", "   ", "\t"}, want: ""},
		{name: "first wins after trim", values: []string{"", "  a  ", "b"}, want: "a"},
		{name: "later value", values: []string{"", "", " c "}, want: "c"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := FirstNonEmpty(tc.values...); got != tc.want {
				t.Fatalf("FirstNonEmpty(%v)=%q, want %q", tc.values, got, tc.want)
			}
		})
	}
}
