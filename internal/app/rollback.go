package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runRollback(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: vpskit rollback <transaction-id> --yes")
	}
	flags := flag.NewFlagSet("rollback", flag.ContinueOnError)
	yes := flags.Bool("yes", false, "confirm restoring the transaction backup")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: vpskit rollback <transaction-id> --yes")
	}
	if !*yes {
		return errors.New("rollback requires explicit --yes confirmation")
	}
	transactionID := arguments[0]
	if filepath.Base(transactionID) != transactionID || !strings.HasPrefix(transactionID, "TX-") {
		return errors.New("invalid transaction id")
	}
	bytes, err := os.ReadFile(filepath.Join(transactionRoot, transactionID, "transaction.json"))
	if err != nil {
		return fmt.Errorf("read transaction record: %w", err)
	}
	var record struct {
		Command          string `json:"command"`
		Status           string `json:"status"`
		PreviousBackupID string `json:"previous_backup_id"`
	}
	if err := json.Unmarshal(bytes, &record); err != nil {
		return fmt.Errorf("parse transaction record: %w", err)
	}
	if record.PreviousBackupID == "" || (record.Status != "COMMITTED" && !(record.Status == "IN_PROGRESS" && record.Command == "update core")) {
		return errors.New("transaction has no committed rollback backup")
	}
	if record.Command != "update self" && record.Command != "update core" && !strings.HasPrefix(record.Command, "instance ") {
		return fmt.Errorf("transaction command is not rollback-compatible: %s", record.Command)
	}
	if strings.HasPrefix(record.Command, "instance ") {
		if err := verifyInstanceRollbackVersion(record.PreviousBackupID); err != nil {
			return err
		}
	}
	if err := restoreBackup(record.PreviousBackupID); err != nil {
		return err
	}
	return nil
}

func verifyInstanceRollbackVersion(backupID string) error {
	currentState, err := readInstalledState()
	if err != nil {
		return err
	}
	_, backupDirectory, err := loadBackup(backupID)
	if err != nil {
		return err
	}
	backupStateBytes, err := os.ReadFile(filepath.Join(backupDirectory, "state.json"))
	if err != nil {
		return fmt.Errorf("read instance rollback backup state: %w", err)
	}
	backupState, err := decodeInstalledState(backupStateBytes)
	if err != nil {
		return fmt.Errorf("decode instance rollback backup state: %w", err)
	}
	return validateInstanceRollbackVersions(currentState.VPSKitVersion, backupState.VPSKitVersion)
}

func validateInstanceRollbackVersions(currentVersion, backupVersion string) error {
	if currentVersion == "" || backupVersion == "" || currentVersion != backupVersion {
		return fmt.Errorf("refusing cross-version instance rollback from %s to %s; use explicit restore for a full-version rollback", currentVersion, backupVersion)
	}
	return nil
}
