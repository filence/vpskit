package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/fsutil"
	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/platform"
	"vpskit.local/vpskit/internal/release"
)

var (
	coreVersionPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?$`)
	coreCommitPattern  = regexp.MustCompile(`^[0-9a-f]{40}$`)
	versionLinePattern = regexp.MustCompile(`(?m)^sing-box version ([^\s]+)\s*$`)
)

type parsedCoreVersion struct {
	major      int
	minor      int
	patch      int
	prerelease string
}

func updateCore(bundleDir, requestedChannel, publicKeyBase64 string) (returnErr error) {
	if !platform.IsRoot() {
		return errors.New("core update requires root privileges")
	}
	if publicKeyBase64 == "" {
		return errors.New("this build has no embedded release public key")
	}
	if !validCoreChannel(requestedChannel) {
		return fmt.Errorf("unsupported core channel %q; use stable, candidate, beta, or pinned", requestedChannel)
	}

	lock, err := platform.AcquireProcessLock(filepath.Join(stateRoot, "locks", "vpskit.lock"))
	if err != nil {
		return err
	}
	defer lock.Close()

	previousDiskSchema, err := installedStateSchemaVersion()
	if err != nil {
		return err
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	if err := healthCheckState(state); err != nil {
		return fmt.Errorf("refusing core update while active service is unhealthy: %w", err)
	}
	installedDigest, err := release.FileSHA256(installedSingBox)
	if err != nil {
		return fmt.Errorf("hash installed sing-box: %w", err)
	}
	if !strings.EqualFold(installedDigest, state.Core.SHA256) {
		return errors.New("installed sing-box digest does not match managed state; refusing update")
	}

	manifest, err := verifyBundle(bundleDir, publicKeyBase64)
	if err != nil {
		return fmt.Errorf("verify signed core bundle: %w", err)
	}
	asset, err := release.FindAsset(manifest, "sing-box")
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
	if _, err := validateVersionsLock(bundleDir, versionsLockAsset, asset, xrayAsset, legoAsset); err != nil {
		return err
	}
	if err := validateCoreAsset(asset, requestedChannel, state.Core.Version); err != nil {
		return err
	}
	if state.Core.Version == asset.Version && !strings.EqualFold(state.Core.SHA256, asset.SHA256) {
		return errors.New("signed core bundle reuses the installed version with a different digest")
	}

	transactionID := "TX-" + time.Now().UTC().Format("20060102-150405") + "-update-core-" + randomHex(3)
	transactionDirectory := filepath.Join(transactionRoot, transactionID)
	stagingDirectory := filepath.Join(transactionDirectory, "staging")
	backupDirectory := filepath.Join(transactionDirectory, "backup")
	for _, directory := range []string{stagingDirectory, backupDirectory} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return fmt.Errorf("create core update directory: %w", err)
		}
		_ = os.Chmod(directory, 0o700)
	}
	_ = os.Chmod(transactionDirectory, 0o700)
	preserveTransactionArtifacts := false
	defer func() {
		if preserveTransactionArtifacts {
			return
		}
		_ = fsutil.RemoveManagedTree(stagingDirectory, transactionDirectory)
		_ = fsutil.RemoveManagedTree(backupDirectory, transactionDirectory)
	}()

	stagedCore := filepath.Join(stagingDirectory, "sing-box")
	if err := fsutil.CopyFileAtomic(filepath.Join(bundleDir, asset.File), stagedCore, 0o755); err != nil {
		return err
	}
	if err := verifyCoreBinary(stagedCore, asset.Version); err != nil {
		return err
	}
	if output, err := runCommand(stagedCore, "check", "-c", serverConfigPath); err != nil {
		return fmt.Errorf("new sing-box configuration check failed: %w: %s", err, sanitizeText(output, ""))
	}
	stagedVersionsLock := filepath.Join(stagingDirectory, "versions.lock")
	if err := fsutil.CopyFileAtomic(filepath.Join(bundleDir, versionsLockAsset.File), stagedVersionsLock, 0o644); err != nil {
		return err
	}

	updatedState := state
	updatedState.SchemaVersion = model.SchemaVersion
	updatedState.TransactionID = transactionID
	updatedState.Core = coreStateFromAsset(asset, requestedChannel)
	updatedState.SynchronizeLegacyInstances()
	stateBytes, err := json.MarshalIndent(updatedState, "", "  ")
	if err != nil {
		return err
	}
	if _, err := decodeInstalledState(stateBytes); err != nil {
		return fmt.Errorf("validate migrated state: %w", err)
	}
	ownership, err := readOwnershipDocument()
	if err != nil {
		return err
	}
	versionsLockOwned := ownershipHasValue(ownership, "files", versionsLockPath)
	appendOwnershipValue(ownership, "files", versionsLockPath)
	ownership["transaction_id"] = transactionID
	ownershipBytes, err := json.MarshalIndent(ownership, "", "  ")
	if err != nil {
		return err
	}
	versionsLockCurrent := managedFileDigestMatches(versionsLockPath, versionsLockAsset.SHA256)
	if coreStateEqual(state.Core, updatedState.Core) && state.SchemaVersion == model.SchemaVersion && versionsLockCurrent && versionsLockOwned {
		return printJSON(commandResult{Command: "update core", Status: "PASS", Detail: map[string]any{
			"result":  "ALREADY_CURRENT",
			"version": state.Core.Version,
			"channel": state.Core.Channel,
		}})
	}

	previousBackupID, err := createPersistentBackupLocked(state, "pre-core-update")
	if err != nil {
		return fmt.Errorf("create pre-core-update backup: %w", err)
	}
	lockExisted := false
	for _, managed := range []updateManagedFile{
		{path: installedSingBox, name: "sing-box", mode: 0o755},
		{path: statePath, name: "state.json", mode: 0o600},
		{path: ownershipPath, name: "ownership.json", mode: 0o600},
		{path: versionsLockPath, name: "versions.lock", mode: 0o644},
	} {
		existed, err := backupUpdateFile(managed.path, filepath.Join(backupDirectory, managed.name), managed.mode)
		if err != nil {
			return err
		}
		if managed.path == versionsLockPath {
			lockExisted = existed
		}
	}

	coreReplaced := !strings.EqualFold(state.Core.SHA256, asset.SHA256)
	progressRecord := map[string]any{
		"schema_version":        1,
		"transaction_id":        transactionID,
		"command":               "update core",
		"status":                "IN_PROGRESS",
		"phase":                 "VALIDATED",
		"started_at":            time.Now().UTC(),
		"previous_core_version": state.Core.Version,
		"new_core_version":      asset.Version,
		"previous_core_sha256":  state.Core.SHA256,
		"new_core_sha256":       asset.SHA256,
		"previous_backup_id":    previousBackupID,
		"previous_state_schema": previousDiskSchema,
		"new_state_schema":      model.SchemaVersion,
		"versions_lock_sha256":  versionsLockAsset.SHA256,
	}
	if err := writeTransactionRecord(transactionDirectory, progressRecord); err != nil {
		return err
	}
	mutated := false
	restartAttempted := false
	committed := false
	defer func() {
		if committed {
			return
		}
		rollbackError := rollbackCoreUpdate(mutated, coreReplaced, restartAttempted, lockExisted, backupDirectory, state)
		status := "ABORTED"
		if mutated {
			status = "ROLLED_BACK"
		}
		if rollbackError != nil {
			status = "ROLLBACK_FAILED"
			preserveTransactionArtifacts = true
		}
		record := map[string]any{
			"schema_version":        1,
			"transaction_id":        transactionID,
			"command":               "update core",
			"status":                status,
			"failed_at":             time.Now().UTC(),
			"previous_core_version": state.Core.Version,
			"new_core_version":      asset.Version,
			"previous_core_sha256":  state.Core.SHA256,
			"new_core_sha256":       asset.SHA256,
			"previous_backup_id":    previousBackupID,
			"previous_state_schema": previousDiskSchema,
			"new_state_schema":      model.SchemaVersion,
			"error":                 sanitizeError(returnErr, ""),
		}
		if rollbackError != nil {
			record["rollback_error"] = sanitizeError(rollbackError, "")
		}
		_ = writeTransactionRecord(transactionDirectory, record)
		if returnErr == nil && rollbackError != nil {
			returnErr = rollbackError
		}
	}()

	progressRecord["phase"] = "APPLYING"
	if err := writeTransactionRecord(transactionDirectory, progressRecord); err != nil {
		return err
	}
	mutated = true
	if coreReplaced {
		if err := fsutil.CopyFileAtomic(stagedCore, installedSingBox, 0o755); err != nil {
			return err
		}
	}
	if err := fsutil.CopyFileAtomic(stagedVersionsLock, versionsLockPath, 0o644); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(statePath, append(stateBytes, '\n'), 0o600); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(ownershipPath, append(ownershipBytes, '\n'), 0o600); err != nil {
		return err
	}
	if coreReplaced {
		if output, err := runCommand(installedSingBox, "check", "-c", serverConfigPath); err != nil {
			return fmt.Errorf("installed sing-box configuration check failed: %w: %s", err, sanitizeText(output, ""))
		}
		restartAttempted = updatedState.Hysteria2.Enabled
		if updatedState.Hysteria2.Enabled {
			if output, err := runCommand("systemctl", "restart", serviceUnitName); err != nil {
				return fmt.Errorf("service restart after core update failed: %w: %s", err, sanitizeText(output, ""))
			}
		}
		if err := waitForManagedServiceState(updatedState, 30*time.Second); err != nil {
			return fmt.Errorf("service health check after core update failed: %w", err)
		}
	}
	if err := verifyCoreBinary(installedSingBox, asset.Version); err != nil {
		return fmt.Errorf("verify installed core: %w", err)
	}
	installedDigestAfter, err := release.FileSHA256(installedSingBox)
	if err != nil {
		return fmt.Errorf("hash installed core after update: %w", err)
	}
	if !strings.EqualFold(installedDigestAfter, asset.SHA256) {
		return fmt.Errorf("installed core digest does not match signed asset: actual=%s expected=%s", installedDigestAfter, asset.SHA256)
	}
	configBytes, err := os.ReadFile(serverConfigPath)
	if err != nil {
		return fmt.Errorf("read active configuration after core update: %w", err)
	}
	if actualConfigHash := sha256Hex(configBytes); actualConfigHash != updatedState.ConfigSHA256 {
		return fmt.Errorf("active configuration changed during core update: actual=%s expected=%s", actualConfigHash, updatedState.ConfigSHA256)
	}
	if !managedFileDigestMatches(versionsLockPath, versionsLockAsset.SHA256) {
		return errors.New("installed versions lock does not match signed asset")
	}

	result := "UPDATED"
	if !coreReplaced {
		result = "METADATA_RECONCILED"
	}
	record := map[string]any{
		"schema_version":        1,
		"transaction_id":        transactionID,
		"command":               "update core",
		"status":                "COMMITTED",
		"committed_at":          time.Now().UTC(),
		"result":                result,
		"channel":               requestedChannel,
		"previous_core_version": state.Core.Version,
		"new_core_version":      asset.Version,
		"previous_core_sha256":  state.Core.SHA256,
		"new_core_sha256":       asset.SHA256,
		"source_ref":            asset.SourceRef,
		"source_commit":         asset.SourceCommit,
		"previous_backup_id":    previousBackupID,
		"previous_state_schema": previousDiskSchema,
		"new_state_schema":      model.SchemaVersion,
		"versions_lock_sha256":  versionsLockAsset.SHA256,
	}
	if err := writeTransactionRecord(transactionDirectory, record); err != nil {
		return err
	}
	committed = true
	_ = appendAudit(map[string]any{
		"time":           time.Now().UTC(),
		"transaction_id": transactionID,
		"command":        "update core",
		"status":         "COMMITTED",
		"result":         result,
	})
	return printJSON(commandResult{Command: "update core", Status: "PASS", Detail: map[string]any{
		"result":                 result,
		"transaction_id":         transactionID,
		"previous_version":       state.Core.Version,
		"new_version":            asset.Version,
		"channel":                requestedChannel,
		"previous_backup_id":     previousBackupID,
		"service_restart_needed": coreReplaced,
	}})
}

func validateCoreAsset(asset release.Asset, requestedChannel, currentVersion string) error {
	version, err := parseCoreVersion(asset.Version)
	if err != nil {
		return fmt.Errorf("invalid signed core version: %w", err)
	}
	if asset.Channel == "" || !validCoreChannel(asset.Channel) {
		return fmt.Errorf("signed core asset has invalid channel %q", asset.Channel)
	}
	if requestedChannel != "pinned" && asset.Channel != requestedChannel {
		return fmt.Errorf("signed core channel %q is not allowed by requested channel %q", asset.Channel, requestedChannel)
	}
	if requestedChannel == "stable" && version.prerelease != "" {
		return errors.New("stable channel refuses a prerelease core")
	}
	if asset.SourceRepo != "SagerNet/sing-box" {
		return fmt.Errorf("unexpected core source repository %q", asset.SourceRepo)
	}
	wantedRef := "v" + strings.TrimPrefix(asset.Version, "v")
	if asset.SourceRef != wantedRef {
		return fmt.Errorf("core source_ref %q does not match version %q", asset.SourceRef, asset.Version)
	}
	if !coreCommitPattern.MatchString(asset.SourceCommit) {
		return errors.New("core source_commit must be a lowercase 40-character Git commit")
	}
	wantedURL := "https://github.com/SagerNet/sing-box/releases/tag/" + wantedRef
	if asset.SourceURL != wantedURL {
		return fmt.Errorf("core source_url %q does not match source_ref %q", asset.SourceURL, asset.SourceRef)
	}
	if asset.StateSchemaMin < 1 || asset.StateSchemaMin > model.SchemaVersion {
		return fmt.Errorf("core requires unsupported state schema %d", asset.StateSchemaMin)
	}
	current, err := parseCoreVersion(currentVersion)
	if err != nil {
		return fmt.Errorf("invalid installed core version: %w", err)
	}
	if compareCoreVersion(version, current) < 0 {
		return fmt.Errorf("refusing core downgrade from %s to %s", currentVersion, asset.Version)
	}
	return nil
}

func verifyCoreBinary(path, expectedVersion string) error {
	output, err := runCommand(path, "version")
	if err != nil {
		return fmt.Errorf("execute core version: %w: %s", err, sanitizeText(output, ""))
	}
	match := versionLinePattern.FindStringSubmatch(output)
	if len(match) != 2 {
		return errors.New("core version output is not recognized")
	}
	if strings.TrimPrefix(match[1], "v") != strings.TrimPrefix(expectedVersion, "v") {
		return fmt.Errorf("core binary reports version %q, manifest declares %q", match[1], expectedVersion)
	}
	return nil
}

func parseCoreVersion(value string) (parsedCoreVersion, error) {
	match := coreVersionPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(match) != 5 {
		return parsedCoreVersion{}, fmt.Errorf("unsupported semantic version %q", value)
	}
	parts := make([]int, 3)
	for index := range parts {
		parsed, err := strconv.Atoi(match[index+1])
		if err != nil {
			return parsedCoreVersion{}, err
		}
		parts[index] = parsed
	}
	return parsedCoreVersion{major: parts[0], minor: parts[1], patch: parts[2], prerelease: match[4]}, nil
}

func compareCoreVersion(left, right parsedCoreVersion) int {
	for _, pair := range [][2]int{{left.major, right.major}, {left.minor, right.minor}, {left.patch, right.patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	if left.prerelease == right.prerelease {
		return 0
	}
	if left.prerelease == "" {
		return 1
	}
	if right.prerelease == "" {
		return -1
	}
	return comparePrerelease(left.prerelease, right.prerelease)
}

func comparePrerelease(left, right string) int {
	leftParts := strings.Split(left, ".")
	rightParts := strings.Split(right, ".")
	limit := len(leftParts)
	if len(rightParts) < limit {
		limit = len(rightParts)
	}
	for index := 0; index < limit; index++ {
		leftNumber, leftErr := strconv.ParseUint(leftParts[index], 10, 64)
		rightNumber, rightErr := strconv.ParseUint(rightParts[index], 10, 64)
		switch {
		case leftErr == nil && rightErr == nil:
			if leftNumber < rightNumber {
				return -1
			}
			if leftNumber > rightNumber {
				return 1
			}
		case leftErr == nil:
			return -1
		case rightErr == nil:
			return 1
		default:
			if comparison := strings.Compare(leftParts[index], rightParts[index]); comparison != 0 {
				return comparison
			}
		}
	}
	if len(leftParts) < len(rightParts) {
		return -1
	}
	if len(leftParts) > len(rightParts) {
		return 1
	}
	return 0
}

func validCoreChannel(channel string) bool {
	switch channel {
	case "stable", "candidate", "beta", "pinned":
		return true
	default:
		return false
	}
}

func coreAssetChannel(asset release.Asset) string {
	if validCoreChannel(asset.Channel) {
		return asset.Channel
	}
	return "pinned"
}

func coreStateFromAsset(asset release.Asset, channel string) model.CoreState {
	return model.CoreState{
		ID:           "sing-box",
		Version:      asset.Version,
		Channel:      channel,
		Path:         installedSingBox,
		SHA256:       strings.ToLower(asset.SHA256),
		SourceURL:    asset.SourceURL,
		SourceRef:    asset.SourceRef,
		SourceCommit: asset.SourceCommit,
	}
}

func coreStateEqual(left, right model.CoreState) bool {
	return left.ID == right.ID &&
		left.Version == right.Version &&
		left.Channel == right.Channel &&
		left.Path == right.Path &&
		strings.EqualFold(left.SHA256, right.SHA256) &&
		left.SourceURL == right.SourceURL &&
		left.SourceRef == right.SourceRef &&
		left.SourceCommit == right.SourceCommit
}

func rollbackCoreUpdate(mutated, coreReplaced, restartAttempted, lockExisted bool, backupDirectory string, previousState model.State) error {
	if !mutated {
		return nil
	}
	var rollbackErrors []string
	if coreReplaced {
		if err := fsutil.CopyFileAtomic(filepath.Join(backupDirectory, "sing-box"), installedSingBox, 0o755); err != nil {
			rollbackErrors = append(rollbackErrors, "restore sing-box: "+err.Error())
		}
	}
	if err := fsutil.CopyFileAtomic(filepath.Join(backupDirectory, "state.json"), statePath, 0o600); err != nil {
		rollbackErrors = append(rollbackErrors, "restore state: "+err.Error())
	}
	if err := fsutil.CopyFileAtomic(filepath.Join(backupDirectory, "ownership.json"), ownershipPath, 0o600); err != nil {
		rollbackErrors = append(rollbackErrors, "restore ownership: "+err.Error())
	}
	if lockExisted {
		if err := fsutil.CopyFileAtomic(filepath.Join(backupDirectory, "versions.lock"), versionsLockPath, 0o644); err != nil {
			rollbackErrors = append(rollbackErrors, "restore versions lock: "+err.Error())
		}
	} else if err := os.Remove(versionsLockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		rollbackErrors = append(rollbackErrors, "remove new versions lock: "+err.Error())
	}
	if (coreReplaced || restartAttempted) && previousState.Hysteria2.Enabled {
		if output, err := runCommand("systemctl", "restart", serviceUnitName); err != nil {
			rollbackErrors = append(rollbackErrors, "restart restored Hysteria2 service: "+sanitizeText(output, ""))
		} else if err := waitForManagedServiceState(previousState, 30*time.Second); err != nil {
			rollbackErrors = append(rollbackErrors, "health check restored service: "+err.Error())
		}
	}
	return joinUpdateErrors(rollbackErrors)
}

func managedFileDigestMatches(path, expected string) bool {
	digest, err := release.FileSHA256(path)
	return err == nil && strings.EqualFold(digest, expected)
}

func ownershipHasValue(document map[string]any, key, value string) bool {
	values, ok := document[key].([]any)
	if !ok {
		return false
	}
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
