package app

import (
	"strings"
	"testing"
)

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

func TestHysteria2SalamanderUsageRequiresExplicitConfirmation(t *testing.T) {
	err := runHysteria2Salamander([]string{"enable"})
	if err == nil || !strings.Contains(err.Error(), "requires explicit --yes") {
		t.Fatalf("unexpected Salamander enable confirmation error: %v", err)
	}
	if err := runHysteria2Salamander([]string{"plan", "--yes"}); err == nil || !strings.Contains(err.Error(), "does not accept --yes") {
		t.Fatalf("unexpected Salamander plan confirmation error: %v", err)
	}
}
