package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/platform"
)

type migrationPlan struct {
	SourceSchema      int      `json:"source_schema"`
	TargetSchema      int      `json:"target_schema"`
	MigrationRequired bool     `json:"migration_required"`
	WriteRequired     bool     `json:"write_required"`
	Steps             []string `json:"steps"`
	Rollback          string   `json:"rollback"`
}

func runMigrate(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: vpskit migrate <check|plan|apply>")
	}
	if !platform.IsRoot() {
		return errors.New("state migration requires root privileges")
	}
	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		return fmt.Errorf("read installed state for migration: %w", err)
	}
	envelope, err := decodeStateEnvelope(stateBytes)
	if err != nil {
		return err
	}
	plan, err := buildMigrationPlan(envelope.SchemaVersion)
	if err != nil {
		return err
	}
	switch arguments[0] {
	case "check":
		if len(arguments) != 1 {
			return errors.New("usage: vpskit migrate check")
		}
		return printJSON(commandResult{Command: "migrate check", Status: "PASS", Detail: map[string]any{
			"compatible":         true,
			"source_schema":      plan.SourceSchema,
			"target_schema":      plan.TargetSchema,
			"migration_required": plan.MigrationRequired,
			"write_performed":    false,
		}})
	case "plan":
		if len(arguments) != 1 {
			return errors.New("usage: vpskit migrate plan")
		}
		return printJSON(commandResult{Command: "migrate plan", Status: "PASS", Detail: plan})
	case "apply":
		flags := flag.NewFlagSet("migrate apply", flag.ContinueOnError)
		yes := flags.Bool("yes", false, "confirm state schema migration")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 || !*yes {
			return errors.New("migrate apply requires explicit --yes confirmation")
		}
		if !plan.MigrationRequired {
			return printJSON(commandResult{Command: "migrate apply", Status: "PASS", Detail: map[string]any{
				"result":          "ALREADY_CURRENT",
				"state_schema":    model.SchemaVersion,
				"write_performed": false,
				"config_revision": currentConfigRevision(stateBytes),
			}})
		}
		if err := requireInstalledMigrationExecutable(); err != nil {
			return err
		}
		var previous model.State
		if err := json.Unmarshal(stateBytes, &previous); err != nil {
			return fmt.Errorf("parse previous state: %w", err)
		}
		updated, err := migrateState(previous)
		if err != nil {
			return err
		}
		if err := validateNodeMetadata(updated.Node); err != nil {
			return fmt.Errorf("validate migrated node metadata: %w", err)
		}
		secrets, err := readInstalledSecrets()
		if err != nil {
			return err
		}
		commit, err := commitManagedStateChange(previous, updated, secrets, "migrate apply", "migrate", []string{"state.schema", "node.metadata"})
		if err != nil {
			return err
		}
		return printJSON(commandResult{Command: "migrate apply", Status: "PASS", Detail: map[string]any{
			"result":                 "MIGRATED",
			"source_schema":          plan.SourceSchema,
			"state_schema":           commit.State.SchemaVersion,
			"config_revision":        commit.State.ConfigRevision,
			"client_update_required": false,
			"transaction_id":         commit.TransactionID,
			"previous_backup_id":     commit.BackupID,
			"subscription_publish":   commit.SubscriptionPublish,
		}})
	default:
		return fmt.Errorf("unsupported migrate operation %q", arguments[0])
	}
}

func requireInstalledMigrationExecutable() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve migration executable: %w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}
	installed := installedBinary
	if resolved, resolveErr := filepath.EvalSymlinks(installedBinary); resolveErr == nil {
		installed = resolved
	}
	if filepath.Clean(executable) != filepath.Clean(installed) {
		return fmt.Errorf("migrate apply must run from installed binary %s; refusing state migration from temporary executable %s", installedBinary, executable)
	}
	return nil
}

func buildMigrationPlan(source int) (migrationPlan, error) {
	if source < 1 {
		return migrationPlan{}, errors.New("state schema_version must be at least 1")
	}
	if source > model.SchemaVersion {
		return migrationPlan{}, fmt.Errorf("state schema %d is newer than supported schema %d; refusing to rewrite it", source, model.SchemaVersion)
	}
	steps := make([]string, 0, model.SchemaVersion-source)
	for version := source; version < model.SchemaVersion; version++ {
		switch version {
		case 1:
			steps = append(steps, "schema 1 -> 2: pin core channel")
		case 2:
			steps = append(steps, "schema 2 -> 3: add explicit inbound enable state")
		case 3:
			steps = append(steps, "schema 3 -> 4: add separately pinned Xray core")
		case 4:
			steps = append(steps, "schema 4 -> 5: preserve client config revision")
		case 5:
			steps = append(steps, "schema 5 -> 6: add stable node metadata and Renderer/Publisher state boundary")
		case 6:
			steps = append(steps, "schema 6 -> 7: add opt-in client rule profile state")
		case 7:
			steps = append(steps, "schema 7 -> 8: record direct or managed rule source mode")
		case 8:
			steps = append(steps, "schema 8 -> 9: add owner whitelist and custom client rules")
		case 9:
			steps = append(steps, "schema 9 -> 10: add protocol-neutral instance inventory and adapter ownership")
		}
	}
	return migrationPlan{
		SourceSchema:      source,
		TargetSchema:      model.SchemaVersion,
		MigrationRequired: source != model.SchemaVersion,
		WriteRequired:     source != model.SchemaVersion,
		Steps:             steps,
		Rollback:          "automatic transaction rollback plus persistent pre-migrate backup",
	}, nil
}

func currentConfigRevision(stateBytes []byte) int {
	var state model.State
	if json.Unmarshal(stateBytes, &state) != nil {
		return 0
	}
	return state.ConfigRevision
}
