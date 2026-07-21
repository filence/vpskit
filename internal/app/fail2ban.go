package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/fsutil"
	"vpskit.local/vpskit/internal/platform"
)

const (
	fail2banCommand       = "fail2ban-client"
	fail2banServiceName   = "fail2ban.service"
	fail2banJailName      = "sshd"
	fail2banJailPath      = "/etc/fail2ban/jail.d/vpskit-sshd.conf"
	fail2banOwnershipPath = "/var/lib/vpskit/fail2ban.json"
)

type fail2banOwnership struct {
	SchemaVersion  int       `json:"schema_version"`
	PackageManaged bool      `json:"package_managed"`
	JailPath       string    `json:"jail_path"`
	SSHPort        int       `json:"ssh_port"`
	ConfigSHA256   string    `json:"config_sha256"`
	AppliedAt      time.Time `json:"applied_at"`
}

func runSecurity(arguments []string) error {
	if len(arguments) == 0 || arguments[0] != "fail2ban" {
		return errors.New("usage: vpskit security fail2ban <status|plan|apply|remove>")
	}
	return runFail2ban(arguments[1:])
}

func runFail2ban(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: vpskit security fail2ban <status|plan|apply|remove>")
	}
	switch arguments[0] {
	case "status":
		if len(arguments) != 1 {
			return errors.New("usage: vpskit security fail2ban status")
		}
		return runFail2banStatus()
	case "plan":
		if len(arguments) != 1 {
			return errors.New("usage: vpskit security fail2ban plan")
		}
		return runFail2banPlan()
	case "apply":
		if len(arguments) != 2 || arguments[1] != "--yes" {
			return errors.New("usage: vpskit security fail2ban apply --yes")
		}
		return applyFail2ban()
	case "remove":
		if len(arguments) != 2 || arguments[1] != "--yes" {
			return errors.New("usage: vpskit security fail2ban remove --yes")
		}
		return removeFail2ban()
	default:
		return errors.New("usage: vpskit security fail2ban <status|plan|apply|remove>")
	}
}

func runFail2banStatus() error {
	status, err := collectFail2banStatus()
	if err != nil {
		return err
	}
	return printJSON(commandResult{Command: "security fail2ban status", Status: "PASS", Detail: status})
}

func runFail2banPlan() error {
	status, err := collectFail2banStatus()
	if err != nil {
		return err
	}
	port, err := detectedSSHPort()
	if err != nil {
		return err
	}
	status["planned_jail"] = map[string]any{
		"name":       fail2banJailName,
		"path":       fail2banJailPath,
		"ssh_port":   port,
		"backend":    "systemd",
		"maxretry":   5,
		"findtime":   "10m",
		"bantime":    "1h",
		"scope":      "VPSKit-owned SSH jail override only",
		"exclusions": []string{"VPSKit proxy services", "unknown Fail2ban jails", "SSH configuration"},
	}
	return printJSON(commandResult{Command: "security fail2ban plan", Status: "PASS", Detail: status})
}

func collectFail2banStatus() (map[string]any, error) {
	installed := execCommandSuccess("sh", "-c", "command -v fail2ban-client >/dev/null 2>&1") == nil
	jailManaged := false
	if info, err := os.Lstat(fail2banJailPath); err == nil {
		jailManaged = info.Mode().IsRegular()
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	owner, ownerErr := readFail2banOwnership()
	if ownerErr != nil && !errors.Is(ownerErr, os.ErrNotExist) {
		return nil, ownerErr
	}
	detail := map[string]any{
		"installed":                installed,
		"service_state":            systemdUnitState(fail2banServiceName),
		"vpskit_jail_file_present": jailManaged,
		"vpskit_ownership_present": ownerErr == nil,
		"jail_name":                fail2banJailName,
		"jail_path":                fail2banJailPath,
	}
	if ownerErr == nil {
		detail["ownership"] = owner
	}
	if installed && systemdUnitState(fail2banServiceName) == "active" {
		output, err := runCommand(fail2banCommand, "status", fail2banJailName)
		detail["vpskit_jail_active"] = err == nil
		if err != nil {
			detail["jail_status_error"] = sanitizeText(output, "")
		}
	}
	return detail, nil
}

func applyFail2ban() error {
	if !platform.IsRoot() {
		return errors.New("security fail2ban apply requires root privileges")
	}
	if err := requireSystemdJournal(); err != nil {
		return err
	}
	port, err := detectedSSHPort()
	if err != nil {
		return err
	}
	if existing, err := os.Lstat(fail2banJailPath); err == nil {
		if !existing.Mode().IsRegular() {
			return fmt.Errorf("refusing to replace non-regular VPSKit Fail2ban jail path: %s", fail2banJailPath)
		}
		owner, ownerErr := readFail2banOwnership()
		if ownerErr != nil || owner.JailPath != fail2banJailPath || !fail2banJailMatchesOwnership(owner) {
			return errors.New("refusing to replace existing Fail2ban jail without matching VPSKit ownership")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	installedBefore := execCommandSuccess("sh", "-c", "command -v fail2ban-client >/dev/null 2>&1") == nil
	if !installedBefore {
		if output, err := runCommand("apt-get", "install", "--yes", "--no-install-recommends", "fail2ban"); err != nil {
			return fmt.Errorf("install fail2ban: %w: %s", err, sanitizeText(output, ""))
		}
	}
	contents := renderFail2banSSHDJail(port)
	if err := fsutil.WriteFileAtomic(fail2banJailPath, []byte(contents), 0o644); err != nil {
		return err
	}
	if output, err := runCommand(fail2banCommand, "-d"); err != nil {
		return fmt.Errorf("validate Fail2ban configuration: %w: %s", err, sanitizeText(output, ""))
	}
	if output, err := runCommand("systemctl", "enable", fail2banServiceName); err != nil {
		return fmt.Errorf("enable Fail2ban service: %w: %s", err, sanitizeText(output, ""))
	}
	if output, err := runCommand("systemctl", "restart", fail2banServiceName); err != nil {
		return fmt.Errorf("restart Fail2ban service with VPSKit SSH jail: %w: %s", err, sanitizeText(output, ""))
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if output, err := runCommand(fail2banCommand, "status", fail2banJailName); err == nil {
			ownership := fail2banOwnership{SchemaVersion: 1, PackageManaged: !installedBefore, JailPath: fail2banJailPath, SSHPort: port, ConfigSHA256: sha256Bytes([]byte(contents)), AppliedAt: time.Now().UTC()}
			if err := writeFail2banOwnership(ownership); err != nil {
				return err
			}
			_ = appendAudit(map[string]any{"command": "security fail2ban apply", "status": "COMPLETED", "ssh_port": port})
			return printJSON(commandResult{Command: "security fail2ban apply", Status: "PASS", Detail: map[string]any{
				"ssh_port":        port,
				"service":         "active",
				"jail":            fail2banJailName,
				"jail_status":     sanitizeText(output, ""),
				"package_managed": !installedBefore,
			}})
		}
		time.Sleep(500 * time.Millisecond)
	}
	return errors.New("Fail2ban service started but VPSKit SSH jail did not become active")
}

func removeFail2ban() error {
	if !platform.IsRoot() {
		return errors.New("security fail2ban remove requires root privileges")
	}
	owner, err := readFail2banOwnership()
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("VPSKit does not own a Fail2ban configuration to remove")
	}
	if err != nil {
		return err
	}
	if owner.JailPath != fail2banJailPath {
		return errors.New("refusing to remove unexpected Fail2ban jail ownership path")
	}
	if _, err := os.Lstat(fail2banJailPath); err == nil && !fail2banJailMatchesOwnership(owner) {
		return errors.New("refusing to remove a modified VPSKit Fail2ban jail")
	}
	if err := os.Remove(fail2banJailPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if execCommandSuccess("sh", "-c", "command -v fail2ban-client >/dev/null 2>&1") == nil {
		if output, err := runCommand("systemctl", "reload-or-restart", fail2banServiceName); err != nil {
			return fmt.Errorf("reload Fail2ban after VPSKit jail removal: %w: %s", err, sanitizeText(output, ""))
		}
	}
	if err := os.Remove(fail2banOwnershipPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_ = appendAudit(map[string]any{"command": "security fail2ban remove", "status": "COMPLETED", "package_retained": true})
	return printJSON(commandResult{Command: "security fail2ban remove", Status: "PASS", Detail: map[string]any{
		"removed_jail":     fail2banJailPath,
		"package_retained": true,
		"note":             "The package is retained to avoid removing a user-managed installation.",
	}})
}

func detectedSSHPort() (int, error) {
	output, err := runCommand("sshd", "-T")
	if err != nil {
		return 0, fmt.Errorf("read effective SSH configuration: %w: %s", err, sanitizeText(output, ""))
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "port" {
			port, err := strconv.Atoi(fields[1])
			if err == nil && port > 0 && port <= 65535 {
				return port, nil
			}
		}
	}
	return 0, errors.New("effective SSH configuration has no valid port")
}

func requireSystemdJournal() error {
	if execCommandSuccess("systemctl", "is-active", "--quiet", "systemd-journald.service") != nil {
		return errors.New("Fail2ban SSH jail requires active systemd-journald")
	}
	return nil
}

func renderFail2banSSHDJail(port int) string {
	return fmt.Sprintf("# Managed by VPSKit. Do not edit; use vpskit security fail2ban remove --yes first.\n[DEFAULT]\nbackend = systemd\nbantime = 1h\nfindtime = 10m\nmaxretry = 5\n\n[%s]\nenabled = true\nfilter = sshd\nport = %d\n", fail2banJailName, port)
}

func fail2banJailMatchesOwnership(owner fail2banOwnership) bool {
	contents, err := os.ReadFile(fail2banJailPath)
	return err == nil && strings.HasPrefix(string(contents), "# Managed by VPSKit.") && owner.ConfigSHA256 != "" && strings.EqualFold(sha256Bytes(contents), owner.ConfigSHA256)
}

func readFail2banOwnership() (fail2banOwnership, error) {
	contents, err := os.ReadFile(fail2banOwnershipPath)
	if err != nil {
		return fail2banOwnership{}, err
	}
	var value fail2banOwnership
	if err := json.Unmarshal(contents, &value); err != nil {
		return fail2banOwnership{}, err
	}
	if value.SchemaVersion != 1 || value.JailPath == "" || value.ConfigSHA256 == "" {
		return fail2banOwnership{}, errors.New("invalid Fail2ban ownership record")
	}
	return value, nil
}

func writeFail2banOwnership(value fail2banOwnership) error {
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(filepath.Clean(fail2banOwnershipPath), contents, 0o600)
}
