package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/fsutil"
	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/platform"
	"vpskit.local/vpskit/internal/release"
)

type updateManagedFile struct {
	path   string
	name   string
	mode   os.FileMode
	exists bool
}

func runUpdate(arguments []string, publicKeyBase64 string) error {
	if len(arguments) == 0 {
		return errors.New("usage: vpskit update <self|core> --bundle-dir <dir>")
	}
	switch arguments[0] {
	case "self":
		flags := flag.NewFlagSet("update self", flag.ContinueOnError)
		bundleDir := flags.String("bundle-dir", "", "verified bundle directory")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*bundleDir) == "" {
			return errors.New("--bundle-dir is required")
		}
		return updateSelf(strings.TrimSpace(*bundleDir), publicKeyBase64)
	case "core":
		flags := flag.NewFlagSet("update core", flag.ContinueOnError)
		bundleDir := flags.String("bundle-dir", "", "verified bundle directory")
		channel := flags.String("channel", "stable", "allowed channel: stable, candidate, beta, or pinned")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*bundleDir) == "" {
			return errors.New("--bundle-dir is required")
		}
		return updateCore(strings.TrimSpace(*bundleDir), strings.ToLower(strings.TrimSpace(*channel)), publicKeyBase64)
	default:
		return errors.New("usage: vpskit update <self|core> --bundle-dir <dir>")
	}
}

func updateSelf(bundleDir, publicKeyBase64 string) (returnErr error) {
	if !platform.IsRoot() {
		return errors.New("update requires root privileges")
	}
	if publicKeyBase64 == "" {
		return errors.New("this build has no embedded release public key")
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	previousDiskSchema, err := installedStateSchemaVersion()
	if err != nil {
		return err
	}
	if err := healthCheckState(state); err != nil {
		return fmt.Errorf("refusing update while active service is unhealthy: %w", err)
	}
	manifest, err := verifyBundle(bundleDir, publicKeyBase64)
	if err != nil {
		return fmt.Errorf("verify signed update bundle: %w", err)
	}
	vpskitAsset, err := release.FindAsset(manifest, "vpskit")
	if err != nil {
		return err
	}
	singBoxAsset, err := release.FindAsset(manifest, "sing-box")
	if err != nil {
		return err
	}
	xrayAsset, err := release.FindAsset(manifest, "xray")
	if err != nil {
		return err
	}
	legoAsset, err := release.FindAsset(manifest, "lego")
	if err != nil {
		return err
	}
	versionsLockAsset, err := release.FindAsset(manifest, "versions-lock")
	if err != nil {
		return err
	}
	if _, err := validateVersionsLock(bundleDir, versionsLockAsset, singBoxAsset, xrayAsset, legoAsset); err != nil {
		return err
	}
	lock, err := platform.AcquireProcessLock(filepath.Join(stateRoot, "locks", "vpskit.lock"))
	if err != nil {
		return err
	}
	defer lock.Close()
	previousBackupID, err := createPersistentBackupLocked(state, "pre-update")
	if err != nil {
		return fmt.Errorf("create pre-update backup: %w", err)
	}

	transactionID := "TX-" + time.Now().UTC().Format("20060102-150405") + "-update-self-" + randomHex(3)
	transactionDirectory := filepath.Join(transactionRoot, transactionID)
	stagingDirectory := filepath.Join(transactionDirectory, "staging")
	backupDirectory := filepath.Join(transactionDirectory, "backup")
	if err := os.MkdirAll(stagingDirectory, 0o700); err != nil {
		return fmt.Errorf("create update staging: %w", err)
	}
	if err := os.MkdirAll(backupDirectory, 0o700); err != nil {
		return fmt.Errorf("create update backup: %w", err)
	}
	_ = os.Chmod(transactionDirectory, 0o700)
	_ = os.Chmod(stagingDirectory, 0o700)
	_ = os.Chmod(backupDirectory, 0o700)
	preserveTransactionArtifacts := false
	defer func() {
		if preserveTransactionArtifacts {
			return
		}
		_ = fsutil.RemoveManagedTree(stagingDirectory, transactionDirectory)
		_ = fsutil.RemoveManagedTree(backupDirectory, transactionDirectory)
	}()
	hasCertificateLifecycle := state.Hysteria2.ID != ""
	timerWasActive := execCommandSuccess("systemctl", "is-active", "--quiet", certificateRenewTimerUnitName) == nil

	managedFiles := []updateManagedFile{
		{path: installedBinary, name: "vpskit", mode: 0o755},
		{path: installedSingBox, name: "sing-box", mode: 0o755},
		{path: installedXray, name: "xray", mode: 0o755},
		{path: installedLego, name: "lego", mode: 0o755},
		{path: versionsLockPath, name: "versions.lock", mode: 0o644},
		{path: serviceUnitPath, name: "vpskit-sing-box.service", mode: 0o644},
		{path: xrayServiceUnitPath, name: "vpskit-xray.service", mode: 0o644},
		{path: certificateRenewServiceUnitPath, name: "vpskit-certificate-renew.service", mode: 0o644},
		{path: certificateRenewTimerUnitPath, name: "vpskit-certificate-renew.timer", mode: 0o644},
		{path: serverConfigPath, name: "generated-sing-box.json", mode: 0o640},
		{path: xrayServerConfigPath, name: "generated-xray.json", mode: 0o640},
		{path: secretPath, name: "instances.json", mode: 0o600},
		{path: filepath.Join(exportRoot, "mihomo.yaml"), name: "mihomo.yaml", mode: 0o600},
		{path: filepath.Join(exportRoot, "sing-box-reality.json"), name: "sing-box-reality.json", mode: 0o600},
		{path: filepath.Join(exportRoot, "sing-box-hysteria2.json"), name: "sing-box-hysteria2.json", mode: 0o600},
		{path: filepath.Join(exportRoot, "share-links.txt"), name: "share-links.txt", mode: 0o600},
		{path: statePath, name: "state.json", mode: 0o600},
		{path: ownershipPath, name: "ownership.json", mode: 0o600},
	}
	for index := range managedFiles {
		managedFiles[index].exists, err = backupUpdateFile(managedFiles[index].path, filepath.Join(backupDirectory, managedFiles[index].name), managedFiles[index].mode)
		if err != nil {
			return err
		}
	}
	for _, asset := range []struct {
		manifest release.Asset
		name     string
		mode     os.FileMode
	}{
		{vpskitAsset, "vpskit", 0o755},
		{singBoxAsset, "sing-box", 0o755},
		{xrayAsset, "xray", 0o755},
		{legoAsset, "lego", 0o755},
		{versionsLockAsset, "versions.lock", 0o644},
	} {
		staged := filepath.Join(stagingDirectory, asset.name)
		if err := fsutil.CopyFileAtomic(filepath.Join(bundleDir, asset.manifest.File), staged, asset.mode); err != nil {
			return err
		}
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(stagingDirectory, "vpskit-sing-box.service"), []byte(systemdUnit()), 0o644); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(stagingDirectory, "vpskit-xray.service"), []byte(xraySystemdUnit()), 0o644); err != nil {
		return err
	}
	if hasCertificateLifecycle {
		if err := fsutil.WriteFileAtomic(filepath.Join(stagingDirectory, "vpskit-certificate-renew.service"), []byte(certificateRenewServiceUnit()), 0o644); err != nil {
			return err
		}
		if err := fsutil.WriteFileAtomic(filepath.Join(stagingDirectory, "vpskit-certificate-renew.timer"), []byte(certificateRenewTimerUnit()), 0o644); err != nil {
			return err
		}
	}
	ownership, err := readOwnershipDocument()
	if err != nil {
		return err
	}
	if hasCertificateLifecycle {
		appendOwnershipValue(ownership, "files", certificateRenewServiceUnitPath)
		appendOwnershipValue(ownership, "files", certificateRenewTimerUnitPath)
		appendOwnershipValue(ownership, "services", certificateRenewServiceUnitName)
		appendOwnershipValue(ownership, "services", certificateRenewTimerUnitName)
	}
	appendOwnershipValue(ownership, "files", versionsLockPath)
	appendOwnershipValue(ownership, "files", installedXray)
	appendOwnershipValue(ownership, "files", xrayServiceUnitPath)
	appendOwnershipValue(ownership, "files", xrayServerConfigPath)
	appendOwnershipValue(ownership, "services", xrayServiceUnitName)
	ownership["transaction_id"] = transactionID
	updatedState := state
	updatedState.SchemaVersion = model.SchemaVersion
	updatedState.TransactionID = transactionID
	updatedState.VPSKitVersion = manifest.ReleaseID
	updatedState.Core = model.CoreState{
		ID:           "sing-box",
		Version:      singBoxAsset.Version,
		Channel:      coreAssetChannel(singBoxAsset),
		Path:         installedSingBox,
		SHA256:       singBoxAsset.SHA256,
		SourceURL:    singBoxAsset.SourceURL,
		SourceRef:    singBoxAsset.SourceRef,
		SourceCommit: singBoxAsset.SourceCommit,
	}
	updatedState.RealityCore = model.CoreState{
		ID:           "xray",
		Version:      xrayAsset.Version,
		Channel:      coreAssetChannel(xrayAsset),
		Path:         installedXray,
		SHA256:       xrayAsset.SHA256,
		SourceURL:    xrayAsset.SourceURL,
		SourceRef:    xrayAsset.SourceRef,
		SourceCommit: xrayAsset.SourceCommit,
	}
	secrets, err := readInstalledSecrets()
	if err != nil {
		return err
	}
	secrets.SchemaVersion = model.SchemaVersion
	artifacts, clientSet, err := renderProfileArtifacts(updatedState, secrets)
	if err != nil {
		return err
	}
	updatedState.ConfigSHA256 = sha256Bytes(artifacts[serverConfigPath])
	updatedState.RealityConfigSHA256 = sha256Bytes(artifacts[xrayServerConfigPath])
	updatedState.Exports = exportStateForProfile(updatedState)
	updatedStateBytes, err := json.MarshalIndent(updatedState, "", "  ")
	if err != nil {
		return err
	}
	secretBytes, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return err
	}
	if err := stageProfileMutation(stagingDirectory, artifacts, updatedStateBytes, secretBytes); err != nil {
		return err
	}
	if output, err := runCommand(filepath.Join(stagingDirectory, "sing-box"), "check", "-c", filepath.Join(stagingDirectory, "sing-box.json")); err != nil {
		return fmt.Errorf("new sing-box configuration check failed: %w: %s", err, sanitizeText(output, ""))
	}
	if output, err := runCommand(filepath.Join(stagingDirectory, "xray"), "run", "-test", "-config", filepath.Join(stagingDirectory, "xray.json")); err != nil {
		return fmt.Errorf("new Xray configuration check failed: %w: %s", err, sanitizeText(output, ""))
	}
	ownershipBytes, err := json.MarshalIndent(ownership, "", "  ")
	if err != nil {
		return err
	}

	mutated := false
	restartAttempted := false
	committed := false
	defer func() {
		if committed {
			return
		}
		rollbackError := rollbackSelfUpdate(mutated, restartAttempted, timerWasActive, managedFiles, backupDirectory, transactionDirectory, state)
		status := "ROLLED_BACK"
		if rollbackError != nil {
			status = "ROLLBACK_FAILED"
			preserveTransactionArtifacts = true
		}
		record := map[string]any{
			"schema_version":          1,
			"transaction_id":          transactionID,
			"command":                 "update self",
			"status":                  status,
			"failed_at":               time.Now().UTC(),
			"previous_vpskit_version": state.VPSKitVersion,
			"new_vpskit_version":      manifest.ReleaseID,
			"previous_backup_id":      previousBackupID,
			"previous_state_schema":   previousDiskSchema,
			"new_state_schema":        model.SchemaVersion,
			"error":                   sanitizeError(returnErr, ""),
		}
		if rollbackError != nil {
			record["rollback_error"] = sanitizeError(rollbackError, "")
		}
		_ = writeTransactionRecord(transactionDirectory, record)
		if returnErr == nil && rollbackError != nil {
			returnErr = rollbackError
		}
	}()

	mutated = true
	activationFiles := []struct {
		name string
		path string
		mode os.FileMode
	}{
		{"vpskit", installedBinary, 0o755},
		{"sing-box", installedSingBox, 0o755},
		{"xray", installedXray, 0o755},
		{"lego", installedLego, 0o755},
		{"versions.lock", versionsLockPath, 0o644},
		{"vpskit-sing-box.service", serviceUnitPath, 0o644},
		{"vpskit-xray.service", xrayServiceUnitPath, 0o644},
	}
	if hasCertificateLifecycle {
		activationFiles = append(activationFiles,
			struct {
				name string
				path string
				mode os.FileMode
			}{"vpskit-certificate-renew.service", certificateRenewServiceUnitPath, 0o644},
			struct {
				name string
				path string
				mode os.FileMode
			}{"vpskit-certificate-renew.timer", certificateRenewTimerUnitPath, 0o644},
		)
	}
	for _, asset := range activationFiles {
		if err := fsutil.CopyFileAtomic(filepath.Join(stagingDirectory, asset.name), asset.path, asset.mode); err != nil {
			return err
		}
	}
	if err := activateProfileMutation(artifacts, clientSet, updatedStateBytes, secretBytes); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(ownershipPath, append(ownershipBytes, '\n'), 0o600); err != nil {
		return err
	}
	if output, err := runCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("systemd daemon-reload failed: %w: %s", err, output)
	}
	if hasCertificateLifecycle {
		if output, err := runCommand("systemctl", "enable", "--now", certificateRenewTimerUnitName); err != nil {
			return fmt.Errorf("certificate renewal timer enable failed: %w: %s", err, output)
		}
	}
	if output, err := runCommand(installedSingBox, "check", "-c", serverConfigPath); err != nil {
		return fmt.Errorf("installed sing-box configuration check failed: %w: %s", err, output)
	}
	if output, err := runCommand(installedXray, "run", "-test", "-config", xrayServerConfigPath); err != nil {
		return fmt.Errorf("installed Xray configuration check failed: %w: %s", err, output)
	}
	restartAttempted = true
	if err := restartManagedServices(updatedState); err != nil {
		return fmt.Errorf("service restart after update failed: %w", err)
	}
	if err := waitForManagedServiceState(updatedState, 30*time.Second); err != nil {
		return fmt.Errorf("service health check after update failed: %w", err)
	}
	if execErr := execCommandSuccess(installedBinary, "version"); execErr != nil {
		return fmt.Errorf("updated vpskit binary cannot execute: %w", execErr)
	}
	if hasCertificateLifecycle {
		if execErr := execCommandSuccess("systemctl", "is-active", "--quiet", certificateRenewTimerUnitName); execErr != nil {
			return fmt.Errorf("certificate renewal timer is not active: %w", execErr)
		}
		if _, err := readCertificateDetails(updatedState.Domain); err != nil {
			return fmt.Errorf("managed certificate validation after update failed: %w", err)
		}
	}
	record := map[string]any{
		"schema_version":                1,
		"transaction_id":                transactionID,
		"command":                       "update self",
		"status":                        "COMMITTED",
		"committed_at":                  time.Now().UTC(),
		"previous_vpskit_version":       state.VPSKitVersion,
		"new_vpskit_version":            manifest.ReleaseID,
		"previous_core_version":         state.Core.Version,
		"new_core_version":              singBoxAsset.Version,
		"previous_reality_core_version": state.RealityCore.Version,
		"new_reality_core_version":      xrayAsset.Version,
		"previous_backup_id":            previousBackupID,
		"previous_state_schema":         previousDiskSchema,
		"new_state_schema":              model.SchemaVersion,
		"config_sha256":                 updatedState.ConfigSHA256,
		"renewal_timer_active":          hasCertificateLifecycle,
		"exports_regenerated":           true,
	}
	if err := writeTransactionRecord(transactionDirectory, record); err != nil {
		return err
	}
	committed = true
	_ = fsutil.RemoveManagedTree(stagingDirectory, transactionDirectory)
	_ = fsutil.RemoveManagedTree(backupDirectory, transactionDirectory)
	_ = appendAudit(map[string]any{
		"time":           time.Now().UTC(),
		"transaction_id": transactionID,
		"command":        "update self",
		"status":         "COMMITTED",
	})
	return printJSON(commandResult{Command: "update self", Status: "PASS", Detail: map[string]any{
		"result":                 "UPDATED",
		"transaction_id":         transactionID,
		"previous_version":       state.VPSKitVersion,
		"new_version":            manifest.ReleaseID,
		"previous_backup_id":     previousBackupID,
		"renewal_timer_active":   hasCertificateLifecycle,
		"exports_regenerated":    true,
		"client_update_required": false,
	}})
}

func execCommandSuccess(name string, arguments ...string) error {
	if output, err := runCommand(name, arguments...); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(output))
	}
	return nil
}

func backupUpdateFile(source, destination string, mode os.FileMode) (bool, error) {
	info, err := os.Lstat(source)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect update path %s: %w", source, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, fmt.Errorf("refusing non-regular update path: %s", source)
	}
	if err := fsutil.CopyFileAtomic(source, destination, mode); err != nil {
		return false, fmt.Errorf("backup update path %s: %w", source, err)
	}
	return true, nil
}

func readOwnershipDocument() (map[string]any, error) {
	bytes, err := os.ReadFile(ownershipPath)
	if err != nil {
		return nil, fmt.Errorf("read ownership: %w", err)
	}
	var ownership map[string]any
	if err := json.Unmarshal(bytes, &ownership); err != nil {
		return nil, fmt.Errorf("parse ownership: %w", err)
	}
	if ownership == nil {
		return nil, errors.New("ownership document is empty")
	}
	return ownership, nil
}

func appendOwnershipValue(document map[string]any, key, value string) {
	raw, ok := document[key].([]any)
	if !ok {
		raw = []any{}
	}
	for _, item := range raw {
		if item == value {
			return
		}
	}
	document[key] = append(raw, value)
}

func rollbackSelfUpdate(mutated, restartAttempted, timerWasActive bool, managedFiles []updateManagedFile, backupDirectory, transactionDirectory string, previousState model.State) error {
	if !mutated {
		return nil
	}
	var rollbackErrors []string
	_, _ = runCommand("systemctl", "disable", "--now", certificateRenewTimerUnitName)
	for _, managed := range managedFiles {
		backup := filepath.Join(backupDirectory, managed.name)
		if managed.exists {
			if err := fsutil.CopyFileAtomic(backup, managed.path, managed.mode); err != nil {
				rollbackErrors = append(rollbackErrors, "restore "+managed.path+": "+err.Error())
			}
		} else if err := os.Remove(managed.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollbackErrors = append(rollbackErrors, "remove new "+managed.path+": "+err.Error())
		}
	}
	if output, err := runCommand("systemctl", "daemon-reload"); err != nil {
		rollbackErrors = append(rollbackErrors, "daemon-reload: "+output)
	}
	if timerWasActive {
		if output, err := runCommand("systemctl", "enable", "--now", certificateRenewTimerUnitName); err != nil {
			rollbackErrors = append(rollbackErrors, "restore renewal timer: "+output)
		}
	}
	if restartAttempted {
		if err := restartManagedServices(previousState); err != nil {
			rollbackErrors = append(rollbackErrors, "restart restored services: "+err.Error())
		} else if err := waitForManagedServiceState(previousState, 30*time.Second); err != nil {
			rollbackErrors = append(rollbackErrors, "health check restored service: "+err.Error())
		}
	}
	return joinUpdateErrors(rollbackErrors)
}

func joinUpdateErrors(values []string) error {
	if len(values) == 0 {
		return nil
	}
	return errors.New(strings.Join(values, "; "))
}
