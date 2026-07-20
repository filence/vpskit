package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/fsutil"
	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/platform"
	"vpskit.local/vpskit/internal/release"
	"vpskit.local/vpskit/internal/render"
)

func installRealityOnly(options InstallOptions) (returnErr error) {
	if err := validateRealityOnlyInstallOptions(options); err != nil {
		return err
	}
	if !platform.IsRoot() {
		return errors.New("install requires root privileges")
	}
	if options.PublicKeyBase64 == "" {
		return errors.New("this build has no embedded release public key")
	}
	if _, err := os.Stat(statePath); err == nil {
		return errors.New("VPSKit is already installed; refusing an initial-install overwrite")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect existing state: %w", err)
	}
	if _, err := os.Stat(serviceUnitPath); err == nil {
		return errors.New("an unmanaged vpskit-sing-box.service already exists")
	}
	if _, err := os.Stat(xrayServiceUnitPath); err == nil {
		return errors.New("an unmanaged vpskit-xray.service already exists")
	}
	manifest, err := verifyBundle(options.BundleDir, options.PublicKeyBase64)
	if err != nil {
		return fmt.Errorf("verify signed bundle: %w", err)
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
	if _, err := validateVersionsLock(options.BundleDir, versionsLockAsset, singBoxAsset, xrayAsset, legoAsset); err != nil {
		return err
	}
	preflight, err := collectProfilePreflight(true, options.TCPPort, false, 0, options.RealityTarget, filepath.Join(options.BundleDir, singBoxAsset.File))
	if err != nil {
		return fmt.Errorf("preflight failed: %w", err)
	}
	processLock, err := platform.AcquireProcessLock(filepath.Join(stateRoot, "locks", "vpskit.lock"))
	if err != nil {
		return err
	}
	defer processLock.Close()

	transactionID := "TX-" + time.Now().UTC().Format("20060102-150405") + "-install-reality-" + randomHex(3)
	transactionDirectory := filepath.Join(transactionRoot, transactionID)
	stagingDirectory := filepath.Join(transactionDirectory, "staging")
	if err := os.MkdirAll(stagingDirectory, 0o700); err != nil {
		return fmt.Errorf("create transaction staging: %w", err)
	}
	_ = os.Chmod(transactionDirectory, 0o700)
	_ = os.Chmod(stagingDirectory, 0o700)
	committed := false
	userCreated := false
	systemMutated := false
	defer func() {
		if committed {
			return
		}
		_ = fsutil.RemoveManagedTree(stagingDirectory, transactionDirectory)
		rollbackError := rollbackInitialInstall(userCreated, systemMutated)
		failure := map[string]any{
			"schema_version": 1,
			"transaction_id": transactionID,
			"command":        "install reality-only",
			"status":         "ROLLED_BACK",
			"failed_at":      time.Now().UTC(),
			"error":          sanitizeError(returnErr, ""),
		}
		if rollbackError != nil {
			failure["status"] = "ROLLBACK_FAILED"
			failure["rollback_error"] = rollbackError.Error()
		}
		_ = writeTransactionRecord(transactionDirectory, failure)
		if returnErr == nil && rollbackError != nil {
			returnErr = rollbackError
		}
	}()

	values, err := generateRuntimeValues(options, "", "", filepath.Join(options.BundleDir, singBoxAsset.File))
	if err != nil {
		return err
	}
	values.Hysteria2Enabled = false
	values.Hysteria2Password = ""
	values.Domain = ""
	serverConfig, err := render.ServerConfig(values)
	if err != nil {
		return err
	}
	stagingConfig := filepath.Join(stagingDirectory, "sing-box.json")
	if err := fsutil.WriteFileAtomic(stagingConfig, serverConfig, 0o600); err != nil {
		return err
	}
	if output, err := runCommand(filepath.Join(options.BundleDir, singBoxAsset.File), "check", "-c", stagingConfig); err != nil {
		return fmt.Errorf("sing-box staging check failed: %w: %s", err, sanitizeText(output, ""))
	}
	xrayConfig, err := render.XrayRealityServerConfig(values)
	if err != nil {
		return err
	}
	stagingXrayConfig := filepath.Join(stagingDirectory, "xray.json")
	if err := fsutil.WriteFileAtomic(stagingXrayConfig, xrayConfig, 0o600); err != nil {
		return err
	}
	if output, err := runCommand(filepath.Join(options.BundleDir, xrayAsset.File), "run", "-test", "-config", stagingXrayConfig); err != nil {
		return fmt.Errorf("xray staging check failed: %w: %s", err, sanitizeText(output, ""))
	}

	userCreated, err = ensureServiceUser()
	if err != nil {
		return err
	}
	systemMutated = true
	if err := installRealityOnlyManagedFiles(options, vpskitAsset, singBoxAsset, xrayAsset, legoAsset, versionsLockAsset, values, transactionID); err != nil {
		return err
	}
	if output, err := runCommand(installedSingBox, "check", "-c", serverConfigPath); err != nil {
		return fmt.Errorf("installed sing-box check failed: %w: %s", err, output)
	}
	if output, err := runCommand(installedXray, "run", "-test", "-config", xrayServerConfigPath); err != nil {
		return fmt.Errorf("installed Xray check failed: %w: %s", err, output)
	}
	if err := startServiceProfile(true, options.TCPPort, false, 0, false); err != nil {
		return err
	}
	if err := fsutil.RemoveManagedTree(stagingDirectory, transactionDirectory); err != nil {
		return fmt.Errorf("remove sensitive transaction staging: %w", err)
	}
	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		return fmt.Errorf("read committed state: %w", err)
	}
	commitRecord := map[string]any{
		"schema_version": 1,
		"transaction_id": transactionID,
		"command":        "install reality-only",
		"status":         "COMMITTED",
		"committed_at":   time.Now().UTC(),
		"state_sha256":   sha256Bytes(stateBytes),
		"preflight":      preflight,
	}
	if err := writeTransactionRecord(transactionDirectory, commitRecord); err != nil {
		return err
	}
	committed = true
	_ = appendAudit(map[string]any{
		"time":           time.Now().UTC(),
		"transaction_id": transactionID,
		"command":        "install reality-only",
		"status":         "COMMITTED",
	})
	return printJSON(commandResult{Command: "install reality-only", Status: "PASS", Detail: map[string]any{
		"transaction_id": transactionID,
		"core_versions":  map[string]string{"sing-box": singBoxAsset.Version, "xray": xrayAsset.Version},
		"tcp_listener":   fmt.Sprintf("tcp/%d", options.TCPPort),
		"udp_listener":   "disabled",
		"firewall":       "manual/noop",
		"exports":        exportRoot,
	}})
}

func validateRealityOnlyInstallOptions(options InstallOptions) error {
	if options.BundleDir == "" {
		return errors.New("--bundle-dir is required")
	}
	if net.ParseIP(options.ConnectHost) == nil && !validDomain(strings.ToLower(options.ConnectHost)) {
		return fmt.Errorf("invalid connect host: %q", options.ConnectHost)
	}
	if net.ParseIP(options.RealityTarget) != nil || !validDomain(options.RealityTarget) {
		return fmt.Errorf("invalid Reality server name: %q", options.RealityTarget)
	}
	if options.TCPPort < 1 || options.TCPPort > 65535 {
		return fmt.Errorf("invalid TCP port: %d", options.TCPPort)
	}
	return nil
}

func installRealityOnlyManagedFiles(options InstallOptions, vpskitAsset, singBoxAsset, xrayAsset, legoAsset, versionsLockAsset release.Asset, values model.RuntimeValues, transactionID string) error {
	directories := []struct {
		path string
		mode os.FileMode
	}{
		{configRoot, 0o750}, {generatedRoot, 0o750}, {exportRoot, 0o700},
		{stateRoot, 0o750}, {secretRoot, 0o700}, {logRoot, 0o750},
		{filepath.Dir(installedSingBox), 0o755},
	}
	for _, directory := range directories {
		if err := os.MkdirAll(directory.path, directory.mode); err != nil {
			return fmt.Errorf("create managed directory %s: %w", directory.path, err)
		}
		if err := os.Chmod(directory.path, directory.mode); err != nil {
			return err
		}
	}
	for _, asset := range []struct {
		source string
		target string
		mode   os.FileMode
	}{
		{filepath.Join(options.BundleDir, vpskitAsset.File), installedBinary, 0o755},
		{filepath.Join(options.BundleDir, singBoxAsset.File), installedSingBox, 0o755},
		{filepath.Join(options.BundleDir, xrayAsset.File), installedXray, 0o755},
		{filepath.Join(options.BundleDir, legoAsset.File), installedLego, 0o755},
		{filepath.Join(options.BundleDir, versionsLockAsset.File), versionsLockPath, 0o644},
	} {
		if err := fsutil.CopyFileAtomic(asset.source, asset.target, asset.mode); err != nil {
			return err
		}
	}
	serverConfig, err := render.ServerConfig(values)
	if err != nil {
		return err
	}
	realityClient, err := render.SingBoxRealityClient(values, 2080)
	if err != nil {
		return err
	}
	exportFiles := map[string][]byte{
		filepath.Join(exportRoot, "sing-box-reality.json"): realityClient,
		filepath.Join(exportRoot, "mihomo.yaml"):           render.Mihomo(values),
		filepath.Join(exportRoot, "share-links.txt"):       render.ShareLinks(values),
	}
	if err := fsutil.WriteFileAtomic(serverConfigPath, serverConfig, 0o640); err != nil {
		return err
	}
	xrayConfig, err := render.XrayRealityServerConfig(values)
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(xrayServerConfigPath, xrayConfig, 0o640); err != nil {
		return err
	}
	for path, content := range exportFiles {
		if err := fsutil.WriteFileAtomic(path, content, 0o600); err != nil {
			return err
		}
	}
	secrets := model.Secrets{
		SchemaVersion:     model.SchemaVersion,
		RealityUUID:       values.RealityUUID,
		RealityPrivateKey: values.RealityPrivateKey,
	}
	secretBytes, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(secretPath, append(secretBytes, '\n'), 0o600); err != nil {
		return err
	}
	state := model.State{
		SchemaVersion:     model.SchemaVersion,
		VPSKitVersion:     options.VPSKitVersion,
		TransactionID:     transactionID,
		InstalledAt:       time.Now().UTC(),
		Profile:           "reality-only",
		ConnectHost:       values.ConnectHost,
		RealityServerName: values.RealityServerName,
		Core: model.CoreState{
			ID:           "sing-box",
			Version:      singBoxAsset.Version,
			Channel:      coreAssetChannel(singBoxAsset),
			Path:         installedSingBox,
			SHA256:       singBoxAsset.SHA256,
			SourceURL:    singBoxAsset.SourceURL,
			SourceRef:    singBoxAsset.SourceRef,
			SourceCommit: singBoxAsset.SourceCommit,
		},
		RealityCore: model.CoreState{
			ID: "xray", Version: xrayAsset.Version, Channel: coreAssetChannel(xrayAsset), Path: installedXray,
			SHA256: xrayAsset.SHA256, SourceURL: xrayAsset.SourceURL, SourceRef: xrayAsset.SourceRef, SourceCommit: xrayAsset.SourceCommit,
		},
		Reality: model.RealityState{
			Enabled: true, ID: "reality-main", ListenPort: values.TCPPort,
			UUIDRef: "secret://reality-main/uuid", PrivateKeyRef: "secret://reality-main/private-key",
			PublicKey: values.RealityPublicKey, ShortID: values.RealityShortID,
		},
		Firewall:            model.FirewallState{Provider: "manual/noop", Status: "no active local firewall detected during install"},
		ConfigSHA256:        sha256Bytes(serverConfig),
		RealityConfigSHA256: sha256Bytes(xrayConfig),
		ConfigRevision:      1,
	}
	state.Exports = exportStateForProfile(state)
	stateBytes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(statePath, append(stateBytes, '\n'), 0o600); err != nil {
		return err
	}
	ownership := map[string]any{
		"schema_version": 1,
		"transaction_id": transactionID,
		"files":          []string{installedBinary, installedSingBox, installedXray, installedLego, versionsLockPath, serviceUnitPath, xrayServiceUnitPath, serverConfigPath, xrayServerConfigPath, statePath, ownershipPath, secretPath},
		"directories":    []string{configRoot, exportRoot, stateRoot, secretRoot, logRoot, "/usr/local/lib/vpskit"},
		"services":       []string{serviceUnitName, xrayServiceUnitName},
		"firewall_rules": []string{},
	}
	ownershipBytes, err := json.MarshalIndent(ownership, "", "  ")
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(ownershipPath, append(ownershipBytes, '\n'), 0o600); err != nil {
		return err
	}
	if err := setServiceFileOwnership(); err != nil {
		return err
	}
	if err := verifyServiceReadAccess(); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(serviceUnitPath, []byte(systemdUnit()), 0o644); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(xrayServiceUnitPath, []byte(xraySystemdUnit()), 0o644)
}
