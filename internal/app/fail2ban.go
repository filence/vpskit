package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/fsutil"
	"vpskit.local/vpskit/internal/platform"
)

const (
	fail2banCommand                = "fail2ban-client"
	fail2banServiceName            = "fail2ban.service"
	fail2banJailName               = "sshd"
	fail2banJailPath               = "/etc/fail2ban/jail.d/vpskit-sshd.conf"
	fail2banOwnershipPath          = "/var/lib/vpskit/fail2ban.json"
	fail2banOwnershipSchemaVersion = 2
	maxFail2banWhitelistEntries    = 32
)

type fail2banOwnership struct {
	SchemaVersion  int       `json:"schema_version"`
	PackageManaged bool      `json:"package_managed"`
	JailPath       string    `json:"jail_path"`
	SSHPort        int       `json:"ssh_port"`
	ConfigSHA256   string    `json:"config_sha256"`
	AppliedAt      time.Time `json:"applied_at"`
	Whitelist      []string  `json:"whitelist,omitempty"`
}

func runSecurity(arguments []string) error {
	if len(arguments) == 0 || arguments[0] != "fail2ban" {
		return errors.New("usage: vpskit security fail2ban <status|plan|apply|remove|whitelist>")
	}
	return runFail2ban(arguments[1:])
}

func runFail2ban(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: vpskit security fail2ban <status|plan|apply|remove|whitelist>")
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
	case "whitelist":
		return runFail2banWhitelist(arguments[1:])
	default:
		return errors.New("usage: vpskit security fail2ban <status|plan|apply|remove|whitelist>")
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
		detail["whitelist_count"] = len(owner.Whitelist)
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
	whitelist := []string(nil)
	if existing, err := readFail2banOwnership(); err == nil {
		whitelist = existing.Whitelist
	}
	contents := renderFail2banSSHDJail(port, whitelist)
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
			ownership := fail2banOwnership{SchemaVersion: fail2banOwnershipSchemaVersion, PackageManaged: !installedBefore, JailPath: fail2banJailPath, SSHPort: port, ConfigSHA256: sha256Bytes([]byte(contents)), AppliedAt: time.Now().UTC(), Whitelist: whitelist}
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

func runFail2banWhitelist(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: vpskit security fail2ban whitelist <list|add|remove> [--cidr <IP-or-CIDR>] [--yes]")
	}
	operation := strings.ToLower(strings.TrimSpace(arguments[0]))
	if operation == "list" {
		if len(arguments) != 1 {
			return errors.New("usage: vpskit security fail2ban whitelist list")
		}
		owner, err := readFail2banOwnership()
		if errors.Is(err, os.ErrNotExist) {
			return printJSON(commandResult{Command: "security fail2ban whitelist list", Status: "PASS", Detail: []string{}})
		}
		if err != nil {
			return err
		}
		return printJSON(commandResult{Command: "security fail2ban whitelist list", Status: "PASS", Detail: owner.Whitelist})
	}
	if operation != "add" && operation != "remove" {
		return errors.New("usage: vpskit security fail2ban whitelist <list|add|remove> [--cidr <IP-or-CIDR>] [--yes]")
	}
	if !platform.IsRoot() {
		return fmt.Errorf("security fail2ban whitelist %s requires root privileges", operation)
	}
	flags := flag.NewFlagSet("security fail2ban whitelist "+operation, flag.ContinueOnError)
	cidr := flags.String("cidr", "", "trusted IP address or CIDR")
	yes := flags.Bool("yes", false, "confirm VPSKit SSH jail update")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*cidr) == "" || !*yes {
		return fmt.Errorf("security fail2ban whitelist %s requires --cidr <IP-or-CIDR> --yes", operation)
	}
	value, err := normalizedFail2banWhitelistEntry(*cidr)
	if err != nil {
		return err
	}
	return mutateFail2banWhitelist(operation, value)
}

func normalizedFail2banWhitelist(input []string) ([]string, error) {
	if len(input) > maxFail2banWhitelistEntries {
		return nil, fmt.Errorf("at most %d Fail2ban whitelist entries are supported", maxFail2banWhitelistEntries)
	}
	seen := map[string]bool{}
	values := make([]string, 0, len(input))
	for _, raw := range input {
		value, err := normalizedFail2banWhitelistEntry(raw)
		if err != nil {
			return nil, err
		}
		if seen[value] {
			return nil, fmt.Errorf("duplicate Fail2ban whitelist entry %q", value)
		}
		seen[value] = true
		values = append(values, value)
	}
	sort.Strings(values)
	return values, nil
}

func normalizedFail2banWhitelistEntry(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", errors.New("Fail2ban whitelist entry is empty")
	}
	if address := net.ParseIP(value); address != nil {
		if address.To4() != nil {
			return address.String() + "/32", nil
		}
		return address.String() + "/128", nil
	}
	_, network, err := net.ParseCIDR(value)
	if err != nil {
		return "", fmt.Errorf("invalid trusted IP or CIDR %q", raw)
	}
	return network.String(), nil
}

func mutateFail2banWhitelist(operation, value string) error {
	owner, err := readFail2banOwnership()
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("VPSKit SSH jail is not managed; run security fail2ban apply --yes first")
	}
	if err != nil {
		return err
	}
	if owner.JailPath != fail2banJailPath || !fail2banJailMatchesOwnership(owner) {
		return errors.New("refusing to update a modified or non-VPSKit Fail2ban jail")
	}
	updated := append([]string(nil), owner.Whitelist...)
	if operation == "add" {
		updated = append(updated, value)
	} else {
		index := -1
		for current, candidate := range updated {
			if candidate == value {
				index = current
				break
			}
		}
		if index < 0 {
			return fmt.Errorf("Fail2ban whitelist entry %q does not exist", value)
		}
		updated = append(updated[:index], updated[index+1:]...)
	}
	updated, err = normalizedFail2banWhitelist(updated)
	if err != nil {
		return err
	}
	newContents := renderFail2banSSHDJail(owner.SSHPort, updated)
	oldContents, err := os.ReadFile(fail2banJailPath)
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(fail2banJailPath, []byte(newContents), 0o644); err != nil {
		return err
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		_ = fsutil.WriteFileAtomic(fail2banJailPath, oldContents, 0o644)
		_, _ = runCommand("systemctl", "restart", fail2banServiceName)
	}()
	if output, err := runCommand(fail2banCommand, "-d"); err != nil {
		return fmt.Errorf("validate Fail2ban whitelist configuration: %w: %s", err, sanitizeText(output, ""))
	}
	if output, err := runCommand("systemctl", "restart", fail2banServiceName); err != nil {
		return fmt.Errorf("restart Fail2ban after whitelist update: %w: %s", err, sanitizeText(output, ""))
	}
	// systemd reports a successful restart before fail2ban has always opened its
	// control socket.  Poll the actual jail briefly, instead of rolling a valid
	// configuration back solely because that socket is still starting up.
	readyDeadline := time.Now().Add(15 * time.Second)
	var lastStatusOutput string
	var lastStatusErr error
	jailReady := false
	for time.Now().Before(readyDeadline) {
		output, statusErr := runCommand(fail2banCommand, "status", fail2banJailName)
		if statusErr == nil {
			jailReady = true
			break
		}
		lastStatusOutput = output
		lastStatusErr = statusErr
		time.Sleep(500 * time.Millisecond)
	}
	if !jailReady {
		return fmt.Errorf("Fail2ban SSH jail is not active after whitelist update: %w: %s", lastStatusErr, sanitizeText(lastStatusOutput, ""))
	}
	owner.SchemaVersion = fail2banOwnershipSchemaVersion
	owner.Whitelist = updated
	owner.ConfigSHA256 = sha256Bytes([]byte(newContents))
	owner.AppliedAt = time.Now().UTC()
	if err := writeFail2banOwnership(owner); err != nil {
		return err
	}
	committed = true
	_ = appendAudit(map[string]any{"command": "security fail2ban whitelist " + operation, "status": "COMPLETED", "entry": value})
	return printJSON(commandResult{Command: "security fail2ban whitelist " + operation, Status: "PASS", Detail: map[string]any{
		"entry": value, "whitelist": updated, "service": "active", "jail": fail2banJailName,
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

func renderFail2banSSHDJail(port int, whitelist []string) string {
	contents := "# Managed by VPSKit. Do not edit; use vpskit security fail2ban remove --yes first.\n[DEFAULT]\nbackend = systemd\nbantime = 1h\nfindtime = 10m\nmaxretry = 5\n"
	if len(whitelist) > 0 {
		contents += "ignoreip = 127.0.0.1/8 ::1 " + strings.Join(whitelist, " ") + "\n"
	}
	return fmt.Sprintf("%s\n[%s]\nenabled = true\nfilter = sshd\nport = %d\n", contents, fail2banJailName, port)
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
	if (value.SchemaVersion != 1 && value.SchemaVersion != fail2banOwnershipSchemaVersion) || value.JailPath == "" || value.ConfigSHA256 == "" {
		return fail2banOwnership{}, errors.New("invalid Fail2ban ownership record")
	}
	whitelist, err := normalizedFail2banWhitelist(value.Whitelist)
	if err != nil {
		return fail2banOwnership{}, fmt.Errorf("invalid Fail2ban whitelist: %w", err)
	}
	value.Whitelist = whitelist
	return value, nil
}

func writeFail2banOwnership(value fail2banOwnership) error {
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(filepath.Clean(fail2banOwnershipPath), contents, 0o600)
}
