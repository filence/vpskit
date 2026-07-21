package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/platform"
)

type doctorFixAction struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// runDoctor keeps the diagnostic-only command as the default. --fix is
// deliberately narrow: it repairs only VPSKit-owned runtime prerequisites and
// re-enables services which the installed state explicitly declares enabled.
func runDoctor(arguments []string) error {
	fix, err := parseDoctorFixArguments(arguments)
	if err != nil {
		return err
	}
	if !fix {
		return runStatus("doctor")
	}
	return runDoctorFix()
}

func parseDoctorFixArguments(arguments []string) (bool, error) {
	if len(arguments) == 0 {
		return false, nil
	}
	if len(arguments) == 1 && arguments[0] == "--fix" {
		return true, nil
	}
	return false, errors.New("usage: vpskit doctor [--fix]")
}

func runDoctorFix() error {
	if !platform.IsRoot() {
		return errors.New("doctor --fix requires root privileges")
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	actions := make([]doctorFixAction, 0, 8)

	if recovered, err := recoverIncompleteMutation(); err != nil {
		return fmt.Errorf("recover incomplete VPSKit transaction before fixing: %w", err)
	} else if recovered != nil {
		actions = append(actions, doctorFixAction{Name: "recover_incomplete_transaction", Status: "FIXED", Detail: recovered.ID})
	} else {
		actions = append(actions, doctorFixAction{Name: "recover_incomplete_transaction", Status: "NOT_NEEDED"})
	}

	for _, directory := range []struct {
		path string
		mode os.FileMode
	}{
		{stateRoot, 0o750},
		{filepath.Join(stateRoot, "locks"), 0o700},
		{transactionRoot, 0o700},
		{logRoot, 0o750},
	} {
		changed, err := ensureDoctorDirectory(directory.path, directory.mode)
		if err != nil {
			return err
		}
		status := "OK"
		if changed {
			status = "FIXED"
		}
		actions = append(actions, doctorFixAction{Name: "managed_directory", Status: status, Detail: directory.path})
	}

	if err := validateDoctorManagedConfig(state); err != nil {
		return err
	}
	actions = append(actions, doctorFixAction{Name: "managed_configuration", Status: "OK"})
	if output, err := runCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("systemd daemon-reload failed: %w: %s", err, sanitizeText(output, ""))
	}
	actions = append(actions, doctorFixAction{Name: "systemd_daemon_reload", Status: "FIXED"})

	if state.Reality.Enabled {
		if err := reconcileManagedService(xrayServiceUnitName, true); err != nil {
			return err
		}
		actions = append(actions, doctorFixAction{Name: "xray_service", Status: "FIXED", Detail: "enabled and active"})
	}
	if state.Hysteria2.Enabled {
		if err := reconcileManagedService(serviceUnitName, true); err != nil {
			return err
		}
		actions = append(actions, doctorFixAction{Name: "sing_box_service", Status: "FIXED", Detail: "enabled and active"})
		if err := enableDoctorCertificateTimer(); err != nil {
			return err
		}
		actions = append(actions, doctorFixAction{Name: "certificate_renew_timer", Status: "FIXED", Detail: "enabled and active"})
	}
	if err := healthCheckState(state); err != nil {
		return fmt.Errorf("doctor --fix post-check failed: %w", err)
	}
	_ = appendAudit(map[string]any{
		"command": "doctor --fix",
		"status":  "COMPLETED",
		"actions": actions,
	})
	return printJSON(commandResult{Command: "doctor", Status: "PASS", Detail: map[string]any{
		"mode":    "fix",
		"actions": actions,
	}})
}

func ensureDoctorDirectory(path string, mode os.FileMode) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(path, mode); err != nil {
			return false, fmt.Errorf("create VPSKit managed directory %s: %w", path, err)
		}
		if err := os.Chmod(path, mode); err != nil {
			return false, fmt.Errorf("set VPSKit managed directory mode %s: %w", path, err)
		}
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("refusing to repair non-directory VPSKit path: %s", path)
	}
	if info.Mode().Perm() == mode.Perm() {
		return false, nil
	}
	if err := os.Chmod(path, mode); err != nil {
		return false, fmt.Errorf("set VPSKit managed directory mode %s: %w", path, err)
	}
	return true, nil
}

func validateDoctorManagedConfig(state model.State) error {
	if err := doctorConfigHashMatches(serverConfigPath, state.ConfigSHA256); err != nil {
		return fmt.Errorf("refusing doctor --fix because managed sing-box config is not trusted: %w", err)
	}
	if err := doctorConfigHashMatches(xrayServerConfigPath, state.RealityConfigSHA256); err != nil {
		return fmt.Errorf("refusing doctor --fix because managed Xray config is not trusted: %w", err)
	}
	if state.Hysteria2.Enabled {
		if output, err := runCommand(installedSingBox, "check", "-c", serverConfigPath); err != nil {
			return fmt.Errorf("refusing doctor --fix because sing-box configuration validation failed: %w: %s", err, sanitizeText(output, ""))
		}
	}
	if state.Reality.Enabled {
		if output, err := runCommand(installedXray, "run", "-test", "-config", xrayServerConfigPath); err != nil {
			return fmt.Errorf("refusing doctor --fix because Xray configuration validation failed: %w: %s", err, sanitizeText(output, ""))
		}
	}
	return nil
}

func doctorConfigHashMatches(path, expected string) error {
	if strings.TrimSpace(expected) == "" {
		return errors.New("state has no expected checksum")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(sha256Bytes(contents), expected) {
		return errors.New("checksum mismatch")
	}
	return nil
}

func enableDoctorCertificateTimer() error {
	for _, path := range []string{managedCertificate, managedKey, certificateRenewTimerUnitPath} {
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("certificate renewal prerequisite %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("certificate renewal prerequisite is not a regular file: %s", path)
		}
	}
	if output, err := runCommand("systemctl", "enable", "--now", certificateRenewTimerUnitName); err != nil {
		return fmt.Errorf("start certificate renewal timer: %w: %s", err, sanitizeText(output, ""))
	}
	return nil
}
