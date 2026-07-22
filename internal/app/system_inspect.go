package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func runSystem(arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("usage: vpskit system <inspect|updates>")
	}
	switch arguments[0] {
	case "inspect":
		report, err := collectSystemInspect()
		if err != nil {
			return err
		}
		return printJSON(commandResult{Command: "system inspect", Status: "PASS", Detail: report})
	case "updates":
		report, err := collectSystemUpdateCandidates()
		if err != nil {
			return err
		}
		return printJSON(commandResult{Command: "system updates", Status: "PASS", Detail: report})
	default:
		return errors.New("usage: vpskit system <inspect|updates>")
	}
}

func collectSystemUpdateCandidates() (map[string]any, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("system updates is only supported on Linux, got %s", runtime.GOOS)
	}
	output, err := runCommand("apt-get", "-s", "upgrade")
	if err != nil {
		return nil, fmt.Errorf("simulate system package upgrade: %w: %s", err, sanitizeText(output, ""))
	}
	packages := parseAPTUpgradeCandidates(output)
	return map[string]any{
		"checked_at":      time.Now().UTC(),
		"mode":            "simulation_only",
		"candidate_count": len(packages),
		"candidates":      packages,
		"reboot_required": fileExists("/var/run/reboot-required"),
		"note":            "No package was installed, upgraded, removed, or rebooted.",
	}, nil
}

func parseAPTUpgradeCandidates(output string) []string {
	packages := make([]string, 0)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "Inst" {
			packages = append(packages, fields[1])
		}
	}
	return packages
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func collectSystemInspect() (map[string]any, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("system inspect is only supported on Linux, got %s", runtime.GOOS)
	}
	osRelease, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return nil, fmt.Errorf("read operating system metadata: %w", err)
	}
	memory := map[string]int64{}
	if content, err := os.ReadFile("/proc/meminfo"); err == nil {
		memory = parseMemInfo(string(content))
	}
	uptimeSeconds := int64(0)
	if content, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(content))
		if len(fields) > 0 {
			if seconds, parseErr := strconv.ParseFloat(fields[0], 64); parseErr == nil {
				uptimeSeconds = int64(seconds)
			}
		}
	}
	kernel, _ := runCommand("uname", "-r")
	disk := inspectRootDisk()
	congestion, _ := runCommand("sysctl", "-n", "net.ipv4.tcp_congestion_control")
	defaultQdisc, _ := runCommand("sysctl", "-n", "net.core.default_qdisc")
	timeSyncOutput, timeSyncErr := runCommand("timedatectl", "show", "--property=NTPSynchronized", "--value")
	timeSync := timeSyncErr == nil && strings.EqualFold(strings.TrimSpace(timeSyncOutput), "yes")
	rebootRequired := false
	if _, err := os.Stat("/var/run/reboot-required"); err == nil {
		rebootRequired = true
	}
	services := map[string]string{
		"xray":       systemdUnitState(xrayServiceUnitName),
		"sing-box":   systemdUnitState(serviceUnitName),
		"cert-timer": systemdUnitState(certificateRenewTimerUnitName),
	}
	listeners := map[string]any{}
	if state, err := readInstalledState(); err == nil {
		if state.Reality.Enabled {
			listeners["reality_tcp"] = map[string]any{"port": state.Reality.ListenPort, "listening": listenerPresent("tcp", state.Reality.ListenPort)}
		}
		if state.Hysteria2.Enabled {
			listeners["hysteria2_udp"] = map[string]any{"port": state.Hysteria2.ListenPort, "listening": listenerPresent("udp", state.Hysteria2.ListenPort)}
		}
	}
	network := collectNetworkInspect()
	return map[string]any{
		"inspected_at":       time.Now().UTC(),
		"os":                 extractOSPrettyName(string(osRelease)),
		"architecture":       runtime.GOARCH,
		"kernel":             strings.TrimSpace(kernel),
		"uptime_seconds":     uptimeSeconds,
		"memory_kib":         memory,
		"root_disk":          disk,
		"congestion_control": strings.TrimSpace(congestion),
		"default_qdisc":      strings.TrimSpace(defaultQdisc),
		"time_synchronized":  timeSync,
		"reboot_required":    rebootRequired,
		"services":           services,
		"listeners":          listeners,
		"network":            network,
	}, nil
}

func collectNetworkInspect() map[string]any {
	return map[string]any{
		"ipv4": inspectIPFamily("-4"),
		"ipv6": inspectIPFamily("-6"),
		"dns":  inspectDNS(),
	}
}

func inspectIPFamily(family string) map[string]any {
	addressesOutput, addressesErr := runCommand("ip", "-o", family, "addr", "show", "scope", "global")
	routeOutput, routeErr := runCommand("ip", family, "route", "show", "default")
	addresses := parseGlobalIPAddresses(addressesOutput)
	return map[string]any{
		"configured":       len(addresses) > 0,
		"global_addresses": addresses,
		"default_route":    strings.TrimSpace(routeOutput),
		"route_available":  routeErr == nil && strings.TrimSpace(routeOutput) != "",
		"inspect_error":    firstNonEmptyError(addressesErr, routeErr),
	}
}

func inspectDNS() map[string]any {
	resolvConf, readErr := os.ReadFile("/etc/resolv.conf")
	servers := parseResolvConfNameservers(string(resolvConf))
	result := map[string]any{
		"servers": servers,
	}
	if readErr != nil {
		result["inspect_error"] = readErr.Error()
		return result
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, "one.one.one.one")
	if err != nil {
		result["resolution_status"] = "FAIL"
		result["resolution_error"] = sanitizeText(err.Error(), "")
		return result
	}
	values := make([]string, 0, len(addresses))
	for _, address := range addresses {
		values = append(values, address.IP.String())
	}
	result["resolution_status"] = "PASS"
	result["resolved_addresses"] = values
	return result
}

func parseGlobalIPAddresses(output string) []string {
	values := make([]string, 0)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		for index, field := range fields {
			if (field == "inet" || field == "inet6") && index+1 < len(fields) {
				values = append(values, fields[index+1])
				break
			}
		}
	}
	return values
}

func parseResolvConfNameservers(content string) []string {
	values := make([]string, 0)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "nameserver" {
			values = append(values, fields[1])
		}
	}
	return values
}

func firstNonEmptyError(values ...error) string {
	for _, value := range values {
		if value != nil {
			return value.Error()
		}
	}
	return ""
}

func parseMemInfo(content string) map[string]int64 {
	result := map[string]int64{}
	allowed := map[string]string{"MemTotal": "total", "MemAvailable": "available", "SwapTotal": "swap_total", "SwapFree": "swap_free"}
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		outputKey, ok := allowed[key]
		if !ok {
			continue
		}
		value, err := strconv.ParseInt(fields[1], 10, 64)
		if err == nil {
			result[outputKey] = value
		}
	}
	return result
}

func inspectRootDisk() map[string]any {
	output, err := runCommand("df", "-Pk", "/")
	if err != nil {
		return map[string]any{"status": "UNAVAILABLE"}
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return map[string]any{"status": "UNAVAILABLE"}
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 6 {
		return map[string]any{"status": "UNAVAILABLE"}
	}
	return map[string]any{
		"status":        "AVAILABLE",
		"total_kib":     parseInt64(fields[1]),
		"used_kib":      parseInt64(fields[2]),
		"available_kib": parseInt64(fields[3]),
		"used_percent":  fields[4],
	}
}

func parseInt64(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

func systemdUnitState(unit string) string {
	output, err := runCommand("systemctl", "is-active", unit)
	state := strings.TrimSpace(output)
	if state == "" && err != nil {
		return "unknown"
	}
	return state
}
