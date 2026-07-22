package app

import (
	"strings"
	"testing"

	"vpskit.local/vpskit/internal/model"
)

func TestNormalizeUserRulesCanonicalizesAndSorts(t *testing.T) {
	rules, err := normalizeUserRules([]model.UserRule{
		{Source: "custom", Type: "IP-CIDR", Value: "10.0.0.42/8", Policy: "reject"},
		{Source: "whitelist", Type: "domain", Value: "Captcha.Example.COM.", Policy: "direct"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Fatalf("unexpected rule count: %#v", rules)
	}
	if rules[0].Source != model.UserRuleSourceCustom || rules[0].Value != "10.0.0.0/8" || rules[0].Policy != model.UserRulePolicyReject {
		t.Fatalf("custom CIDR was not canonicalized: %#v", rules[0])
	}
	if rules[1].Source != model.UserRuleSourceWhitelist || rules[1].Value != "captcha.example.com" || rules[1].Policy != model.UserRulePolicyDirect {
		t.Fatalf("whitelist domain was not canonicalized: %#v", rules[1])
	}
}

func TestNormalizeUserRulesRejectsUnsafeOrConflictingEntries(t *testing.T) {
	for _, input := range [][]model.UserRule{
		{{Source: "whitelist", Type: "domain-suffix", Value: "example.com", Policy: "direct"}},
		{{Source: "custom", Type: "domain", Value: "*.example.com", Policy: "proxy"}},
		{{Source: "custom", Type: "domain", Value: "example.com", Policy: "proxy"}, {Source: "custom", Type: "domain", Value: "example.com", Policy: "reject"}},
	} {
		if _, err := normalizeUserRules(input); err == nil {
			t.Fatalf("expected invalid user rules to fail: %#v", input)
		}
	}
}

func TestUserRuleOverlapWarningsReportDomainSuffixOverlap(t *testing.T) {
	warnings := userRuleOverlapWarnings([]model.UserRule{
		{Source: model.UserRuleSourceWhitelist, Type: model.UserRuleTypeDomain, Value: "login.example.com", Policy: model.UserRulePolicyDirect},
		{Source: model.UserRuleSourceCustom, Type: model.UserRuleTypeDomainSuffix, Value: "example.com", Policy: model.UserRulePolicyReject},
	})
	if len(warnings) != 1 || !strings.Contains(warnings[0]["reason"], "重叠") {
		t.Fatalf("expected one overlap warning, got %#v", warnings)
	}
}
