package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/fsutil"
	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/platform"
	"vpskit.local/vpskit/internal/release"
)

const backupManifestName = "backup.json"

type backupManifest struct {
	SchemaVersion    int           `json:"schema_version"`
	BackupID         string        `json:"backup_id"`
	CreatedAt        time.Time     `json:"created_at"`
	StateTransaction string        `json:"state_transaction_id"`
	Entries          []backupEntry `json:"entries"`
}

type backupEntry struct {
	Source   string `json:"source"`
	Relative string `json:"relative"`
	Mode     uint32 `json:"mode"`
	SHA256   string `json:"sha256"`
}

type managedBackupSource struct {
	source   string
	relative string
	mode     os.FileMode
}

func runBackup(arguments []string) error {
	if len(arguments) != 0 {
		return errors.New("usage: vpskit backup")
	}
	if !platform.IsRoot() {
		return errors.New("backup requires root privileges")
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	lock, err := platform.AcquireProcessLock(filepath.Join(stateRoot, "locks", "vpskit.lock"))
	if err != nil {
		return err
	}
	defer lock.Close()
	backupID := "BK-" + time.Now().UTC().Format("20060102-150405") + "-" + randomHex(3)
	backupDirectory := filepath.Join(backupRoot, backupID)
	if err := os.MkdirAll(backupDirectory, 0o700); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	entries, err := captureManagedEntries(backupDirectory)
	if err != nil {
		_ = fsutil.RemoveManagedTree(backupDirectory, backupRoot)
		return err
	}
	manifest := backupManifest{
		SchemaVersion:    1,
		BackupID:         backupID,
		CreatedAt:        time.Now().UTC(),
		StateTransaction: state.TransactionID,
		Entries:          entries,
	}
	if err := writeBackupManifest(backupDirectory, manifest); err != nil {
		_ = fsutil.RemoveManagedTree(backupDirectory, backupRoot)
		return err
	}
	if err := pruneBackups(5, backupID); err != nil {
		return err
	}
	return printJSON(commandResult{Command: "backup", Status: "PASS", Detail: map[string]any{
		"backup_id":         backupID,
		"entries":           len(entries),
		"state_transaction": state.TransactionID,
	}})
}

func createPersistentBackupLocked(state model.State, suffix string) (string, error) {
	backupID := "BK-" + time.Now().UTC().Format("20060102-150405") + "-" + suffix + "-" + randomHex(3)
	backupDirectory := filepath.Join(backupRoot, backupID)
	if err := os.MkdirAll(backupDirectory, 0o700); err != nil {
		return "", err
	}
	entries, err := captureManagedEntries(backupDirectory)
	if err != nil {
		_ = fsutil.RemoveManagedTree(backupDirectory, backupRoot)
		return "", err
	}
	manifest := backupManifest{SchemaVersion: 1, BackupID: backupID, CreatedAt: time.Now().UTC(), StateTransaction: state.TransactionID, Entries: entries}
	if err := writeBackupManifest(backupDirectory, manifest); err != nil {
		_ = fsutil.RemoveManagedTree(backupDirectory, backupRoot)
		return "", err
	}
	if err := pruneBackups(5, backupID); err != nil {
		return "", err
	}
	return backupID, nil
}

func runRestore(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: vpskit restore <backup-id> --yes")
	}
	flags := flag.NewFlagSet("restore", flag.ContinueOnError)
	yes := flags.Bool("yes", false, "confirm restoring the selected backup")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: vpskit restore <backup-id> --yes")
	}
	if !*yes {
		return errors.New("restore requires explicit --yes confirmation")
	}
	if !platform.IsRoot() {
		return errors.New("restore requires root privileges")
	}
	return restoreBackup(arguments[0])
}

func captureManagedEntries(destination string) ([]backupEntry, error) {
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return nil, err
	}
	entries := make([]backupEntry, 0)
	for _, source := range managedBackupSources() {
		info, err := os.Lstat(source.source)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("inspect backup path %s: %w", source.source, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("refusing non-regular backup path: %s", source.source)
		}
		if err := copyBackupEntry(source.source, filepath.Join(destination, filepath.FromSlash(source.relative)), source.mode); err != nil {
			return nil, err
		}
		digest, err := release.FileSHA256(source.source)
		if err != nil {
			return nil, err
		}
		entries = append(entries, backupEntry{Source: source.source, Relative: source.relative, Mode: uint32(source.mode.Perm()), SHA256: digest})
	}
	if _, err := os.Stat(legoStateRoot); err == nil {
		err := filepath.WalkDir(legoStateRoot, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative, err := filepath.Rel(legoStateRoot, path)
			if err != nil {
				return err
			}
			if relative == "." {
				return nil
			}
			if entry.IsDir() {
				return os.MkdirAll(filepath.Join(destination, "lego", relative), 0o700)
			}
			if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
				return fmt.Errorf("refusing non-regular lego backup path: %s", path)
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			relativeSlash := filepath.ToSlash(filepath.Join("lego", relative))
			if err := copyBackupEntry(path, filepath.Join(destination, filepath.FromSlash(relativeSlash)), info.Mode().Perm()); err != nil {
				return err
			}
			digest, err := release.FileSHA256(path)
			if err != nil {
				return err
			}
			entries = append(entries, backupEntry{Source: path, Relative: relativeSlash, Mode: uint32(info.Mode().Perm()), SHA256: digest})
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Relative < entries[j].Relative })
	return entries, nil
}

func managedBackupSources() []managedBackupSource {
	return []managedBackupSource{
		{installedBinary, "bin/vpskit", 0o755},
		{installedSingBox, "bin/sing-box", 0o755},
		{installedXray, "bin/xray", 0o755},
		{installedLego, "bin/lego", 0o755},
		{versionsLockPath, "versions.lock", 0o644},
		{statePath, "state.json", 0o600},
		{ownershipPath, "ownership.json", 0o600},
		{serverConfigPath, "generated/sing-box.json", 0o640},
		{xrayServerConfigPath, "generated/xray.json", 0o640},
		{filepath.Join(exportRoot, "mihomo.yaml"), "exports/mihomo.yaml", 0o600},
		{filepath.Join(exportRoot, "sing-box-reality.json"), "exports/sing-box-reality.json", 0o600},
		{filepath.Join(exportRoot, "sing-box-hysteria2.json"), "exports/sing-box-hysteria2.json", 0o600},
		{filepath.Join(exportRoot, "share-links.txt"), "exports/share-links.txt", 0o600},
		{managedCertificate, "certificates/hysteria2.crt", 0o640},
		{managedKey, "certificates/hysteria2.key", 0o640},
		{secretPath, "secrets/instances.json", 0o600},
		{serviceUnitPath, "units/vpskit-sing-box.service", 0o644},
		{xrayServiceUnitPath, "units/vpskit-xray.service", 0o644},
		{certificateRenewServiceUnitPath, "units/vpskit-certificate-renew.service", 0o644},
		{certificateRenewTimerUnitPath, "units/vpskit-certificate-renew.timer", 0o644},
	}
}

func copyBackupEntry(source, destination string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	if err := fsutil.CopyFileAtomic(source, destination, mode); err != nil {
		return fmt.Errorf("copy backup entry %s: %w", source, err)
	}
	return nil
}

func writeBackupManifest(directory string, manifest backupManifest) error {
	bytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(filepath.Join(directory, backupManifestName), append(bytes, '\n'), 0o600)
}

func loadBackup(backupID string) (backupManifest, string, error) {
	if backupID == "" || filepath.Base(backupID) != backupID || !strings.HasPrefix(backupID, "BK-") {
		return backupManifest{}, "", errors.New("invalid backup id")
	}
	directory := filepath.Join(backupRoot, backupID)
	bytes, err := os.ReadFile(filepath.Join(directory, backupManifestName))
	if err != nil {
		return backupManifest{}, "", fmt.Errorf("read backup manifest: %w", err)
	}
	var manifest backupManifest
	if err := json.Unmarshal(bytes, &manifest); err != nil {
		return backupManifest{}, "", fmt.Errorf("parse backup manifest: %w", err)
	}
	if manifest.SchemaVersion != 1 || manifest.BackupID != backupID || len(manifest.Entries) == 0 {
		return backupManifest{}, "", errors.New("invalid backup manifest")
	}
	seen := map[string]bool{}
	for _, entry := range manifest.Entries {
		if !validBackupRelative(entry.Relative) || seen[entry.Relative] {
			return backupManifest{}, "", fmt.Errorf("invalid backup entry: %s", entry.Relative)
		}
		seen[entry.Relative] = true
		if !validBackupSource(entry.Source, entry.Relative) {
			return backupManifest{}, "", fmt.Errorf("backup source is outside managed set: %s", entry.Source)
		}
		path := filepath.Join(directory, filepath.FromSlash(entry.Relative))
		digest, err := release.FileSHA256(path)
		if err != nil || !strings.EqualFold(digest, entry.SHA256) {
			return backupManifest{}, "", fmt.Errorf("backup digest mismatch: %s", entry.Relative)
		}
	}
	return manifest, directory, nil
}

func validBackupRelative(relative string) bool {
	return relative != "" && filepath.Base(filepath.FromSlash(relative)) != "." && !strings.HasPrefix(relative, "/") && !strings.Contains(relative, "..")
}

func validBackupSource(source, relative string) bool {
	for _, allowed := range managedBackupSources() {
		if source == allowed.source && relative == allowed.relative {
			return true
		}
	}
	return strings.HasPrefix(relative, "lego/") && strings.HasPrefix(source, legoStateRoot+string(os.PathSeparator))
}

func pruneBackups(limit int, additionallyProtected ...string) error {
	entries, err := os.ReadDir(backupRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	ids := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "BK-") {
			ids = append(ids, entry.Name())
		}
	}
	sort.Strings(ids)
	protected := referencedBackupIDs()
	for _, backupID := range additionallyProtected {
		protected[backupID] = true
	}
	for len(ids) > limit {
		removeIndex := oldestRemovableBackup(ids, protected)
		if removeIndex < 0 {
			break
		}
		if err := fsutil.RemoveManagedTree(filepath.Join(backupRoot, ids[removeIndex]), backupRoot); err != nil {
			return err
		}
		ids = append(ids[:removeIndex], ids[removeIndex+1:]...)
	}
	return nil
}

func oldestRemovableBackup(ids []string, protected map[string]bool) int {
	for index, id := range ids {
		if !protected[id] {
			return index
		}
	}
	return -1
}

func referencedBackupIDs() map[string]bool {
	protected := make(map[string]bool)
	entries, err := os.ReadDir(transactionRoot)
	if err != nil {
		return protected
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "TX-") {
			continue
		}
		bytes, err := os.ReadFile(filepath.Join(transactionRoot, entry.Name(), "transaction.json"))
		if err != nil {
			continue
		}
		var record struct {
			Status           string `json:"status"`
			PreviousBackupID string `json:"previous_backup_id"`
		}
		if json.Unmarshal(bytes, &record) == nil && record.PreviousBackupID != "" && (record.Status == "COMMITTED" || record.Status == "IN_PROGRESS") {
			protected[record.PreviousBackupID] = true
		}
	}
	return protected
}

func restoreBackup(backupID string) (returnErr error) {
	return restoreBackupWithOutput(backupID, true)
}

func restoreBackupWithOutput(backupID string, emitResult bool) (returnErr error) {
	manifest, backupDirectory, err := loadBackup(backupID)
	if err != nil {
		return err
	}
	currentState, err := readInstalledState()
	if err != nil {
		return err
	}
	lock, err := platform.AcquireProcessLock(filepath.Join(stateRoot, "locks", "vpskit.lock"))
	if err != nil {
		return err
	}
	defer lock.Close()
	transactionID := "TX-" + time.Now().UTC().Format("20060102-150405") + "-restore-" + randomHex(3)
	transactionDirectory := filepath.Join(transactionRoot, transactionID)
	stagingDirectory := filepath.Join(transactionDirectory, "staging")
	snapshotDirectory := filepath.Join(transactionDirectory, "backup")
	if err := os.MkdirAll(stagingDirectory, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(snapshotDirectory, 0o700); err != nil {
		return err
	}
	defer func() {
		_ = fsutil.RemoveManagedTree(stagingDirectory, transactionDirectory)
		_ = fsutil.RemoveManagedTree(snapshotDirectory, transactionDirectory)
	}()
	snapshotEntries, err := captureManagedEntries(snapshotDirectory)
	if err != nil {
		return err
	}
	for _, entry := range manifest.Entries {
		if err := copyBackupEntry(filepath.Join(backupDirectory, filepath.FromSlash(entry.Relative)), filepath.Join(stagingDirectory, filepath.FromSlash(entry.Relative)), os.FileMode(entry.Mode)); err != nil {
			return err
		}
	}
	restoredState, err := readStateFromBackup(stagingDirectory, manifest)
	if err != nil {
		return err
	}
	configPath := filepath.Join(stagingDirectory, "generated/sing-box.json")
	xrayConfigPath := filepath.Join(stagingDirectory, "generated/xray.json")
	certPath := filepath.Join(stagingDirectory, "certificates/hysteria2.crt")
	keyPath := filepath.Join(stagingDirectory, "certificates/hysteria2.key")
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read staged backup configuration: %w", err)
	}
	actualConfigHash := sha256Hex(configBytes)
	if actualConfigHash != restoredState.ConfigSHA256 {
		return fmt.Errorf("backup configuration hash does not match backup state: actual=%s expected=%s", actualConfigHash, restoredState.ConfigSHA256)
	}
	xrayConfigBytes, err := os.ReadFile(xrayConfigPath)
	if err != nil {
		return fmt.Errorf("read staged Xray backup configuration: %w", err)
	}
	if actualHash := sha256Hex(xrayConfigBytes); actualHash != restoredState.RealityConfigSHA256 {
		return fmt.Errorf("backup Xray configuration hash does not match backup state: actual=%s expected=%s", actualHash, restoredState.RealityConfigSHA256)
	}
	if restoredState.Hysteria2.ID != "" {
		if err := validateCertificate(certPath, keyPath, restoredState.Domain); err != nil {
			return fmt.Errorf("backup certificate validation failed: %w", err)
		}
	}
	stagedSingBox := filepath.Join(stagingDirectory, "bin", "sing-box")
	stagedXray := filepath.Join(stagingDirectory, "bin", "xray")
	checkConfigPath := configPath
	if restoredState.Hysteria2.ID != "" {
		validationConfig, err := configWithStagedCertificatePaths(configBytes, certPath, keyPath)
		if err != nil {
			return fmt.Errorf("prepare staged backup configuration: %w", err)
		}
		checkConfigPath = filepath.Join(stagingDirectory, "validation-sing-box.json")
		if err := fsutil.WriteFileAtomic(checkConfigPath, validationConfig, 0o600); err != nil {
			return err
		}
	}
	if output, err := runCommand(stagedSingBox, "check", "-c", checkConfigPath); err != nil {
		return fmt.Errorf("backup sing-box configuration check failed: %w: %s", err, output)
	}
	if output, err := runCommand(stagedXray, "run", "-test", "-config", xrayConfigPath); err != nil {
		return fmt.Errorf("backup Xray configuration check failed: %w: %s", err, output)
	}
	ownership, err := readJSONMap(filepath.Join(stagingDirectory, "ownership.json"))
	if err != nil {
		return err
	}
	restoredState.TransactionID = transactionID
	ownership["transaction_id"] = transactionID
	stateBytes, err := json.MarshalIndent(restoredState, "", "  ")
	if err != nil {
		return err
	}
	ownershipBytes, err := json.MarshalIndent(ownership, "", "  ")
	if err != nil {
		return err
	}
	timerWasActive := execCommandSuccess("systemctl", "is-active", "--quiet", certificateRenewTimerUnitName) == nil
	mutated := false
	restartAttempted := false
	committed := false
	defer func() {
		if committed {
			return
		}
		var rollbackError error
		if mutated {
			rollbackError = rollbackRestore(snapshotEntries, restartAttempted, timerWasActive, snapshotDirectory, currentState)
		}
		status := "ROLLED_BACK"
		if rollbackError != nil {
			status = "ROLLBACK_FAILED"
		}
		record := map[string]any{"schema_version": 1, "transaction_id": transactionID, "command": "restore", "status": status, "failed_at": time.Now().UTC(), "backup_id": backupID, "error": sanitizeError(returnErr, "")}
		if rollbackError != nil {
			record["rollback_error"] = sanitizeError(rollbackError, "")
		}
		_ = writeTransactionRecord(transactionDirectory, record)
		if returnErr == nil && rollbackError != nil {
			returnErr = rollbackError
		}
	}()

	mutated = true
	if err := applyRestoreEntries(stagingDirectory, manifest.Entries, stateBytes, ownershipBytes); err != nil {
		return err
	}
	if output, err := runCommand(installedSingBox, "check", "-c", serverConfigPath); err != nil {
		return fmt.Errorf("restored sing-box configuration check failed: %w: %s", err, output)
	}
	if output, err := runCommand(installedXray, "run", "-test", "-config", xrayServerConfigPath); err != nil {
		return fmt.Errorf("restored Xray configuration check failed: %w: %s", err, output)
	}
	if restoredState.Hysteria2.ID != "" {
		if _, err := readCertificateDetails(restoredState.Domain); err != nil {
			return fmt.Errorf("restored managed certificate validation failed: %w", err)
		}
	}
	if output, err := runCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("restore daemon-reload failed: %w: %s", err, output)
	}
	if restoredState.Hysteria2.ID != "" {
		if _, err := os.Stat(certificateRenewTimerUnitPath); err == nil {
			if output, err := runCommand("systemctl", "enable", "--now", certificateRenewTimerUnitName); err != nil {
				return fmt.Errorf("restore renewal timer failed: %w: %s", err, output)
			}
		}
	}
	restartAttempted = true
	if err := restartManagedServices(restoredState); err != nil {
		return fmt.Errorf("restore service restart failed: %w", err)
	}
	if err := waitForManagedServiceState(restoredState, 30*time.Second); err != nil {
		return fmt.Errorf("restore service health check failed: %w", err)
	}
	record := map[string]any{"schema_version": 1, "transaction_id": transactionID, "command": "restore", "status": "COMMITTED", "committed_at": time.Now().UTC(), "backup_id": backupID, "config_sha256": restoredState.ConfigSHA256, "reality_config_sha256": restoredState.RealityConfigSHA256}
	if err := writeTransactionRecord(transactionDirectory, record); err != nil {
		return err
	}
	committed = true
	_ = appendAudit(map[string]any{"time": time.Now().UTC(), "transaction_id": transactionID, "command": "restore", "status": "COMMITTED", "backup_id": backupID})
	if !emitResult {
		return nil
	}
	return printJSON(commandResult{Command: "restore", Status: "PASS", Detail: map[string]any{"result": "RESTORED", "transaction_id": transactionID, "backup_id": backupID}})
}

func configWithStagedCertificatePaths(configBytes []byte, certificatePath, keyPath string) ([]byte, error) {
	var document map[string]any
	if err := json.Unmarshal(configBytes, &document); err != nil {
		return nil, fmt.Errorf("parse backed up sing-box configuration: %w", err)
	}
	inbounds, ok := document["inbounds"].([]any)
	if !ok {
		return nil, errors.New("backed up sing-box configuration has no inbound list")
	}
	updated := false
	for _, rawInbound := range inbounds {
		inbound, ok := rawInbound.(map[string]any)
		if !ok || inbound["type"] != "hysteria2" {
			continue
		}
		tlsSettings, ok := inbound["tls"].(map[string]any)
		if !ok {
			return nil, errors.New("backed up Hysteria2 inbound has no TLS settings")
		}
		tlsSettings["certificate_path"] = certificatePath
		tlsSettings["key_path"] = keyPath
		updated = true
	}
	if !updated {
		return nil, errors.New("backed up configuration has no Hysteria2 inbound")
	}
	output, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(output, '\n'), nil
}

func readStateFromBackup(stagingDirectory string, manifest backupManifest) (model.State, error) {
	for _, entry := range manifest.Entries {
		if entry.Source != statePath {
			continue
		}
		bytes, err := os.ReadFile(filepath.Join(stagingDirectory, filepath.FromSlash(entry.Relative)))
		if err != nil {
			return model.State{}, err
		}
		state, err := decodeInstalledState(bytes)
		if err != nil || (state.Reality.ID == "" && state.Hysteria2.ID == "") {
			return model.State{}, errors.New("backup state schema is not supported")
		}
		return state, nil
	}
	return model.State{}, errors.New("backup does not contain state.json")
}

func readJSONMap(path string) (map[string]any, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var value map[string]any
	if err := json.Unmarshal(bytes, &value); err != nil {
		return nil, err
	}
	if value == nil {
		return nil, errors.New("JSON object is empty")
	}
	return value, nil
}

func applyRestoreEntries(stagingDirectory string, entries []backupEntry, stateBytes, ownershipBytes []byte) error {
	presentSources := make(map[string]bool, len(entries))
	for _, entry := range entries {
		presentSources[entry.Source] = true
	}
	for _, managed := range managedBackupSources() {
		if presentSources[managed.source] {
			continue
		}
		if err := os.Remove(managed.source); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove managed path absent from backup %s: %w", managed.source, err)
		}
	}
	for _, entry := range entries {
		if entry.Source == statePath || entry.Source == ownershipPath || strings.HasPrefix(entry.Relative, "lego/") {
			continue
		}
		if err := copyBackupEntry(filepath.Join(stagingDirectory, filepath.FromSlash(entry.Relative)), entry.Source, os.FileMode(entry.Mode)); err != nil {
			return err
		}
	}
	legoSource := filepath.Join(stagingDirectory, "lego")
	if _, err := os.Stat(legoSource); err == nil {
		if err := fsutil.RemoveManagedTree(legoStateRoot, stateRoot); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := copyManagedTree(legoSource, legoStateRoot); err != nil {
			return err
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if err := fsutil.RemoveManagedTree(legoStateRoot, stateRoot); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else {
		return err
	}
	if err := fsutil.WriteFileAtomic(statePath, append(stateBytes, '\n'), 0o600); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(ownershipPath, append(ownershipBytes, '\n'), 0o600); err != nil {
		return err
	}
	if err := setServiceDirectoryModes(); err != nil {
		return err
	}
	if err := setServiceFileOwnership(); err != nil {
		return err
	}
	return verifyServiceReadAccess()
}

func rollbackRestore(snapshotEntries []backupEntry, restartAttempted, timerWasActive bool, snapshotDirectory string, previousState model.State) error {
	var rollbackErrors []string
	_, _ = runCommand("systemctl", "disable", "--now", certificateRenewTimerUnitName)
	if err := applyRestoreEntries(snapshotDirectory, snapshotEntries, mustReadFile(filepath.Join(snapshotDirectory, "state.json")), mustReadFile(filepath.Join(snapshotDirectory, "ownership.json"))); err != nil {
		rollbackErrors = append(rollbackErrors, "restore previous files: "+err.Error())
	}
	if output, err := runCommand("systemctl", "daemon-reload"); err != nil {
		rollbackErrors = append(rollbackErrors, "daemon-reload: "+output)
	}
	if timerWasActive {
		if output, err := runCommand("systemctl", "enable", "--now", certificateRenewTimerUnitName); err != nil {
			rollbackErrors = append(rollbackErrors, "restore timer: "+output)
		}
	}
	if restartAttempted {
		if err := restartManagedServices(previousState); err != nil {
			rollbackErrors = append(rollbackErrors, "restart services: "+err.Error())
		} else if err := waitForManagedServiceState(previousState, 30*time.Second); err != nil {
			rollbackErrors = append(rollbackErrors, "health check: "+err.Error())
		}
	}
	return joinUpdateErrors(rollbackErrors)
}

func mustReadFile(path string) []byte {
	bytes, _ := os.ReadFile(path)
	return bytes
}
