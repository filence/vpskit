package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"vpskit.local/vpskit/internal/fsutil"
	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/platform"
)

type stateChangeCommit struct {
	TransactionID       string
	BackupID            string
	State               model.State
	SubscriptionPublish subscriptionPostCommit
}

func commitManagedStateChange(previous, updated model.State, secrets model.Secrets, command, backupSuffix string, changedFields []string) (result stateChangeCommit, returnErr error) {
	lock, err := platform.AcquireProcessLock(filepath.Join(stateRoot, "locks", "vpskit.lock"))
	if err != nil {
		return stateChangeCommit{}, err
	}
	defer lock.Close()

	backupID, err := createPersistentBackupLocked(previous, backupSuffix)
	if err != nil {
		return stateChangeCommit{}, fmt.Errorf("create rollback backup: %w", err)
	}
	transactionID := "TX-" + time.Now().UTC().Format("20060102-150405") + "-" + backupSuffix + "-" + randomHex(3)
	transactionDirectory := filepath.Join(transactionRoot, transactionID)
	stagingDirectory := filepath.Join(transactionDirectory, "staging")
	snapshotDirectory := filepath.Join(transactionDirectory, "backup")
	if err := os.MkdirAll(stagingDirectory, 0o700); err != nil {
		return stateChangeCommit{}, err
	}
	if err := os.MkdirAll(snapshotDirectory, 0o700); err != nil {
		return stateChangeCommit{}, err
	}
	snapshotEntries, err := captureManagedEntries(snapshotDirectory)
	if err != nil {
		return stateChangeCommit{}, err
	}

	updated.SchemaVersion = model.SchemaVersion
	secrets.SchemaVersion = model.SchemaVersion
	updated.TransactionID = transactionID
	updated.Profile = profileFromInboundState(updated.Reality.Enabled, updated.Hysteria2.Enabled)
	artifacts, clientSet, err := renderProfileArtifacts(updated, secrets)
	if err != nil {
		return stateChangeCommit{}, err
	}
	updated.ConfigSHA256 = sha256Bytes(artifacts[serverConfigPath])
	updated.RealityConfigSHA256 = sha256Bytes(artifacts[xrayServerConfigPath])
	updated.Exports = exportStateForProfile(updated)
	stateBytes, err := json.MarshalIndent(updated, "", "  ")
	if err != nil {
		return stateChangeCommit{}, err
	}
	secretBytes, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return stateChangeCommit{}, err
	}
	if err := stageProfileMutation(stagingDirectory, artifacts, stateBytes, secretBytes); err != nil {
		return stateChangeCommit{}, err
	}
	if output, err := runCommand(installedSingBox, "check", "-c", filepath.Join(stagingDirectory, "sing-box.json")); err != nil {
		return stateChangeCommit{}, fmt.Errorf("staged sing-box configuration check failed: %w: %s", err, sanitizeText(output, ""))
	}
	if output, err := runCommand(installedXray, "run", "-test", "-config", filepath.Join(stagingDirectory, "xray.json")); err != nil {
		return stateChangeCommit{}, fmt.Errorf("staged Xray configuration check failed: %w: %s", err, sanitizeText(output, ""))
	}

	progressRecord := map[string]any{
		"schema_version":           1,
		"transaction_id":           transactionID,
		"command":                  command,
		"status":                   "IN_PROGRESS",
		"started_at":               time.Now().UTC(),
		"previous_backup_id":       backupID,
		"previous_config_revision": previous.ConfigRevision,
		"config_revision":          updated.ConfigRevision,
		"changed_fields":           changedFields,
	}
	if err := writeTransactionRecord(transactionDirectory, progressRecord); err != nil {
		return stateChangeCommit{}, err
	}

	timerWasActive := execCommandSuccess("systemctl", "is-active", "--quiet", certificateRenewTimerUnitName) == nil
	mutated := false
	committed := false
	defer func() {
		if committed {
			_ = fsutil.RemoveManagedTree(stagingDirectory, transactionDirectory)
			_ = fsutil.RemoveManagedTree(snapshotDirectory, transactionDirectory)
			return
		}
		var rollbackError error
		if mutated {
			rollbackError = rollbackRestore(snapshotEntries, false, timerWasActive, snapshotDirectory, previous)
		}
		progressRecord["status"] = "ROLLED_BACK"
		progressRecord["failed_at"] = time.Now().UTC()
		progressRecord["error"] = sanitizeError(returnErr, "")
		if rollbackError != nil {
			progressRecord["status"] = "ROLLBACK_FAILED"
			progressRecord["rollback_error"] = sanitizeError(rollbackError, "")
		}
		_ = writeTransactionRecord(transactionDirectory, progressRecord)
		_ = fsutil.RemoveManagedTree(stagingDirectory, transactionDirectory)
		_ = fsutil.RemoveManagedTree(snapshotDirectory, transactionDirectory)
		if returnErr == nil && rollbackError != nil {
			returnErr = rollbackError
		}
	}()

	mutated = true
	if err := activateProfileMutation(artifacts, clientSet, stateBytes, secretBytes); err != nil {
		return stateChangeCommit{}, err
	}
	if output, err := runCommand(installedSingBox, "check", "-c", serverConfigPath); err != nil {
		return stateChangeCommit{}, fmt.Errorf("installed sing-box configuration check failed: %w: %s", err, sanitizeText(output, ""))
	}
	if output, err := runCommand(installedXray, "run", "-test", "-config", xrayServerConfigPath); err != nil {
		return stateChangeCommit{}, fmt.Errorf("installed Xray configuration check failed: %w: %s", err, sanitizeText(output, ""))
	}

	progressRecord["status"] = "COMMITTED"
	progressRecord["committed_at"] = time.Now().UTC()
	progressRecord["state_schema"] = updated.SchemaVersion
	if err := writeTransactionRecord(transactionDirectory, progressRecord); err != nil {
		return stateChangeCommit{}, err
	}
	committed = true
	subscriptionPublish := attemptAutoPublishSubscription(context.Background())
	_ = appendAudit(map[string]any{
		"time":           time.Now().UTC(),
		"transaction_id": transactionID,
		"command":        command,
		"status":         "COMMITTED",
		"backup_id":      backupID,
		"subscription":   subscriptionPublish.Status,
	})
	return stateChangeCommit{TransactionID: transactionID, BackupID: backupID, State: updated, SubscriptionPublish: subscriptionPublish}, nil
}
