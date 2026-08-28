package main

import "testing"

// The parent project's lesson carried over: version rendering is a
// pure function with an unstamped fallback, tested as a truth table.
func TestFormat(t *testing.T) {
	cases := []struct {
		product, revision, timestamp string
		dirty                        bool
		want                         string
	}{
		{"0.1.0", "", "", false, "0.1.0"},
		{"0.1.0", "eef3f139d61bf4a8", "2026-08-28T19:00:00Z", false, "0.1.0 (eef3f13, 2026-08-28)"},
		{"0.1.0", "eef3f139d61bf4a8", "2026-08-28T19:00:00Z", true, "0.1.0 (eef3f13, dirty, 2026-08-28)"},
		{"0.1.0", "abc1234", "", true, "0.1.0 (abc1234, dirty)"},
	}
	for _, c := range cases {
		if got := format(c.product, c.revision, c.timestamp, c.dirty); got != c.want {
			t.Errorf("format(%q, %q, %q, %v) = %q, want %q",
				c.product, c.revision, c.timestamp, c.dirty, got, c.want)
		}
	}
}
