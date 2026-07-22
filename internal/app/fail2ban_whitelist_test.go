package app

import (
	"strings"
	"testing"
)

func TestFail2banWhitelistCanonicalization(t *testing.T) {
	entries, err := normalizedFail2banWhitelist([]string{"2001:0db8::1", "198.51.100.24", "198.51.100.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(entries, ",")
	for _, expected := range []string{"198.51.100.0/24", "198.51.100.24/32", "2001:db8::1/128"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing canonical entry %q in %q", expected, joined)
		}
	}
}

func TestFail2banWhitelistRejectsInvalidAndDuplicateEntries(t *testing.T) {
	for _, input := range [][]string{{"not-an-ip"}, {"198.51.100.1", "198.51.100.1/32"}} {
		if _, err := normalizedFail2banWhitelist(input); err == nil {
			t.Fatalf("invalid whitelist was accepted: %#v", input)
		}
	}
}

func TestFail2banJailRendersTrustedEntriesOnlyWhenConfigured(t *testing.T) {
	withoutWhitelist := renderFail2banSSHDJail(22, nil)
	if strings.Contains(withoutWhitelist, "ignoreip") {
		t.Fatalf("empty whitelist must preserve the existing jail rendering: %s", withoutWhitelist)
	}
	withWhitelist := renderFail2banSSHDJail(22, []string{"198.51.100.24/32"})
	if !strings.Contains(withWhitelist, "ignoreip = 127.0.0.1/8 ::1 198.51.100.24/32") {
		t.Fatalf("trusted entry was not rendered: %s", withWhitelist)
	}
}
