package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/fsutil"
	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/platform"
	"vpskit.local/vpskit/internal/render"
)

var allProfileExportPaths = []string{
	filepath.Join(exportRoot, "mihomo.yaml"),
	filepath.Join(exportRoot, "sing-box-reality.json"),
	filepath.Join(exportRoot, "sing-box-hysteria2.json"),
	filepath.Join(exportRoot, "share-links.txt"),
}

func runInstance(arguments []string) error {
	if len(arguments) < 2 {
		return errors.New("usage: vpskit instance <enable|disable|modify|delete> <reality|hysteria2> [--port <port>] [--reality-server-name <domain>] [--yes]")
	}
	operation := strings.ToLower(strings.TrimSpace(arguments[0]))
	target := strings.ToLower(strings.TrimSpace(arguments[1]))
	if target != "reality" && target != "hysteria2" {
		return fmt.Errorf("unsupported instance %q; use reality or hysteria2", target)
	}
	flags := flag.NewFlagSet("instance "+operation, flag.ContinueOnError)
	port := flags.Int("port", 0, "new listen port for instance modify")
	realityServerName := flags.String("reality-server-name", "", "new verified REALITY target domain for instance modify reality")
	yes := flags.Bool("yes", false, "confirm irreversible instance deletion")
	if err := flags.Parse(arguments[2:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	switch operation {
	case "enable", "disable":
		if *port != 0 || strings.TrimSpace(*realityServerName) != "" || *yes {
			return fmt.Errorf("instance %s does not accept modification options or --yes", operation)
		}
	case "modify":
		if *yes {
			return errors.New("instance modify does not accept --yes")
		}
		if *port == 0 && strings.TrimSpace(*realityServerName) == "" {
			return errors.New("instance modify requires --port and/or --reality-server-name")
		}
		if target != "reality" && strings.TrimSpace(*realityServerName) != "" {
			return errors.New("--reality-server-name is only valid for the reality instance")
		}
	case "delete":
		if *port != 0 || strings.TrimSpace(*realityServerName) != "" || !*yes {
			return errors.New("instance delete requires explicit --yes confirmation")
		}
	default:
		return fmt.Errorf("unsupported instance operation %q", operation)
	}
	if !platform.IsRoot() {
		return errors.New("instance mutation requires root privileges")
	}
	return mutateInstalledInstance(operation, target, *port, strings.ToLower(strings.TrimSpace(*realityServerName)))
}

func mutateInstalledInstance(operation, target string, port int, realityServerName string) (returnErr error) {
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	secrets, err := readInstalledSecrets()
	if err != nil {
		return err
	}
	updatedState := state
	updatedSecrets := secrets
	if err := applyInstanceStateChange(&updatedState, &updatedSecrets, operation, target, port); err != nil {
		return err
	}
	var realityTargetCheck *RealityScanResult
	if realityServerName != "" {
		if target != "reality" || operation != "modify" {
			return errors.New("REALITY target changes require instance modify reality")
		}
		if realityServerName == state.RealityServerName {
			return errors.New("REALITY server name is unchanged")
		}
		check, err := preflightRealityTarget(realityServerName, installedSingBox)
		if err != nil {
			return fmt.Errorf("new REALITY target verification failed: %w", err)
		}
		realityTargetCheck = check
		updatedState.RealityServerName = realityServerName
	}
	if operation == "modify" {
		if port != 0 {
			if err := verifyNewInstancePortAvailable(target, port); err != nil {
				return err
			}
		}
	}
	changedClientFields := clientFacingChanges(state, updatedState)
	if len(changedClientFields) == 0 {
		return errors.New("instance mutation produced no client-facing configuration change")
	}
	if state.ConfigRevision < 1 {
		state.ConfigRevision = 1
	}
	updatedState.ConfigRevision = state.ConfigRevision + 1
	if updatedState.ConfigRevision < 2 {
		updatedState.ConfigRevision = 2
	}

	lock, err := platform.AcquireProcessLock(filepath.Join(stateRoot, "locks", "vpskit.lock"))
	if err != nil {
		return err
	}
	defer lock.Close()

	backupID, err := createPersistentBackupLocked(state, "pre-instance")
	if err != nil {
		return fmt.Errorf("create instance rollback backup: %w", err)
	}
	transactionID := "TX-" + time.Now().UTC().Format("20060102-150405") + "-instance-" + randomHex(3)
	transactionDirectory := filepath.Join(transactionRoot, transactionID)
	stagingDirectory := filepath.Join(transactionDirectory, "staging")
	snapshotDirectory := filepath.Join(transactionDirectory, "backup")
	if err := os.MkdirAll(stagingDirectory, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(snapshotDirectory, 0o700); err != nil {
		return err
	}
	snapshotEntries, err := captureManagedEntries(snapshotDirectory)
	if err != nil {
		return err
	}

	updatedState.SchemaVersion = model.SchemaVersion
	updatedState.TransactionID = transactionID
	updatedState.Profile = profileFromInboundState(updatedState.Reality.Enabled, updatedState.Hysteria2.Enabled)
	artifacts, err := renderProfileArtifacts(updatedState, updatedSecrets)
	if err != nil {
		return err
	}
	updatedState.ConfigSHA256 = sha256Bytes(artifacts[serverConfigPath])
	updatedState.RealityConfigSHA256 = sha256Bytes(artifacts[xrayServerConfigPath])
	updatedState.Exports = exportStateForProfile(updatedState)
	stateBytes, err := json.MarshalIndent(updatedState, "", "  ")
	if err != nil {
		return err
	}
	secretBytes, err := json.MarshalIndent(updatedSecrets, "", "  ")
	if err != nil {
		return err
	}
	if err := stageProfileMutation(stagingDirectory, artifacts, stateBytes, secretBytes); err != nil {
		return err
	}
	stagedConfig := filepath.Join(stagingDirectory, "sing-box.json")
	if output, err := runCommand(installedSingBox, "check", "-c", stagedConfig); err != nil {
		return fmt.Errorf("mutated sing-box configuration check failed: %w: %s", err, sanitizeText(output, ""))
	}
	stagedXrayConfig := filepath.Join(stagingDirectory, "xray.json")
	if output, err := runCommand(installedXray, "run", "-test", "-config", stagedXrayConfig); err != nil {
		return fmt.Errorf("mutated Xray configuration check failed: %w: %s", err, sanitizeText(output, ""))
	}

	command := "instance " + operation
	progressRecord := map[string]any{
		"schema_version":           1,
		"transaction_id":           transactionID,
		"command":                  command,
		"instance":                 target,
		"status":                   "IN_PROGRESS",
		"started_at":               time.Now().UTC(),
		"previous_backup_id":       backupID,
		"previous_config_revision": state.ConfigRevision,
		"config_revision":          updatedState.ConfigRevision,
		"changed_client_fields":    changedClientFields,
	}
	if realityTargetCheck != nil {
		progressRecord["reality_target_verification"] = realityTargetCheck
	}
	if err := writeTransactionRecord(transactionDirectory, progressRecord); err != nil {
		return err
	}

	timerWasActive := execCommandSuccess("systemctl", "is-active", "--quiet", certificateRenewTimerUnitName) == nil
	mutated := false
	restartAttempted := false
	committed := false
	defer func() {
		if committed {
			_ = fsutil.RemoveManagedTree(stagingDirectory, transactionDirectory)
			_ = fsutil.RemoveManagedTree(snapshotDirectory, transactionDirectory)
			return
		}
		var rollbackError error
		if mutated {
			rollbackError = rollbackRestore(snapshotEntries, restartAttempted, timerWasActive, snapshotDirectory, state)
		}
		record := progressRecord
		record["status"] = "ROLLED_BACK"
		record["failed_at"] = time.Now().UTC()
		record["error"] = sanitizeError(returnErr, "")
		if rollbackError != nil {
			record["status"] = "ROLLBACK_FAILED"
			record["rollback_error"] = sanitizeError(rollbackError, "")
		}
		_ = writeTransactionRecord(transactionDirectory, record)
		_ = fsutil.RemoveManagedTree(stagingDirectory, transactionDirectory)
		_ = fsutil.RemoveManagedTree(snapshotDirectory, transactionDirectory)
		if returnErr == nil && rollbackError != nil {
			returnErr = rollbackError
		}
	}()

	mutated = true
	if err := activateProfileMutation(artifacts, stateBytes, secretBytes); err != nil {
		return err
	}
	if output, err := runCommand(installedSingBox, "check", "-c", serverConfigPath); err != nil {
		return fmt.Errorf("installed mutated configuration check failed: %w: %s", err, sanitizeText(output, ""))
	}
	if output, err := runCommand(installedXray, "run", "-test", "-config", xrayServerConfigPath); err != nil {
		return fmt.Errorf("installed mutated Xray configuration check failed: %w: %s", err, sanitizeText(output, ""))
	}
	restartAttempted = true
	if err := restartManagedServices(updatedState); err != nil {
		return fmt.Errorf("restart after instance mutation failed: %w", err)
	}
	if err := waitForManagedServiceState(updatedState, 30*time.Second); err != nil {
		return fmt.Errorf("instance mutation health check failed: %w", err)
	}
	if updatedState.Hysteria2.ID == "" {
		if output, err := runCommand("systemctl", "disable", "--now", certificateRenewTimerUnitName); err != nil && !strings.Contains(output, "not loaded") {
			return fmt.Errorf("disable unused certificate timer: %w: %s", err, output)
		}
	} else if updatedState.Hysteria2.Enabled && !timerWasActive {
		if output, err := runCommand("systemctl", "enable", "--now", certificateRenewTimerUnitName); err != nil {
			return fmt.Errorf("enable certificate timer: %w: %s", err, output)
		}
	}

	progressRecord["status"] = "COMMITTED"
	progressRecord["committed_at"] = time.Now().UTC()
	progressRecord["profile"] = updatedState.Profile
	progressRecord["config_sha256"] = updatedState.ConfigSHA256
	if err := writeTransactionRecord(transactionDirectory, progressRecord); err != nil {
		return err
	}
	committed = true
	_ = appendAudit(map[string]any{
		"time":           time.Now().UTC(),
		"transaction_id": transactionID,
		"command":        command,
		"instance":       target,
		"status":         "COMMITTED",
		"backup_id":      backupID,
	})
	return printJSON(commandResult{Command: command, Status: "PASS", Detail: map[string]any{
		"result":                 map[string]string{"enable": "ENABLED", "disable": "DISABLED", "modify": "MODIFIED", "delete": "DELETED"}[operation],
		"instance":               target,
		"profile":                updatedState.Profile,
		"transaction_id":         transactionID,
		"previous_backup_id":     backupID,
		"config_revision":        updatedState.ConfigRevision,
		"client_update_required": true,
		"changed_client_fields":  changedClientFields,
		"exports_regenerated":    true,
		"reality_server_name":    updatedState.RealityServerName,
	}})
}

func applyInstanceStateChange(state *model.State, secrets *model.Secrets, operation, target string, port int) error {
	switch target {
	case "reality":
		if state.Reality.ID == "" {
			return errors.New("reality instance does not exist")
		}
		switch operation {
		case "enable":
			if state.Reality.Enabled {
				return errors.New("reality instance is already enabled")
			}
			state.Reality.Enabled = true
		case "disable":
			if !state.Reality.Enabled {
				return errors.New("reality instance is already disabled")
			}
			if !state.Hysteria2.Enabled {
				return errors.New("refusing to disable the last enabled instance")
			}
			state.Reality.Enabled = false
		case "modify":
			if port == 0 {
				break
			}
			if port < 1 || port > 65535 {
				return fmt.Errorf("invalid Reality TCP port: %d", port)
			}
			if port == state.Reality.ListenPort {
				return errors.New("reality listen port is unchanged")
			}
			state.Reality.ListenPort = port
		case "delete":
			if !state.Hysteria2.Enabled {
				return errors.New("refusing to delete the last enabled instance")
			}
			state.Reality = model.RealityState{}
			secrets.RealityUUID = ""
			secrets.RealityPrivateKey = ""
		}
	case "hysteria2":
		if state.Hysteria2.ID == "" {
			return errors.New("Hysteria2 instance does not exist")
		}
		switch operation {
		case "enable":
			if state.Hysteria2.Enabled {
				return errors.New("Hysteria2 instance is already enabled")
			}
			state.Hysteria2.Enabled = true
		case "disable":
			if !state.Hysteria2.Enabled {
				return errors.New("Hysteria2 instance is already disabled")
			}
			if !state.Reality.Enabled {
				return errors.New("refusing to disable the last enabled instance")
			}
			state.Hysteria2.Enabled = false
		case "modify":
			if port == 0 {
				break
			}
			if port < 1 || port > 65535 {
				return fmt.Errorf("invalid Hysteria2 UDP port: %d", port)
			}
			if port == state.Hysteria2.ListenPort {
				return errors.New("Hysteria2 listen port is unchanged")
			}
			state.Hysteria2.ListenPort = port
		case "delete":
			if !state.Reality.Enabled {
				return errors.New("refusing to delete the last enabled instance")
			}
			state.Hysteria2 = model.Hysteria2State{}
			secrets.Hysteria2Password = ""
		}
	}
	return nil
}

func clientFacingChanges(previous, updated model.State) []string {
	changes := make([]string, 0, 4)
	if previous.Reality.Enabled != updated.Reality.Enabled || (previous.Reality.ID == "") != (updated.Reality.ID == "") {
		changes = append(changes, "reality.availability")
	}
	if previous.Reality.ListenPort != updated.Reality.ListenPort {
		changes = append(changes, "reality.port")
	}
	if previous.RealityServerName != updated.RealityServerName {
		changes = append(changes, "reality.server_name")
	}
	if previous.Hysteria2.Enabled != updated.Hysteria2.Enabled || (previous.Hysteria2.ID == "") != (updated.Hysteria2.ID == "") {
		changes = append(changes, "hysteria2.availability")
	}
	if previous.Hysteria2.ListenPort != updated.Hysteria2.ListenPort {
		changes = append(changes, "hysteria2.port")
	}
	return changes
}

func readInstalledSecrets() (model.Secrets, error) {
	bytes, err := os.ReadFile(secretPath)
	if err != nil {
		return model.Secrets{}, fmt.Errorf("read installed secrets: %w", err)
	}
	var secrets model.Secrets
	if err := json.Unmarshal(bytes, &secrets); err != nil {
		return model.Secrets{}, fmt.Errorf("parse installed secrets: %w", err)
	}
	return secrets, nil
}

func runtimeValuesFromState(state model.State, secrets model.Secrets) model.RuntimeValues {
	return model.RuntimeValues{
		RealityEnabled:    state.Reality.Enabled,
		Hysteria2Enabled:  state.Hysteria2.Enabled,
		ConnectHost:       state.ConnectHost,
		Domain:            state.Domain,
		RealityServerName: state.RealityServerName,
		TCPPort:           state.Reality.ListenPort,
		UDPPort:           state.Hysteria2.ListenPort,
		RealityUUID:       secrets.RealityUUID,
		RealityPrivateKey: secrets.RealityPrivateKey,
		RealityPublicKey:  state.Reality.PublicKey,
		RealityShortID:    state.Reality.ShortID,
		Hysteria2Password: secrets.Hysteria2Password,
		CertificatePath:   state.Hysteria2.CertificatePath,
		KeyPath:           state.Hysteria2.KeyPath,
	}
}

func renderProfileArtifacts(state model.State, secrets model.Secrets) (map[string][]byte, error) {
	values := runtimeValuesFromState(state, secrets)
	serverConfig, err := render.ServerConfig(values)
	if err != nil {
		return nil, err
	}
	xrayConfig, err := render.XrayRealityServerConfig(values)
	if err != nil {
		return nil, err
	}
	artifacts := map[string][]byte{
		serverConfigPath:                             serverConfig,
		xrayServerConfigPath:                         xrayConfig,
		filepath.Join(exportRoot, "mihomo.yaml"):     render.Mihomo(values),
		filepath.Join(exportRoot, "share-links.txt"): render.ShareLinks(values),
	}
	if state.Reality.Enabled {
		client, err := render.SingBoxRealityClient(values, 2080)
		if err != nil {
			return nil, err
		}
		artifacts[filepath.Join(exportRoot, "sing-box-reality.json")] = client
	}
	if state.Hysteria2.Enabled {
		client, err := render.SingBoxHysteria2Client(values, 2081)
		if err != nil {
			return nil, err
		}
		artifacts[filepath.Join(exportRoot, "sing-box-hysteria2.json")] = client
	}
	return artifacts, nil
}

func exportStateForProfile(state model.State) []model.ExportState {
	exports := []model.ExportState{
		{Format: "mihomo", Path: filepath.Join(exportRoot, "mihomo.yaml")},
		{Format: "share-link", Path: filepath.Join(exportRoot, "share-links.txt")},
	}
	if state.Reality.Enabled {
		exports = append(exports, model.ExportState{Format: "sing-box-reality", Path: filepath.Join(exportRoot, "sing-box-reality.json")})
	}
	if state.Hysteria2.Enabled {
		exports = append(exports, model.ExportState{Format: "sing-box-hysteria2", Path: filepath.Join(exportRoot, "sing-box-hysteria2.json")})
	}
	return exports
}

func stageProfileMutation(stagingDirectory string, artifacts map[string][]byte, stateBytes, secretBytes []byte) error {
	for path, content := range artifacts {
		name := filepath.Base(path)
		if path == serverConfigPath {
			name = "sing-box.json"
		} else if path == xrayServerConfigPath {
			name = "xray.json"
		}
		if err := fsutil.WriteFileAtomic(filepath.Join(stagingDirectory, name), content, 0o600); err != nil {
			return err
		}
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(stagingDirectory, "state.json"), append(stateBytes, '\n'), 0o600); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(filepath.Join(stagingDirectory, "instances.json"), append(secretBytes, '\n'), 0o600)
}

func activateProfileMutation(artifacts map[string][]byte, stateBytes, secretBytes []byte) error {
	for path, content := range artifacts {
		mode := os.FileMode(0o600)
		if path == serverConfigPath || path == xrayServerConfigPath {
			mode = 0o640
		}
		if err := fsutil.WriteFileAtomic(path, content, mode); err != nil {
			return err
		}
	}
	for _, path := range allProfileExportPaths {
		if _, present := artifacts[path]; present {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove stale export %s: %w", path, err)
		}
	}
	if err := fsutil.WriteFileAtomic(secretPath, append(secretBytes, '\n'), 0o600); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(statePath, append(stateBytes, '\n'), 0o600); err != nil {
		return err
	}
	return setServiceFileOwnership()
}

func verifyNewInstancePortAvailable(target string, port int) error {
	if target == "reality" {
		listener, err := net.Listen("tcp", fmt.Sprintf("[::]:%d", port))
		if err != nil {
			return fmt.Errorf("TCP port %d is unavailable: %w", port, err)
		}
		return listener.Close()
	}
	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("::"), Port: port})
	if err != nil {
		return fmt.Errorf("UDP port %d is unavailable: %w", port, err)
	}
	return listener.Close()
}
