package app

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/platform"
)

func normalizedRulesProfile(value string) (string, error) {
	profile := strings.ToLower(strings.TrimSpace(value))
	if profile == "" {
		profile = model.RulesProfileMinimal
	}
	switch profile {
	case model.RulesProfileMinimal, model.RulesProfileACL4SSR, model.RulesProfileACL4SSRAntiAD:
		return profile, nil
	default:
		return "", fmt.Errorf("unsupported rules profile %q; use minimal, acl4ssr, or acl4ssr-antiad", value)
	}
}

func runRules(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: vpskit rules <show|plan|apply|refresh> [--profile minimal|acl4ssr|acl4ssr-antiad] [--yes]")
	}
	if !platform.IsRoot() {
		return errors.New("rules inspection and mutation require root privileges")
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	switch arguments[0] {
	case "show":
		if len(arguments) != 1 {
			return errors.New("usage: vpskit rules show")
		}
		return printJSON(commandResult{Command: "rules show", Status: "PASS", Detail: state.Rules})
	case "plan", "apply":
		flags := flag.NewFlagSet("rules "+arguments[0], flag.ContinueOnError)
		profile := flags.String("profile", "", "minimal, acl4ssr, or acl4ssr-antiad")
		yes := flags.Bool("yes", false, "confirm client configuration update")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 || strings.TrimSpace(*profile) == "" {
			return errors.New("rules plan/apply requires --profile minimal|acl4ssr|acl4ssr-antiad")
		}
		normalized, err := normalizedRulesProfile(*profile)
		if err != nil {
			return err
		}
		if arguments[0] == "plan" {
			if *yes {
				return errors.New("rules plan does not accept --yes")
			}
			return printJSON(commandResult{Command: "rules plan", Status: "PASS", Detail: map[string]any{"mode": "READ_ONLY", "current_profile": state.Rules.Profile, "next_profile": normalized, "next_ruleset_revision": state.Rules.Revision + 1, "client_update_required": state.Rules.Profile != normalized}})
		}
		if !*yes {
			return errors.New("rules apply requires explicit --yes confirmation")
		}
		if state.Rules.Profile == normalized {
			return errors.New("rules profile is unchanged")
		}
		updated := state
		updated.Rules.Profile = normalized
		updated.Rules.SourceMode = model.RulesSourceDirect
		updated.Rules.Revision++
		updated.ConfigRevision++
		secrets, err := readInstalledSecrets()
		if err != nil {
			return err
		}
		commit, err := commitManagedStateChange(state, updated, secrets, "rules apply", "rules", []string{"rules.profile"})
		if err != nil {
			return err
		}
		return printJSON(commandResult{Command: "rules apply", Status: "PASS", Detail: map[string]any{"profile": commit.State.Rules.Profile, "ruleset_revision": commit.State.Rules.Revision, "config_revision": commit.State.ConfigRevision, "client_update_required": true, "transaction_id": commit.TransactionID, "previous_backup_id": commit.BackupID, "subscription_publish": commit.SubscriptionPublish}})
	case "refresh":
		flags := flag.NewFlagSet("rules refresh", flag.ContinueOnError)
		yes := flags.Bool("yes", false, "confirm downloading, hashing, and publishing the active rule sources")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 || !*yes {
			return errors.New("rules refresh requires explicit --yes confirmation")
		}
		if state.Rules.Profile == model.RulesProfileMinimal {
			return errors.New("rules refresh requires acl4ssr or acl4ssr-antiad")
		}
		if _, err := managedRuleProviderBaseURL(); err != nil {
			return err
		}
		nextRulesetRevision := state.Rules.Revision + 1
		manifest, err := refreshManagedRuleCache(state.Rules.Profile, nextRulesetRevision)
		if err != nil {
			return err
		}
		updated := state
		updated.Rules.SourceMode = model.RulesSourceManaged
		updated.Rules.Revision = nextRulesetRevision
		updated.ConfigRevision++
		secrets, err := readInstalledSecrets()
		if err != nil {
			return err
		}
		commit, err := commitManagedStateChange(state, updated, secrets, "rules refresh", "rules", []string{"rules.source_mode", "rules.revision", "rules.cache"})
		if err != nil {
			return err
		}
		return printJSON(commandResult{Command: "rules refresh", Status: "PASS", Detail: map[string]any{"profile": commit.State.Rules.Profile, "source_mode": commit.State.Rules.SourceMode, "ruleset_revision": commit.State.Rules.Revision, "config_revision": commit.State.ConfigRevision, "sources": len(manifest.Sources), "client_update_required": true, "transaction_id": commit.TransactionID, "previous_backup_id": commit.BackupID, "subscription_publish": commit.SubscriptionPublish}})
	default:
		return fmt.Errorf("unsupported rules operation %q", arguments[0])
	}
}
