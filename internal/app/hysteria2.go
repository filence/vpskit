package app

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/platform"
)

func runHysteria2(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: vpskit hysteria2 <inspect|salamander|performance|recommend|port-hop|udp-buffer>")
	}
	switch arguments[0] {
	case "inspect":
		if len(arguments) != 1 {
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
	case "salamander":
		return runHysteria2Salamander(arguments[1:])
	case "performance":
		return runHysteria2Performance(arguments[1:])
	case "recommend":
		return runHysteria2Recommend(arguments[1:])
	case "port-hop":
		return runHysteria2PortHop(arguments[1:])
	case "udp-buffer":
		return runHysteria2UDPBuffer(arguments[1:])
	default:
		return errors.New("usage: vpskit hysteria2 <inspect|salamander|performance|recommend|port-hop|udp-buffer>")
	}
}

// runHysteria2PortHop currently exposes only a read-only plan. sing-box owns
// one UDP listener, so a safe hopping implementation needs a separately owned
// redirect component and an explicitly prepared cloud security group.
func runHysteria2PortHop(arguments []string) error {
	if len(arguments) == 0 || arguments[0] != "plan" {
		return errors.New("usage: vpskit hysteria2 port-hop plan --range <start-end> [--hop-interval <seconds>]")
	}
	flags := flag.NewFlagSet("hysteria2 port-hop plan", flag.ContinueOnError)
	portRange := flags.String("range", "", "UDP port range, for example 20000-20010")
	hopInterval := flags.Int("hop-interval", 30, "client hopping interval in seconds")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *portRange == "" {
		return errors.New("usage: vpskit hysteria2 port-hop plan --range <start-end> [--hop-interval <seconds>]")
	}
	start, end, err := parseHysteria2PortRange(*portRange)
	if err != nil {
		return err
	}
	if *hopInterval < 5 || *hopInterval > 3600 {
		return errors.New("hop-interval must be from 5 to 3600 seconds")
	}
	if !platform.IsRoot() {
		return errors.New("hysteria2 port-hop plan requires root privileges")
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	if !state.Hysteria2.Enabled || state.Hysteria2.ID == "" {
		return errors.New("Hysteria2 must be enabled before planning port hopping")
	}
	if start <= state.Hysteria2.ListenPort && state.Hysteria2.ListenPort <= end {
		return fmt.Errorf("UDP hopping range %d-%d must not include the managed Hysteria2 listener %d", start, end, state.Hysteria2.ListenPort)
	}
	return printJSON(commandResult{Command: "hysteria2 port-hop plan", Status: "PASS", Detail: collectHysteria2PortHopPlan(state, start, end, *hopInterval)})
}

func parseHysteria2PortRange(value string) (int, int, error) {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) != 2 {
		return 0, 0, errors.New("port range must use start-end, for example 20000-20010")
	}
	start, startErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	end, endErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	if startErr != nil || endErr != nil || start < 1 || end > 65535 || end <= start {
		return 0, 0, errors.New("port range must contain two ascending ports from 1 to 65535")
	}
	if end-start+1 > 64 {
		return 0, 0, errors.New("port range must contain no more than 64 UDP ports")
	}
	return start, end, nil
}

func collectHysteria2PortHopPlan(state model.State, start, end, hopInterval int) map[string]any {
	conflicts := make([]int, 0)
	for port := start; port <= end; port++ {
		if listenerPresent("udp", port) {
			conflicts = append(conflicts, port)
		}
	}
	ready := len(conflicts) == 0 && state.Firewall.Provider != "manual/noop"
	return map[string]any{
		"read_only":            true,
		"apply_available":      false,
		"status":               "BLOCKED",
		"range":                fmt.Sprintf("%d-%d", start, end),
		"port_count":           end - start + 1,
		"hop_interval_seconds": hopInterval,
		"backend":              map[string]any{"listener_port": state.Hysteria2.ListenPort, "service": serviceUnitName, "redirect_required": true},
		"conflicts":            map[string]any{"udp_listener_ports": conflicts, "clear": len(conflicts) == 0},
		"firewall":             map[string]any{"local_provider": state.Firewall.Provider, "cloud_security_group_action": fmt.Sprintf("manually allow UDP %d-%d before any future apply", start, end), "cloud_confirmation_required": true},
		"implementation_gate":  map[string]any{"managed_redirect": "NOT_IMPLEMENTED", "rollback": "NOT_IMPLEMENTED", "client_export": "NOT_IMPLEMENTED", "manual_client_validation_required": true},
		"next_step":            map[bool]string{true: "all local preconditions are visible, but apply remains blocked until the dedicated redirect, rollback and client export are implemented", false: "resolve listed UDP listener or firewall-provider preconditions; do not open the cloud range yet"}[ready],
	}
}

func runHysteria2Performance(arguments []string) error {
	if len(arguments) != 1 || arguments[0] != "inspect" {
		return errors.New("usage: vpskit hysteria2 performance inspect")
	}
	if !platform.IsRoot() {
		return errors.New("hysteria2 performance inspection requires root privileges")
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	return printJSON(commandResult{Command: "hysteria2 performance inspect", Status: "PASS", Detail: collectHysteria2PerformanceInspect(state)})
}

// runHysteria2Recommend turns explicitly supplied test conditions into a
// conservative test plan. It deliberately never writes sing-box bandwidth or
// congestion fields: those settings must be justified by repeatable client
// measurements and the pinned core's feature support.
func runHysteria2Recommend(arguments []string) error {
	flags := flag.NewFlagSet("hysteria2 recommend", flag.ContinueOnError)
	serverMbps := flags.Float64("server-mbps", 0, "VPS provider bandwidth cap in Mbps")
	clientMbps := flags.Float64("client-mbps", 0, "available client-network bandwidth in Mbps")
	observedMbps := flags.Float64("observed-mbps", 0, "optional measured Hysteria2 throughput in Mbps")
	rttMS := flags.Float64("rtt-ms", 0, "optional measured RTT in milliseconds")
	lossPercent := flags.Float64("loss-percent", -1, "optional measured packet loss percent")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: vpskit hysteria2 recommend --server-mbps <number> --client-mbps <number> [--observed-mbps <number> --rtt-ms <number> --loss-percent <number>]")
	}
	if *serverMbps <= 0 || *clientMbps <= 0 || *observedMbps < 0 || *rttMS < 0 || *lossPercent < -1 || *lossPercent > 100 {
		return errors.New("server-mbps and client-mbps must be positive; observed-mbps and rtt-ms must not be negative; loss-percent must be from 0 to 100 when supplied")
	}
	if !platform.IsRoot() {
		return errors.New("hysteria2 recommendation requires root privileges to read the installed core version")
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	return printJSON(commandResult{Command: "hysteria2 recommend", Status: "PASS", Detail: collectHysteria2Recommendation(state, hysteria2RecommendationInput{
		ServerMbps: *serverMbps, ClientMbps: *clientMbps, ObservedMbps: *observedMbps, RTTMS: *rttMS, LossPercent: *lossPercent,
	})})
}

type hysteria2RecommendationInput struct {
	ServerMbps   float64
	ClientMbps   float64
	ObservedMbps float64
	RTTMS        float64
	LossPercent  float64
}

func collectHysteria2Recommendation(state model.State, input hysteria2RecommendationInput) map[string]any {
	bottleneckMbps := input.ServerMbps
	if input.ClientMbps < bottleneckMbps {
		bottleneckMbps = input.ClientMbps
	}
	// This is a measurement ceiling, not a server configuration value. Leaving
	// headroom avoids treating a variable consumer link as a guaranteed cap.
	testCeilingMbps := roundHysteria2Mbps(bottleneckMbps * 0.85)
	measurementComplete := input.ObservedMbps > 0 && input.RTTMS > 0 && input.LossPercent >= 0
	assessment := "MEASUREMENT_REQUIRED"
	nextStep := "在同一网络、同一节点下至少重复三次客户端测速；补齐 observed-mbps、rtt-ms 和 loss-percent 后再比较。"
	if measurementComplete {
		if input.ObservedMbps >= testCeilingMbps*0.9 && input.LossPercent <= 1 {
			assessment = "NO_SERVER_TUNING_RECOMMENDED"
			nextStep = "实测已接近保守测试上限，保持现有 Hysteria2 设置；不要仅为追求数字写入带宽或拥塞字段。"
		} else {
			assessment = "INVESTIGATE_CLIENT_PATH"
			nextStep = "实测低于保守测试上限，先分别复测客户端网络、UDP 可用性、CPU 占用和丢包；不要直接把带宽限制或 BBR profile 写入服务端。"
		}
	}
	coreVersion := strings.TrimSpace(state.Core.Version)
	return map[string]any{
		"read_only":       true,
		"service_restart": false,
		"config_revision": state.ConfigRevision,
		"input": map[string]any{
			"server_mbps": input.ServerMbps, "client_mbps": input.ClientMbps,
			"observed_mbps": input.ObservedMbps, "rtt_ms": input.RTTMS, "loss_percent": input.LossPercent,
		},
		"test_plan": map[string]any{
			"bottleneck_mbps": bottleneckMbps, "conservative_test_ceiling_mbps": testCeilingMbps,
			"method": "min(server_mbps, client_mbps) * 0.85; this is a repeatable test target, not a sing-box bandwidth setting",
		},
		"assessment": assessment,
		"next_step":  nextStep,
		"server_config": map[string]any{
			"up_down_mbps": "KEEP_UNSET", "ignore_client_bandwidth": "KEEP_UNSET",
			"reason": "current evidence does not justify overriding Hysteria2 client bandwidth negotiation",
		},
		"feature_gate": map[string]any{
			"sing_box_version": coreVersion,
			"bbr_profile":      map[string]any{"status": hysteria2CapabilityStatus(hysteria2VersionAtLeast(coreVersion, "1.14.0")), "minimum": "1.14.0", "action": "DO_NOT_CONFIGURE_ON_CURRENT_LOCKED_CORE"},
		},
	}
}

func roundHysteria2Mbps(value float64) float64 {
	return float64(int(value*10+0.5)) / 10
}

func runHysteria2Salamander(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: vpskit hysteria2 salamander <plan|enable|disable> [--yes]")
	}
	operation := strings.ToLower(strings.TrimSpace(arguments[0]))
	flags := flag.NewFlagSet("hysteria2 salamander "+operation, flag.ContinueOnError)
	yes := flags.Bool("yes", false, "confirm the managed Hysteria2 configuration change")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if operation != "plan" && operation != "enable" && operation != "disable" {
		return fmt.Errorf("unsupported Salamander operation %q", operation)
	}
	if operation == "plan" && *yes {
		return errors.New("hysteria2 salamander plan does not accept --yes")
	}
	if (operation == "enable" || operation == "disable") && !*yes {
		return fmt.Errorf("hysteria2 salamander %s requires explicit --yes confirmation", operation)
	}
	if !platform.IsRoot() {
		return errors.New("hysteria2 Salamander management requires root privileges")
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	if !state.Hysteria2.Enabled || state.Hysteria2.ID == "" {
		return errors.New("Hysteria2 must be enabled before managing Salamander")
	}
	if !hysteria2VersionAtLeast(state.Core.Version, "1.13.0") {
		return fmt.Errorf("Salamander requires sing-box 1.13.0 or newer; current locked version is %q", state.Core.Version)
	}
	if operation == "plan" {
		return printJSON(commandResult{Command: "hysteria2 salamander plan", Status: "PASS", Detail: map[string]any{
			"enabled":                    state.Hysteria2.Obfuscation == "salamander",
			"current_obfuscation":        state.Hysteria2.Obfuscation,
			"server_version":             state.Core.Version,
			"server_minimum":             "1.13.0",
			"target_client":              "Clash Verge Rev 2.5.2 / Mihomo 1.19.29",
			"client_update_required":     true,
			"manual_validation_required": true,
			"enable_command":             "vpskit hysteria2 salamander enable --yes",
			"disable_command":            "vpskit hysteria2 salamander disable --yes",
		}})
	}
	return mutateHysteria2Salamander(state, operation)
}

func mutateHysteria2Salamander(state model.State, operation string) error {
	secrets, err := readInstalledSecrets()
	if err != nil {
		return err
	}
	updated := state
	switch operation {
	case "enable":
		if updated.Hysteria2.Obfuscation == "salamander" {
			return errors.New("Hysteria2 Salamander is already enabled")
		}
		if updated.Hysteria2.Obfuscation != "" {
			return fmt.Errorf("cannot replace unsupported existing Hysteria2 obfuscation %q", updated.Hysteria2.Obfuscation)
		}
		updated.Hysteria2.Obfuscation = "salamander"
		updated.Hysteria2.ObfuscationPasswordRef = "secret://hy2-backup/obfuscation-password"
		secrets.Hysteria2ObfuscationPassword = randomBase64(24)
	case "disable":
		if updated.Hysteria2.Obfuscation == "" {
			return errors.New("Hysteria2 Salamander is already disabled")
		}
		if updated.Hysteria2.Obfuscation != "salamander" {
			return fmt.Errorf("refusing to disable unmanaged Hysteria2 obfuscation %q", updated.Hysteria2.Obfuscation)
		}
		updated.Hysteria2.Obfuscation = ""
		updated.Hysteria2.ObfuscationPasswordRef = ""
		secrets.Hysteria2ObfuscationPassword = ""
	default:
		return fmt.Errorf("unsupported Salamander operation %q", operation)
	}
	if state.ConfigRevision < 1 {
		state.ConfigRevision = 1
	}
	updated.ConfigRevision = state.ConfigRevision + 1
	commit, err := commitManagedStateChangeWithServiceRestart(state, updated, secrets, "hysteria2 salamander "+operation, "hysteria2-salamander", []string{"hysteria2.obfuscation", "hysteria2.obfuscation_password", "client.hysteria2"}, true)
	if err != nil {
		return err
	}
	return printJSON(commandResult{Command: "hysteria2 salamander " + operation, Status: "PASS", Detail: map[string]any{
		"result":                     map[string]string{"enable": "ENABLED", "disable": "DISABLED"}[operation],
		"config_revision":            commit.State.ConfigRevision,
		"transaction_id":             commit.TransactionID,
		"previous_backup_id":         commit.BackupID,
		"client_update_required":     true,
		"manual_validation_required": true,
		"subscription_publish":       commit.SubscriptionPublish,
	}})
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
		"obfuscation": map[string]any{
			"type":    state.Hysteria2.Obfuscation,
			"enabled": state.Hysteria2.Obfuscation != "",
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

// collectHysteria2PerformanceInspect deliberately does not claim that host
// process data measures the client-to-VPS path. RTT, throughput and loss must
// come from an authenticated client or a controlled remote probe.
func collectHysteria2PerformanceInspect(state model.State) map[string]any {
	return map[string]any{
		"inspected_at": time.Now().UTC(),
		"runtime": map[string]any{
			"enabled":          state.Hysteria2.Enabled,
			"service_state":    systemdUnitState(serviceUnitName),
			"udp_port":         state.Hysteria2.ListenPort,
			"listener_present": state.Hysteria2.Enabled && listenerPresent("udp", state.Hysteria2.ListenPort),
		},
		"process":           collectManagedServiceProcessUsage(serviceUnitName),
		"udp_buffer":        map[string]any{"rmem_max": readHysteria2Sysctl("net.core.rmem_max"), "wmem_max": readHysteria2Sysctl("net.core.wmem_max")},
		"kernel_congestion": readHysteria2TextSysctl("net.ipv4.tcp_congestion_control"),
		"client_path_measurement": map[string]any{
			"status":   "NOT_MEASURED",
			"reason":   "server-side inspection cannot measure authenticated client-to-VPS RTT, throughput, or packet loss",
			"required": []string{"rtt_ms", "throughput_mbps", "packet_loss_percent", "client_core_cpu_percent"},
		},
		"recommendation": "collect comparable client-side measurements before changing UDP buffers or congestion-related settings; do not infer throughput from a single latency result",
	}
}

func collectManagedServiceProcessUsage(unit string) map[string]any {
	output, err := runCommand("systemctl", "show", "--property=MainPID", "--value", unit)
	if err != nil {
		return map[string]any{"status": "UNAVAILABLE"}
	}
	pid, err := strconv.ParseInt(strings.TrimSpace(output), 10, 64)
	if err != nil || pid < 1 {
		return map[string]any{"status": "NOT_RUNNING"}
	}
	output, err = runCommand("ps", "-o", "pcpu=,rss=", "-p", strconv.FormatInt(pid, 10))
	if err != nil {
		return map[string]any{"status": "UNAVAILABLE", "pid": pid}
	}
	usage, err := parseHysteria2ProcessUsage(output)
	if err != nil {
		return map[string]any{"status": "UNAVAILABLE", "pid": pid}
	}
	return map[string]any{"status": "AVAILABLE", "pid": pid, "cpu_percent": usage.CPUPercent, "rss_kib": usage.RSSKiB}
}

type hysteria2ProcessUsage struct {
	CPUPercent float64
	RSSKiB     int64
}

func parseHysteria2ProcessUsage(output string) (hysteria2ProcessUsage, error) {
	fields := strings.Fields(output)
	if len(fields) != 2 {
		return hysteria2ProcessUsage{}, errors.New("unexpected ps output")
	}
	cpu, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || cpu < 0 {
		return hysteria2ProcessUsage{}, errors.New("invalid process CPU percentage")
	}
	rss, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || rss < 0 {
		return hysteria2ProcessUsage{}, errors.New("invalid process RSS")
	}
	return hysteria2ProcessUsage{CPUPercent: cpu, RSSKiB: rss}, nil
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

func readHysteria2TextSysctl(name string) map[string]any {
	output, err := runCommand("sysctl", "-n", name)
	if err != nil || strings.TrimSpace(output) == "" {
		return map[string]any{"status": "UNAVAILABLE"}
	}
	return map[string]any{"status": "AVAILABLE", "value": strings.TrimSpace(output)}
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
