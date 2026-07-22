package app

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"sort"
	"strings"

	"vpskit.local/vpskit/internal/model"
)

const maxUserRules = 128

func normalizedUserRule(rule model.UserRule) (model.UserRule, error) {
	rule.Source = strings.ToLower(strings.TrimSpace(rule.Source))
	if rule.Source != model.UserRuleSourceWhitelist && rule.Source != model.UserRuleSourceCustom {
		return model.UserRule{}, fmt.Errorf("unsupported source %q", rule.Source)
	}
	rule.Type = strings.ToLower(strings.TrimSpace(rule.Type))
	switch rule.Type {
	case model.UserRuleTypeDomain, model.UserRuleTypeDomainSuffix, model.UserRuleTypeIPCIDR:
	default:
		return model.UserRule{}, fmt.Errorf("unsupported type %q; use domain, domain-suffix, or ip-cidr", rule.Type)
	}
	rule.Policy = normalizedUserRulePolicy(rule.Policy)
	if rule.Policy == "" {
		return model.UserRule{}, errors.New("unsupported policy; use direct, proxy, or reject")
	}
	if rule.Source == model.UserRuleSourceWhitelist && (rule.Type != model.UserRuleTypeDomain || rule.Policy != model.UserRulePolicyDirect) {
		return model.UserRule{}, errors.New("whitelist entries must be exact domains with DIRECT policy")
	}
	value := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(rule.Value), "."))
	switch rule.Type {
	case model.UserRuleTypeDomain, model.UserRuleTypeDomainSuffix:
		if !validRuleDomain(value) {
			return model.UserRule{}, fmt.Errorf("invalid domain %q", rule.Value)
		}
	case model.UserRuleTypeIPCIDR:
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return model.UserRule{}, fmt.Errorf("invalid IP CIDR %q", rule.Value)
		}
		value = network.String()
	}
	rule.Value = value
	return rule, nil
}

func normalizedUserRulePolicy(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "direct":
		return model.UserRulePolicyDirect
	case "proxy":
		return model.UserRulePolicyProxy
	case "reject":
		return model.UserRulePolicyReject
	default:
		return ""
	}
}

func validRuleDomain(value string) bool {
	if len(value) < 1 || len(value) > 253 || strings.Contains(value, "..") {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-' {
				continue
			}
			return false
		}
	}
	return true
}

func normalizeUserRules(input []model.UserRule) ([]model.UserRule, error) {
	if len(input) > maxUserRules {
		return nil, fmt.Errorf("at most %d user rules are supported", maxUserRules)
	}
	rules := make([]model.UserRule, 0, len(input))
	seen := map[string]model.UserRule{}
	for _, raw := range input {
		rule, err := normalizedUserRule(raw)
		if err != nil {
			return nil, err
		}
		key := rule.Type + "\x00" + rule.Value
		if previous, exists := seen[key]; exists {
			if previous == rule {
				return nil, fmt.Errorf("duplicate user rule %s %s", rule.Type, rule.Value)
			}
			return nil, fmt.Errorf("conflicting user rules for %s %s", rule.Type, rule.Value)
		}
		seen[key] = rule
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(left, right int) bool {
		if rules[left].Source != rules[right].Source {
			return rules[left].Source < rules[right].Source
		}
		if rules[left].Type != rules[right].Type {
			return rules[left].Type < rules[right].Type
		}
		return rules[left].Value < rules[right].Value
	})
	return rules, nil
}

func userRuleOverlapWarnings(rules []model.UserRule) []map[string]string {
	warnings := make([]map[string]string, 0)
	for left := range rules {
		for right := left + 1; right < len(rules); right++ {
			if !userRulesOverlap(rules[left], rules[right]) {
				continue
			}
			warnings = append(warnings, map[string]string{
				"left":   rules[left].Type + ":" + rules[left].Value + "=" + rules[left].Policy,
				"right":  rules[right].Type + ":" + rules[right].Value + "=" + rules[right].Policy,
				"reason": "规则范围重叠；Mihomo 会优先匹配前面的精确域名规则",
			})
		}
	}
	return warnings
}

func userRulesOverlap(left, right model.UserRule) bool {
	if left.Type == model.UserRuleTypeIPCIDR || right.Type == model.UserRuleTypeIPCIDR {
		return left.Type == right.Type && left.Value == right.Value
	}
	if left.Type == model.UserRuleTypeDomain && right.Type == model.UserRuleTypeDomain {
		return left.Value == right.Value
	}
	if left.Type == model.UserRuleTypeDomainSuffix && right.Type == model.UserRuleTypeDomainSuffix {
		return domainMatchesSuffix(left.Value, right.Value) || domainMatchesSuffix(right.Value, left.Value)
	}
	if left.Type == model.UserRuleTypeDomain {
		return domainMatchesSuffix(left.Value, right.Value)
	}
	return domainMatchesSuffix(right.Value, left.Value)
}

func domainMatchesSuffix(domain, suffix string) bool {
	return domain == suffix || strings.HasSuffix(domain, "."+suffix)
}

func runRuleWhitelist(state model.State, arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: vpskit rules whitelist <list|add|remove> [--domain <domain>] [--yes]")
	}
	operation := strings.ToLower(strings.TrimSpace(arguments[0]))
	if operation == "list" {
		if len(arguments) != 1 {
			return errors.New("usage: vpskit rules whitelist list")
		}
		return printJSON(commandResult{Command: "rules whitelist list", Status: "PASS", Detail: filterUserRules(state.Rules.UserRules, model.UserRuleSourceWhitelist)})
	}
	if operation != "add" && operation != "remove" {
		return fmt.Errorf("unsupported whitelist operation %q", operation)
	}
	flags := flag.NewFlagSet("rules whitelist "+operation, flag.ContinueOnError)
	domain := flags.String("domain", "", "exact domain to allow directly")
	yes := flags.Bool("yes", false, "confirm client configuration update")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*domain) == "" || !*yes {
		return fmt.Errorf("rules whitelist %s requires --domain <domain> --yes", operation)
	}
	rule, err := normalizedUserRule(model.UserRule{Source: model.UserRuleSourceWhitelist, Type: model.UserRuleTypeDomain, Value: *domain, Policy: model.UserRulePolicyDirect})
	if err != nil {
		return err
	}
	return mutateUserRules(state, operation, rule, "rules whitelist "+operation)
}

func runRuleCustom(state model.State, arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: vpskit rules custom <list|check|add|remove> [--type domain|domain-suffix|ip-cidr --value <value> --policy direct|proxy|reject --yes]")
	}
	operation := strings.ToLower(strings.TrimSpace(arguments[0]))
	switch operation {
	case "list":
		if len(arguments) != 1 {
			return errors.New("usage: vpskit rules custom list")
		}
		return printJSON(commandResult{Command: "rules custom list", Status: "PASS", Detail: filterUserRules(state.Rules.UserRules, model.UserRuleSourceCustom)})
	case "check":
		if len(arguments) != 1 {
			return errors.New("usage: vpskit rules custom check")
		}
		return printJSON(commandResult{Command: "rules custom check", Status: "PASS", Detail: map[string]any{"user_rules": len(state.Rules.UserRules), "overlap_warnings": userRuleOverlapWarnings(state.Rules.UserRules)}})
	case "add", "remove":
		flags := flag.NewFlagSet("rules custom "+operation, flag.ContinueOnError)
		ruleType := flags.String("type", "", "domain, domain-suffix, or ip-cidr")
		value := flags.String("value", "", "domain or CIDR value")
		policy := flags.String("policy", "", "direct, proxy, or reject")
		yes := flags.Bool("yes", false, "confirm client configuration update")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 || strings.TrimSpace(*ruleType) == "" || strings.TrimSpace(*value) == "" || strings.TrimSpace(*policy) == "" || !*yes {
			return fmt.Errorf("rules custom %s requires --type --value --policy --yes", operation)
		}
		rule, err := normalizedUserRule(model.UserRule{Source: model.UserRuleSourceCustom, Type: *ruleType, Value: *value, Policy: *policy})
		if err != nil {
			return err
		}
		return mutateUserRules(state, operation, rule, "rules custom "+operation)
	default:
		return fmt.Errorf("unsupported custom rule operation %q", operation)
	}
}

func filterUserRules(rules []model.UserRule, source string) []model.UserRule {
	filtered := make([]model.UserRule, 0)
	for _, rule := range rules {
		if rule.Source == source {
			filtered = append(filtered, rule)
		}
	}
	return filtered
}

func mutateUserRules(state model.State, operation string, rule model.UserRule, command string) error {
	updated := state
	if operation == "add" {
		updated.Rules.UserRules = append(updated.Rules.UserRules, rule)
		normalized, err := normalizeUserRules(updated.Rules.UserRules)
		if err != nil {
			return err
		}
		updated.Rules.UserRules = normalized
	} else {
		index := -1
		for current, candidate := range updated.Rules.UserRules {
			if candidate == rule {
				index = current
				break
			}
		}
		if index < 0 {
			return errors.New("specified user rule does not exist")
		}
		updated.Rules.UserRules = append(updated.Rules.UserRules[:index], updated.Rules.UserRules[index+1:]...)
	}
	updated.ConfigRevision++
	secrets, err := readInstalledSecrets()
	if err != nil {
		return err
	}
	commit, err := commitManagedStateChange(state, updated, secrets, command, "rules", []string{"rules.user_rules"})
	if err != nil {
		return err
	}
	return printJSON(commandResult{Command: command, Status: "PASS", Detail: map[string]any{
		"rule":                   rule,
		"user_rule_count":        len(commit.State.Rules.UserRules),
		"whitelist_count":        len(filterUserRules(commit.State.Rules.UserRules, model.UserRuleSourceWhitelist)),
		"config_revision":        commit.State.ConfigRevision,
		"client_update_required": true,
		"transaction_id":         commit.TransactionID,
		"previous_backup_id":     commit.BackupID,
		"subscription_publish":   commit.SubscriptionPublish,
	}})
}
