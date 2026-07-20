package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/fsutil"
	"vpskit.local/vpskit/internal/platform"
	"vpskit.local/vpskit/internal/release"
)

type incompleteTransaction struct {
	ID               string
	Path             string
	Command          string
	PreviousBackupID string
	PreviousCoreSHA  string
	NewCoreSHA       string
}

func runRecover(arguments []string) error {
	if len(arguments) != 0 {
		return errors.New("usage: vpskit recover")
	}
	if !platform.IsRoot() {
		return errors.New("recovery requires root privileges")
	}
	recovered, err := recoverIncompleteMutation()
	if err != nil {
		return err
	}
	result := "NOT_NEEDED"
	transactionID := ""
	if recovered != nil {
		result = "RECOVERED"
		transactionID = recovered.ID
	}
	return printJSON(commandResult{Command: "recover", Status: "PASS", Detail: map[string]any{
		"result":         result,
		"transaction_id": transactionID,
	}})
}

func recoverIncompleteMutation() (*incompleteTransaction, error) {
	incomplete, err := findIncompleteMutations()
	if err != nil || len(incomplete) == 0 {
		return nil, err
	}
	target := incomplete[len(incomplete)-1]
	if target.PreviousBackupID == "" {
		return nil, fmt.Errorf("incomplete transaction %s has no persistent recovery backup", target.ID)
	}
	recoverySource := "persistent-backup"
	persistentManifest := filepath.Join(backupRoot, target.PreviousBackupID, backupManifestName)
	if _, err := os.Stat(persistentManifest); err == nil {
		if err := restoreBackupWithOutput(target.PreviousBackupID, false); err != nil {
			return nil, fmt.Errorf("recover incomplete transaction %s: %w", target.ID, err)
		}
	} else if errors.Is(err, os.ErrNotExist) && target.Command == "update core" {
		if err := recoverCoreFromTransactionBackup(target); err != nil {
			return nil, err
		}
		recoverySource = "transaction-local-backup"
	} else if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("incomplete transaction %s has no persistent recovery backup", target.ID)
	} else {
		return nil, err
	}
	bytes, err := os.ReadFile(target.Path)
	if err != nil {
		return nil, err
	}
	var record map[string]any
	if err := json.Unmarshal(bytes, &record); err != nil {
		return nil, err
	}
	record["status"] = "RECOVERED"
	record["recovered_at"] = time.Now().UTC()
	record["recovery_backup_id"] = target.PreviousBackupID
	record["recovery_source"] = recoverySource
	if err := writeTransactionRecord(filepath.Dir(target.Path), record); err != nil {
		return nil, err
	}
	transactionDirectory := filepath.Dir(target.Path)
	if err := fsutil.RemoveManagedTree(filepath.Join(transactionDirectory, "staging"), transactionDirectory); err != nil {
		return nil, err
	}
	if recoverySource == "persistent-backup" {
		if err := fsutil.RemoveManagedTree(filepath.Join(transactionDirectory, "backup"), transactionDirectory); err != nil {
			return nil, err
		}
	}
	_ = appendAudit(map[string]any{
		"time":           time.Now().UTC(),
		"transaction_id": target.ID,
		"command":        "recover " + target.Command,
		"status":         "RECOVERED",
		"backup_id":      target.PreviousBackupID,
	})
	return &target, nil
}

func findIncompleteMutations() ([]incompleteTransaction, error) {
	entries, err := os.ReadDir(transactionRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	values := make([]incompleteTransaction, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(transactionRoot, entry.Name(), "transaction.json")
		bytes, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var record struct {
			TransactionID   string `json:"transaction_id"`
			Command         string `json:"command"`
			Status          string `json:"status"`
			PreviousBackup  string `json:"previous_backup_id"`
			PreviousCoreSHA string `json:"previous_core_sha256"`
			NewCoreSHA      string `json:"new_core_sha256"`
		}
		if json.Unmarshal(bytes, &record) != nil || record.Status != "IN_PROGRESS" || (record.Command != "update core" && !strings.HasPrefix(record.Command, "instance ")) {
			continue
		}
		values = append(values, incompleteTransaction{ID: record.TransactionID, Path: path, Command: record.Command, PreviousBackupID: record.PreviousBackup, PreviousCoreSHA: record.PreviousCoreSHA, NewCoreSHA: record.NewCoreSHA})
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return values, nil
}

func recoverCoreFromTransactionBackup(target incompleteTransaction) error {
	backupDirectory := filepath.Join(filepath.Dir(target.Path), "backup")
	coreBackup := filepath.Join(backupDirectory, "sing-box")
	digest, err := release.FileSHA256(coreBackup)
	if err != nil {
		return fmt.Errorf("recover incomplete core transaction %s from local backup: %w", target.ID, err)
	}
	if target.PreviousCoreSHA == "" || !strings.EqualFold(digest, target.PreviousCoreSHA) {
		return fmt.Errorf("recover incomplete core transaction %s: local core backup digest mismatch", target.ID)
	}
	for _, required := range []string{"state.json", "ownership.json"} {
		info, err := os.Lstat(filepath.Join(backupDirectory, required))
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("recover incomplete core transaction %s: local backup is missing %s", target.ID, required)
		}
	}
	previousStateBytes, err := os.ReadFile(filepath.Join(backupDirectory, "state.json"))
	if err != nil {
		return err
	}
	previousState, err := decodeInstalledState(previousStateBytes)
	if err != nil {
		return err
	}
	lockExisted := false
	lockPath := filepath.Join(backupDirectory, "versions.lock")
	if info, err := os.Lstat(lockPath); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("recover incomplete core transaction %s: local versions lock backup is not regular", target.ID)
		}
		lockExisted = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	lock, err := platform.AcquireProcessLock(filepath.Join(stateRoot, "locks", "vpskit.lock"))
	if err != nil {
		return err
	}
	defer lock.Close()
	coreReplaced := !strings.EqualFold(target.PreviousCoreSHA, target.NewCoreSHA)
	if err := rollbackCoreUpdate(true, coreReplaced, true, lockExisted, backupDirectory, previousState); err != nil {
		return fmt.Errorf("recover incomplete core transaction %s from local backup: %w", target.ID, err)
	}
	return nil
}

func shouldAutoRecover(command string) bool {
	switch command {
	case "update", "cert", "backup", "install", "instance", "uninstall":
		return true
	default:
		return false
	}
}
