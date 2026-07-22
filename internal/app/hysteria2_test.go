package app

import "testing"

func TestHysteria2VersionAtLeast(t *testing.T) {
	for _, test := range []struct {
		actual   string
		minimum  string
		supports bool
	}{
		{"1.13.14", "1.13.0", true},
		{"v1.14.0", "1.14.0", true},
		{"1.13.14", "1.14.0", false},
		{"1.14.0-alpha.1", "1.14.0", false},
		{"invalid", "1.14.0", false},
	} {
		if got := hysteria2VersionAtLeast(test.actual, test.minimum); got != test.supports {
			t.Fatalf("hysteria2VersionAtLeast(%q, %q) = %t, want %t", test.actual, test.minimum, got, test.supports)
		}
	}
}
