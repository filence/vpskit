package app

import (
	"encoding/json"
	"errors"
	"flag"
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
)

const (
	hysteria2UDPBufferProfileConservative2MiB = "conservative-2mib"
	hysteria2UDPBufferBytes2MiB               = int64(2 * 1024 * 1024)
	hysteria2UDPBufferOwnershipSchema         = 1
)

var hysteria2UDPBufferSysctls = []string{
	"net.core.rmem_default",
	"net.core.wmem_default",
	"net.core.rmem_max",
	"net.core.wmem_max",
}

type hysteria2UDPBufferOwnership struct {
	SchemaVersion int              `json:"schema_version"`
	Profile       string           `json:"profile"`
	SysctlPath    string           `json:"sysctl_path"`
	ConfigSHA256  string           `json:"config_sha256"`
	Previous      map[string]int64 `json:"previous_sysctls"`
	AppliedAt     time.Time        `json:"applied_at"`
}

type hysteria2UDPSocketBuffers struct {
	ReceiveBytes int64 `json:"receive_bytes"`
	SendBytes    int64 `json:"send_bytes"`
}

func runHysteria2UDPBuffer(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: vpskit hysteria2 udp-buffer <status|plan|apply|rollback> [--profile conservative-2mib] [--yes]")
	}
	operation := strings.ToLower(strings.TrimSpace(arguments[0]))
	if operation != "status" && operation != "plan" && operation != "apply" && operation != "rollback" {
		return errors.New("usage: vpskit hysteria2 udp-buffer <status|plan|apply|rollback> [--profile conservative-2mib] [--yes]")
	}
	flags := flag.NewFlagSet("hysteria2 udp-buffer "+operation, flag.ContinueOnError)
	profile := flags.String("profile", hysteria2UDPBufferProfileConservative2MiB, "managed UDP-buffer profile")
	yes := flags.Bool("yes", false, "confirm the managed system UDP-buffer change")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if operation == "status" && (len(arguments) != 1 || *yes || *profile != hysteria2UDPBufferProfileConservative2MiB) {
		return errors.New("usage: vpskit hysteria2 udp-buffer status")
	}
	if operation == "rollback" && (*profile != hysteria2UDPBufferProfileConservative2MiB || !*yes) {
		return errors.New("usage: vpskit hysteria2 udp-buffer rollback --yes")
	}
	if operation == "plan" && *yes {
		return errors.New("hysteria2 udp-buffer plan does not accept --yes")
	}
	if operation == "apply" && !*yes {
		return errors.New("usage: vpskit hysteria2 udp-buffer apply --profile conservative-2mib --yes")
	}
	if !platform.IsRoot() {
		return fmt.Errorf("hysteria2 udp-buffer %s requires root privileges", operation)
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	if !state.Hysteria2.Enabled || state.Hysteria2.ListenPort < 1 {
		return errors.New("Hysteria2 must be enabled before managing its UDP buffer")
	}
	if _, err := hysteria2UDPBufferProfile(*profile); err != nil && operation != "rollback" {
		return err
	}
	switch operation {
	case "status":
		detail, err := collectHysteria2UDPBufferStatus(state)
		if err != nil {
			return err
		}
		return printJSON(commandResult{Command: "hysteria2 udp-buffer status", Status: "PASS", Detail: detail})
	case "plan":
		return runHysteria2UDPBufferPlan(state, *profile)
	case "apply":
		return applyHysteria2UDPBuffer(state, *profile)
	default:
		return rollbackHysteria2UDPBuffer(state)
	}
}

func runHysteria2UDPBufferPlan(state model.State, profile string) error {
	target, _ := hysteria2UDPBufferProfile(profile)
	detail, err := collectHysteria2UDPBufferStatus(state)
	if err != nil {
		return err
	}
	detail["requested_profile"] = profile
	detail["target_sysctls"] = target
	detail["managed_file"] = hysteria2UDPBufferSysctlPath
	detail["service_restart"] = serviceUnitName
	detail["client_update_required"] = false
	detail["manual_validation_required"] = false
	detail["acceptance"] = []string{
		"all four sysctl values are set to the managed profile",
		"sing-box is active and its UDP listener remains present",
		"the actual UDP socket receive and send buffers meet the profile target",
	}
	return printJSON(commandResult{Command: "hysteria2 udp-buffer plan", Status: "PASS", Detail: detail})
}

func applyHysteria2UDPBuffer(state model.State, profile string) error {
	target, _ := hysteria2UDPBufferProfile(profile)
	lock, err := platform.AcquireProcessLock(filepath.Join(stateRoot, "locks", "vpskit.lock"))
	if err != nil {
		return err
	}
	defer lock.Close()
	if existing, err := readHysteria2UDPBufferOwnership(); err == nil {
		if existing.Profile != profile || !hysteria2UDPBufferConfigMatchesOwnership(existing) {
			return errors.New("refusing to replace a modified or differently managed VPSKit UDP-buffer profile; run rollback --yes first")
		}
		if buffers, err := currentHysteria2UDPSocketBuffers(state.Hysteria2.ListenPort); err == nil && buffers.ReceiveBytes >= hysteria2UDPBufferBytes2MiB && buffers.SendBytes >= hysteria2UDPBufferBytes2MiB {
			return printJSON(commandResult{Command: "hysteria2 udp-buffer apply", Status: "PASS", Detail: map[string]any{
				"result": "ALREADY_APPLIED", "profile": profile, "socket_buffers": buffers, "client_update_required": false,
			}})
		}
		return errors.New("the VPSKit UDP-buffer profile exists but the running socket does not meet it; run rollback --yes before applying again")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := os.Lstat(hysteria2UDPBufferSysctlPath); err == nil {
		return fmt.Errorf("refusing to replace existing unmanaged sysctl file: %s", hysteria2UDPBufferSysctlPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	previous, err := readHysteria2UDPBufferSysctls()
	if err != nil {
		return err
	}
	contents := renderHysteria2UDPBufferSysctl(target)
	ownership := hysteria2UDPBufferOwnership{
		SchemaVersion: hysteria2UDPBufferOwnershipSchema,
		Profile:       profile,
		SysctlPath:    hysteria2UDPBufferSysctlPath,
		ConfigSHA256:  sha256Bytes([]byte(contents)),
		Previous:      previous,
		AppliedAt:     time.Now().UTC(),
	}
	if err := fsutil.WriteFileAtomic(hysteria2UDPBufferSysctlPath, []byte(contents), 0o644); err != nil {
		return err
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		_ = os.Remove(hysteria2UDPBufferSysctlPath)
		_ = os.Remove(hysteria2UDPBufferStatePath)
		_ = applyHysteria2UDPBufferSysctls(previous)
		_, _ = runCommand("systemctl", "restart", serviceUnitName)
	}()
	if err := writeHysteria2UDPBufferOwnership(ownership); err != nil {
		return err
	}
	if err := applyHysteria2UDPBufferSysctls(target); err != nil {
		return err
	}
	if output, err := runCommand("systemctl", "restart", serviceUnitName); err != nil {
		return fmt.Errorf("restart %s after UDP-buffer change: %w: %s", serviceUnitName, err, strings.TrimSpace(output))
	}
	buffers, err := waitForHysteria2UDPSocketBuffers(state.Hysteria2.ListenPort, hysteria2UDPBufferBytes2MiB, 15*time.Second)
	if err != nil {
		return err
	}
	committed = true
	_ = appendAudit(map[string]any{"command": "hysteria2 udp-buffer apply", "status": "COMPLETED", "profile": profile, "bytes": hysteria2UDPBufferBytes2MiB})
	return printJSON(commandResult{Command: "hysteria2 udp-buffer apply", Status: "PASS", Detail: map[string]any{
		"result": "APPLIED", "profile": profile, "sysctl_path": hysteria2UDPBufferSysctlPath, "socket_buffers": buffers,
		"client_update_required": false, "manual_validation_required": false,
	}})
}

func rollbackHysteria2UDPBuffer(state model.State) error {
	lock, err := platform.AcquireProcessLock(filepath.Join(stateRoot, "locks", "vpskit.lock"))
	if err != nil {
		return err
	}
	defer lock.Close()
	ownership, err := readHysteria2UDPBufferOwnership()
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("VPSKit does not own a UDP-buffer profile to roll back")
	}
	if err != nil {
		return err
	}
	if !hysteria2UDPBufferConfigMatchesOwnership(ownership) {
		return errors.New("refusing to remove a modified VPSKit UDP-buffer sysctl file")
	}
	if err := os.Remove(hysteria2UDPBufferSysctlPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		contents := renderHysteria2UDPBufferSysctl(map[string]int64{
			"net.core.rmem_default": hysteria2UDPBufferBytes2MiB, "net.core.wmem_default": hysteria2UDPBufferBytes2MiB,
			"net.core.rmem_max": hysteria2UDPBufferBytes2MiB, "net.core.wmem_max": hysteria2UDPBufferBytes2MiB,
		})
		_ = fsutil.WriteFileAtomic(hysteria2UDPBufferSysctlPath, []byte(contents), 0o644)
		_ = applyHysteria2UDPBufferSysctls(map[string]int64{
			"net.core.rmem_default": hysteria2UDPBufferBytes2MiB, "net.core.wmem_default": hysteria2UDPBufferBytes2MiB,
			"net.core.rmem_max": hysteria2UDPBufferBytes2MiB, "net.core.wmem_max": hysteria2UDPBufferBytes2MiB,
		})
		_, _ = runCommand("systemctl", "restart", serviceUnitName)
	}()
	if err := applyHysteria2UDPBufferSysctls(ownership.Previous); err != nil {
		return err
	}
	if output, err := runCommand("systemctl", "restart", serviceUnitName); err != nil {
		return fmt.Errorf("restart %s after UDP-buffer rollback: %w: %s", serviceUnitName, err, strings.TrimSpace(output))
	}
	if err := waitForManagedServiceState(state, 15*time.Second); err != nil {
		return err
	}
	if err := os.Remove(hysteria2UDPBufferStatePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	committed = true
	_ = appendAudit(map[string]any{"command": "hysteria2 udp-buffer rollback", "status": "COMPLETED", "profile": ownership.Profile})
	return printJSON(commandResult{Command: "hysteria2 udp-buffer rollback", Status: "PASS", Detail: map[string]any{
		"result": "ROLLED_BACK", "restored_sysctls": ownership.Previous, "client_update_required": false,
	}})
}

func collectHysteria2UDPBufferStatus(state model.State) (map[string]any, error) {
	current, err := readHysteria2UDPBufferSysctls()
	if err != nil {
		return nil, err
	}
	detail := map[string]any{
		"sysctl_path":     hysteria2UDPBufferSysctlPath,
		"runtime_sysctls": current,
		"service_state":   systemdUnitState(serviceUnitName),
		"udp_port":        state.Hysteria2.ListenPort,
	}
	if buffers, err := currentHysteria2UDPSocketBuffers(state.Hysteria2.ListenPort); err == nil {
		detail["socket_buffers"] = buffers
	} else {
		detail["socket_buffers"] = map[string]any{"status": "UNAVAILABLE", "reason": err.Error()}
	}
	ownership, ownerErr := readHysteria2UDPBufferOwnership()
	switch {
	case ownerErr == nil:
		detail["managed"] = hysteria2UDPBufferConfigMatchesOwnership(ownership)
		detail["ownership"] = ownership
	case errors.Is(ownerErr, os.ErrNotExist):
		_, fileErr := os.Lstat(hysteria2UDPBufferSysctlPath)
		detail["managed"] = false
		detail["unmanaged_file_present"] = fileErr == nil
	default:
		return nil, ownerErr
	}
	return detail, nil
}

func hysteria2UDPBufferProfile(profile string) (map[string]int64, error) {
	if profile != hysteria2UDPBufferProfileConservative2MiB {
		return nil, fmt.Errorf("unsupported UDP-buffer profile %q; supported: %s", profile, hysteria2UDPBufferProfileConservative2MiB)
	}
	return map[string]int64{
		"net.core.rmem_default": hysteria2UDPBufferBytes2MiB,
		"net.core.wmem_default": hysteria2UDPBufferBytes2MiB,
		"net.core.rmem_max":     hysteria2UDPBufferBytes2MiB,
		"net.core.wmem_max":     hysteria2UDPBufferBytes2MiB,
	}, nil
}

func renderHysteria2UDPBufferSysctl(values map[string]int64) string {
	var builder strings.Builder
	builder.WriteString("# Managed by VPSKit. Use 'vpskit hysteria2 udp-buffer rollback --yes' before editing.\n")
	for _, name := range hysteria2UDPBufferSysctls {
		builder.WriteString(name)
		builder.WriteString(" = ")
		builder.WriteString(strconv.FormatInt(values[name], 10))
		builder.WriteByte('\n')
	}
	return builder.String()
}

func readHysteria2UDPBufferSysctls() (map[string]int64, error) {
	values := make(map[string]int64, len(hysteria2UDPBufferSysctls))
	for _, name := range hysteria2UDPBufferSysctls {
		output, err := runCommand("sysctl", "-n", name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		value, err := strconv.ParseInt(strings.TrimSpace(output), 10, 64)
		if err != nil || value < 1 {
			return nil, fmt.Errorf("read %s: invalid value", name)
		}
		values[name] = value
	}
	return values, nil
}

func applyHysteria2UDPBufferSysctls(values map[string]int64) error {
	for _, name := range hysteria2UDPBufferSysctls {
		value := values[name]
		if value < 1 {
			return fmt.Errorf("invalid managed sysctl value for %s", name)
		}
		output, err := runCommand("sysctl", "-w", name+"="+strconv.FormatInt(value, 10))
		if err != nil {
			return fmt.Errorf("apply %s: %w: %s", name, err, strings.TrimSpace(output))
		}
	}
	return nil
}

func currentHysteria2UDPSocketBuffers(port int) (hysteria2UDPSocketBuffers, error) {
	output, err := runCommand("ss", "-u", "-a", "-m", "-n")
	if err != nil {
		return hysteria2UDPSocketBuffers{}, err
	}
	return parseHysteria2UDPSocketBuffers(output, port)
}

func waitForHysteria2UDPSocketBuffers(port int, minimum int64, timeout time.Duration) (hysteria2UDPSocketBuffers, error) {
	deadline := time.Now().Add(timeout)
	var last hysteria2UDPSocketBuffers
	var lastErr error
	for time.Now().Before(deadline) {
		if systemdUnitState(serviceUnitName) == "active" {
			buffers, err := currentHysteria2UDPSocketBuffers(port)
			if err == nil && buffers.ReceiveBytes >= minimum && buffers.SendBytes >= minimum {
				return buffers, nil
			}
			last, lastErr = buffers, err
		}
		time.Sleep(500 * time.Millisecond)
	}
	if lastErr != nil {
		return hysteria2UDPSocketBuffers{}, fmt.Errorf("Hysteria2 UDP listener did not expose required socket buffers: %w", lastErr)
	}
	return hysteria2UDPSocketBuffers{}, fmt.Errorf("Hysteria2 UDP listener buffers below %d bytes: receive=%d send=%d", minimum, last.ReceiveBytes, last.SendBytes)
}

var hysteria2SocketBufferPattern = regexp.MustCompile(`(?:^|,)rb([0-9]+),.*(?:^|,)tb([0-9]+)(?:,|$)`)

func parseHysteria2UDPSocketBuffers(output string, port int) (hysteria2UDPSocketBuffers, error) {
	lines := strings.Split(output, "\n")
	for index, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 5 || !hysteria2UDPListenerEndpointMatchesPort(fields[3], port) {
			continue
		}
		for next := index + 1; next < len(lines) && next <= index+2; next++ {
			candidate := strings.TrimSpace(lines[next])
			if !strings.HasPrefix(candidate, "skmem:(") {
				continue
			}
			match := hysteria2SocketBufferPattern.FindStringSubmatch(candidate)
			if len(match) != 3 {
				return hysteria2UDPSocketBuffers{}, errors.New("UDP listener skmem output has no receive/send buffer values")
			}
			receive, receiveErr := strconv.ParseInt(match[1], 10, 64)
			send, sendErr := strconv.ParseInt(match[2], 10, 64)
			if receiveErr != nil || sendErr != nil || receive < 1 || send < 1 {
				return hysteria2UDPSocketBuffers{}, errors.New("UDP listener skmem output has invalid buffer values")
			}
			return hysteria2UDPSocketBuffers{ReceiveBytes: receive, SendBytes: send}, nil
		}
		return hysteria2UDPSocketBuffers{}, errors.New("UDP listener has no skmem output")
	}
	return hysteria2UDPSocketBuffers{}, fmt.Errorf("UDP listener on port %d not found", port)
}

func hysteria2UDPListenerEndpointMatchesPort(endpoint string, port int) bool {
	needle := ":" + strconv.Itoa(port)
	return strings.HasSuffix(strings.TrimSuffix(strings.TrimSpace(endpoint), "]"), needle)
}

func readHysteria2UDPBufferOwnership() (hysteria2UDPBufferOwnership, error) {
	contents, err := os.ReadFile(hysteria2UDPBufferStatePath)
	if err != nil {
		return hysteria2UDPBufferOwnership{}, err
	}
	var value hysteria2UDPBufferOwnership
	if err := json.Unmarshal(contents, &value); err != nil {
		return hysteria2UDPBufferOwnership{}, err
	}
	if value.SchemaVersion != hysteria2UDPBufferOwnershipSchema || value.SysctlPath != hysteria2UDPBufferSysctlPath || value.ConfigSHA256 == "" || value.AppliedAt.IsZero() {
		return hysteria2UDPBufferOwnership{}, errors.New("invalid Hysteria2 UDP-buffer ownership record")
	}
	if _, err := hysteria2UDPBufferProfile(value.Profile); err != nil || len(value.Previous) != len(hysteria2UDPBufferSysctls) {
		return hysteria2UDPBufferOwnership{}, errors.New("invalid Hysteria2 UDP-buffer ownership values")
	}
	for _, name := range hysteria2UDPBufferSysctls {
		if value.Previous[name] < 1 {
			return hysteria2UDPBufferOwnership{}, errors.New("invalid Hysteria2 UDP-buffer rollback value")
		}
	}
	return value, nil
}

func writeHysteria2UDPBufferOwnership(value hysteria2UDPBufferOwnership) error {
	if err := os.MkdirAll(hysteria2UDPBufferStateRoot, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(hysteria2UDPBufferStateRoot, 0o700); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(hysteria2UDPBufferStatePath, append(contents, '\n'), 0o600)
}

func hysteria2UDPBufferConfigMatchesOwnership(value hysteria2UDPBufferOwnership) bool {
	contents, err := os.ReadFile(hysteria2UDPBufferSysctlPath)
	return err == nil && strings.HasPrefix(string(contents), "# Managed by VPSKit.") && strings.EqualFold(sha256Bytes(contents), value.ConfigSHA256)
}

func removeManagedHysteria2UDPBufferForUninstall() error {
	ownership, err := readHysteria2UDPBufferOwnership()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read managed Hysteria2 UDP-buffer ownership during uninstall: %w", err)
	}
	if !hysteria2UDPBufferConfigMatchesOwnership(ownership) {
		return errors.New("refusing to remove modified VPSKit Hysteria2 UDP-buffer sysctl file during uninstall")
	}
	if err := applyHysteria2UDPBufferSysctls(ownership.Previous); err != nil {
		return fmt.Errorf("restore Hysteria2 UDP-buffer sysctls during uninstall: %w", err)
	}
	if err := os.Remove(hysteria2UDPBufferSysctlPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
