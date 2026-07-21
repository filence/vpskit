package app

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func runSystem(arguments []string) error {
	if len(arguments) != 1 || arguments[0] != "inspect" {
		return errors.New("usage: vpskit system inspect")
	}
	report, err := collectSystemInspect()
	if err != nil {
		return err
	}
	return printJSON(commandResult{Command: "system inspect", Status: "PASS", Detail: report})
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
	}, nil
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
