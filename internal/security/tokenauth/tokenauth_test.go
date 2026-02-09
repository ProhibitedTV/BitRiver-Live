package tokenauth

import (
	"net/http/httptest"
	"testing"
)

func TestConstantTimeEqual(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		provided string
		want     bool
	}{
		{name: "empty expected", expected: "", provided: "abc", want: false},
		{name: "empty provided", expected: "abc", provided: "", want: false},
		{name: "length mismatch", expected: "abcd", provided: "abc", want: false},
		{name: "equal", expected: "abc", provided: "abc", want: true},
		{name: "different", expected: "abc", provided: "abd", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ConstantTimeEqual(tc.expected, tc.provided); got != tc.want {
				t.Fatalf("ConstantTimeEqual(%q, %q)=%v, want %v", tc.expected, tc.provided, got, tc.want)
			}
		})
	}
}

func TestParseBearerHeader(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
		status BearerHeaderStatus
	}{
		{name: "missing", header: "", status: BearerHeaderMissing},
		{name: "wrong prefix", header: "Token abc", status: BearerHeaderInvalid},
		{name: "empty token", header: "Bearer    ", status: BearerHeaderTokenMissing},
		{name: "case normalized", header: "bEaReR abc", want: "abc", status: BearerHeaderValid},
		{name: "trimmed", header: "  Bearer  abc  ", want: "abc", status: BearerHeaderValid},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, status := ParseBearerHeader(tc.header)
			if got != tc.want || status != tc.status {
				t.Fatalf("ParseBearerHeader(%q)=(%q,%v), want (%q,%v)", tc.header, got, status, tc.want, tc.status)
			}
		})
	}
}

func TestQueryToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/x?token=%20%20abc%20%20", nil)
	got, ok := QueryToken(req, "token")
	if !ok || got != "abc" {
		t.Fatalf("QueryToken()=(%q,%v), want (abc,true)", got, ok)
	}
}
