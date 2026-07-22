package app

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/platform"
)

func runHysteria2(arguments []string) error {
	if len(arguments) != 1 || arguments[0] != "inspect" {
		return errors.New("usage: vpskit hysteria2 inspect")
	}
	if !platform.IsRoot() {
		return errors.New("hysteria2 inspection requires root privileges")
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	return printJSON(commandResult{Command: "hysteria2 inspect", Status: "PASS", Detail: collectHysteria2Inspect(state)})
}

func collectHysteria2Inspect(state model.State) map[string]any {
	coreVersion := strings.TrimSpace(state.Core.Version)
	return map[string]any{
		"inspected_at": time.Now().UTC(),
		"runtime": map[string]any{
			"enabled":          state.Hysteria2.Enabled,
			"service_state":    systemdUnitState(serviceUnitName),
			"udp_port":         state.Hysteria2.ListenPort,
			"listener_present": state.Hysteria2.Enabled && listenerPresent("udp", state.Hysteria2.ListenPort),
		},
		"core": map[string]any{
			"name":    "sing-box",
			"version": coreVersion,
			"channel": state.Core.Channel,
		},
		"udp_buffer": map[string]any{
			"rmem_max": readHysteria2Sysctl("net.core.rmem_max"),
			"wmem_max": readHysteria2Sysctl("net.core.wmem_max"),
		},
		"capabilities": map[string]any{
			"salamander": map[string]any{
				"status":                     hysteria2CapabilityStatus(hysteria2VersionAtLeast(coreVersion, "1.13.0")),
				"server_minimum":             "1.13.0",
				"target_mihomo_supported":    true,
				"requires_manual_validation": true,
				"next_step":                  "experimental opt-in only; update, switch, and connect with the pinned Windows client before keeping it enabled",
			},
			"gecko": map[string]any{
				"status":         hysteria2CapabilityStatus(hysteria2VersionAtLeast(coreVersion, "1.14.0")),
				"server_minimum": "1.14.0",
				"reason":         "current VPSKit locked sing-box version must support Gecko before configuration can be generated",
			},
			"bbr_profile": map[string]any{
				"status":         hysteria2CapabilityStatus(hysteria2VersionAtLeast(coreVersion, "1.14.0")),
				"server_minimum": "1.14.0",
				"reason":         "do not publish client BBR profile fields until the locked sing-box inbound supports them",
			},
			"port_hopping": map[string]any{
				"status": "BLOCKED",
				"reason": "the current sing-box inbound does not own an UDP port range; implementation requires a dedicated managed redirect component, cloud security-group preparation, conflict checks, and rollback",
			},
		},
	}
}

func readHysteria2Sysctl(name string) map[string]any {
	output, err := runCommand("sysctl", "-n", name)
	if err != nil {
		return map[string]any{"status": "UNAVAILABLE"}
	}
	value, parseErr := strconv.ParseInt(strings.TrimSpace(output), 10, 64)
	if parseErr != nil || value < 0 {
		return map[string]any{"status": "UNAVAILABLE"}
	}
	return map[string]any{"status": "AVAILABLE", "bytes": value}
}

func hysteria2CapabilityStatus(supported bool) string {
	if supported {
		return "EXPERIMENTAL"
	}
	return "BLOCKED"
}

func hysteria2VersionAtLeast(actual, minimum string) bool {
	parse := func(value string) ([]int, bool) {
		parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(value), "v"), ".")
		if len(parts) < 2 || len(parts) > 3 {
			return nil, false
		}
		values := []int{0, 0, 0}
		for index, part := range parts {
			parsed, err := strconv.Atoi(part)
			if err != nil || parsed < 0 {
				return nil, false
			}
			values[index] = parsed
		}
		return values, true
	}
	actualValues, actualOK := parse(actual)
	minimumValues, minimumOK := parse(minimum)
	if !actualOK || !minimumOK {
		return false
	}
	for index := range actualValues {
		if actualValues[index] != minimumValues[index] {
			return actualValues[index] > minimumValues[index]
		}
	}
	return true
}
