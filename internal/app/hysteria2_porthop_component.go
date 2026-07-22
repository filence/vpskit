package app

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"vpskit.local/vpskit/internal/fsutil"
	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/platform"
)

// prepare installs an inactive, dedicated nftables unit. It neither enables
// the unit nor changes any firewall rule, state, client export or listener.
func runHysteria2PortHopComponent(arguments []string) error {
	if arguments[0] == "status" {
		if len(arguments) != 1 {
			return errors.New("usage: vpskit hysteria2 port-hop status")
		}
		return printJSON(commandResult{Command: "hysteria2 port-hop status", Status: "PASS", Detail: map[string]any{"unit_state": systemdUnitState(hysteria2PortHopUnitName), "nft_config_present": portHopFileExists(hysteria2PortHopNftPath), "unit_present": portHopFileExists(hysteria2PortHopUnitPath)}})
	}
	if arguments[0] == "activate" || arguments[0] == "deactivate" {
		operation := arguments[0]
		if len(arguments) != 2 || arguments[1] != "--yes" {
			return fmt.Errorf("usage: vpskit hysteria2 port-hop %s --yes", operation)
		}
		if !platform.IsRoot() {
			return fmt.Errorf("hysteria2 port-hop %s requires root privileges", operation)
		}
		if !portHopFileExists(hysteria2PortHopNftPath) || !portHopFileExists(hysteria2PortHopUnitPath) {
			return errors.New("port-hop component is not prepared")
		}
		if operation == "activate" {
			if output, err := runCommand("nft", "-c", "-f", hysteria2PortHopNftPath); err != nil {
				return fmt.Errorf("validate managed nft rules: %w: %s", err, output)
			}
			if output, err := runCommand("systemctl", "enable", "--now", hysteria2PortHopUnitName); err != nil {
				return fmt.Errorf("activate port-hop redirect: %w: %s", err, output)
			}
		} else if output, err := runCommand("systemctl", "disable", "--now", hysteria2PortHopUnitName); err != nil {
			return fmt.Errorf("deactivate port-hop redirect: %w: %s", err, output)
		}
		if state := systemdUnitState(hysteria2PortHopUnitName); (operation == "activate" && state != "active") || (operation == "deactivate" && state == "active") {
			return fmt.Errorf("unexpected port-hop unit state after %s: %s", operation, state)
		}
		return printJSON(commandResult{Command: "hysteria2 port-hop " + operation, Status: "PASS", Detail: map[string]any{"redirect_active": operation == "activate", "client_export_changed": false, "manual_client_validation_required": false}})
	}
	if arguments[0] == "enable" {
		return enableHysteria2PortHop(arguments[1:])
	}
	if arguments[0] == "disable" {
		return disableHysteria2PortHop(arguments[1:])
	}
	flags := flag.NewFlagSet("hysteria2 port-hop prepare", flag.ContinueOnError)
	rawRange := flags.String("range", "", "UDP port range")
	backend := flags.Int("backend-port", 443, "managed Hysteria2 listener")
	yes := flags.Bool("yes", false, "write inactive managed redirect component")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || !*yes {
		return errors.New("usage: vpskit hysteria2 port-hop prepare --range <start-end> --backend-port <port> --yes")
	}
	start, end, err := parseHysteria2PortRange(*rawRange)
	if err != nil {
		return err
	}
	if *backend < 1 || *backend > 65535 || (*backend >= start && *backend <= end) {
		return errors.New("backend-port must be valid and outside the hopping range")
	}
	if !platform.IsRoot() {
		return errors.New("hysteria2 port-hop prepare requires root privileges")
	}
	if _, err := exec.LookPath("nft"); err != nil {
		return errors.New("nft command is required before preparing port hopping")
	}
	contents := renderHysteria2PortHopNft(start, end, *backend)
	unit := "[Unit]\nDescription=VPSKit Hysteria2 port-hop redirect (managed)\nAfter=network-online.target\nWants=network-online.target\n\n[Service]\nType=oneshot\nRemainAfterExit=yes\nExecStart=/usr/sbin/nft -f " + hysteria2PortHopNftPath + "\nExecStop=/usr/sbin/nft delete table inet vpskit_hysteria2_port_hop\n\n[Install]\nWantedBy=multi-user.target\n"
	if err := fsutil.WriteFileAtomic(hysteria2PortHopNftPath, []byte(contents), 0o600); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(hysteria2PortHopUnitPath, []byte(unit), 0o644); err != nil {
		return err
	}
	if out, err := runCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("systemd daemon-reload: %w: %s", err, out)
	}
	return printJSON(commandResult{Command: "hysteria2 port-hop prepare", Status: "PASS", Detail: map[string]any{"prepared": true, "enabled": false, "range": *rawRange, "backend_port": *backend, "next_step": "open the matching cloud UDP range, then explicitly enable and validate the managed component"}})
}

func portHopFileExists(path string) bool { _, err := os.Stat(path); return err == nil }

func renderHysteria2PortHopNft(start, end, backendPort int) string {
	return fmt.Sprintf(`table inet vpskit_hysteria2_port_hop {
  chain prerouting {
    type nat hook prerouting priority dstnat; policy accept;
    udp dport %d-%d redirect to :%d comment "VPSKit managed Hysteria2 port hop"
  }
}
`, start, end, backendPort)
}

func enableHysteria2PortHop(arguments []string) error {
	flags := flag.NewFlagSet("hysteria2 port-hop enable", flag.ContinueOnError)
	rawRange := flags.String("range", "", "UDP port range")
	hopInterval := flags.Int("hop-interval", 30, "client hopping interval in seconds")
	yes := flags.Bool("yes", false, "publish Hysteria2 port hopping to clients")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || !*yes {
		return errors.New("usage: vpskit hysteria2 port-hop enable --range <start-end> [--hop-interval <seconds>] --yes")
	}
	start, end, err := parseHysteria2PortRange(*rawRange)
	if err != nil {
		return err
	}
	if *hopInterval < 5 || *hopInterval > 3600 {
		return errors.New("hop-interval must be from 5 to 3600 seconds")
	}
	if !platform.IsRoot() {
		return errors.New("hysteria2 port-hop enable requires root privileges")
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	if !state.Hysteria2.Enabled || state.Hysteria2.ID == "" {
		return errors.New("Hysteria2 must be enabled before enabling port hopping")
	}
	if start <= state.Hysteria2.ListenPort && state.Hysteria2.ListenPort <= end {
		return fmt.Errorf("UDP hopping range %d-%d must not include the managed Hysteria2 listener %d", start, end, state.Hysteria2.ListenPort)
	}
	if state.Hysteria2.PortHopping.Enabled {
		return errors.New("Hysteria2 port hopping is already enabled")
	}
	if systemdUnitState(hysteria2PortHopUnitName) != "active" {
		return errors.New("managed port-hop redirect is not active; run prepare and activate before enabling client exports")
	}
	contents, err := os.ReadFile(hysteria2PortHopNftPath)
	if err != nil {
		return fmt.Errorf("read managed port-hop nft configuration: %w", err)
	}
	if string(contents) != renderHysteria2PortHopNft(start, end, state.Hysteria2.ListenPort) {
		return errors.New("managed port-hop redirect does not match the requested range and backend port")
	}
	if output, err := runCommand("nft", "list", "table", "inet", "vpskit_hysteria2_port_hop"); err != nil {
		return fmt.Errorf("verify active managed nft table: %w: %s", err, output)
	}
	secrets, err := readInstalledSecrets()
	if err != nil {
		return err
	}
	updated := state
	updated.Hysteria2.PortHopping = model.Hysteria2PortHoppingState{Enabled: true, RangeStart: start, RangeEnd: end, HopInterval: *hopInterval}
	if state.ConfigRevision < 1 {
		state.ConfigRevision = 1
	}
	updated.ConfigRevision = state.ConfigRevision + 1
	commit, err := commitManagedStateChange(state, updated, secrets, "hysteria2 port-hop enable", "hysteria2-port-hop", []string{"hysteria2.port_hopping", "client.hysteria2"})
	if err != nil {
		return err
	}
	return printJSON(commandResult{Command: "hysteria2 port-hop enable", Status: "PASS", Detail: map[string]any{
		"result":                            "ENABLED",
		"range":                             *rawRange,
		"hop_interval_seconds":              *hopInterval,
		"config_revision":                   commit.State.ConfigRevision,
		"transaction_id":                    commit.TransactionID,
		"previous_backup_id":                commit.BackupID,
		"service_restart":                   false,
		"client_update_required":            true,
		"manual_client_validation_required": true,
		"subscription_publish":              commit.SubscriptionPublish,
	}})
}

func disableHysteria2PortHop(arguments []string) error {
	if len(arguments) != 1 || arguments[0] != "--yes" {
		return errors.New("usage: vpskit hysteria2 port-hop disable --yes")
	}
	if !platform.IsRoot() {
		return errors.New("hysteria2 port-hop disable requires root privileges")
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	if !state.Hysteria2.PortHopping.Enabled {
		return errors.New("Hysteria2 port hopping is already disabled")
	}
	secrets, err := readInstalledSecrets()
	if err != nil {
		return err
	}
	updated := state
	updated.Hysteria2.PortHopping = model.Hysteria2PortHoppingState{}
	if state.ConfigRevision < 1 {
		state.ConfigRevision = 1
	}
	updated.ConfigRevision = state.ConfigRevision + 1
	commit, err := commitManagedStateChange(state, updated, secrets, "hysteria2 port-hop disable", "hysteria2-port-hop", []string{"hysteria2.port_hopping", "client.hysteria2"})
	if err != nil {
		return err
	}
	return printJSON(commandResult{Command: "hysteria2 port-hop disable", Status: "PASS", Detail: map[string]any{
		"result":                  "DISABLED",
		"config_revision":         commit.State.ConfigRevision,
		"transaction_id":          commit.TransactionID,
		"previous_backup_id":      commit.BackupID,
		"service_restart":         false,
		"client_update_required":  true,
		"redirect_remains_active": systemdUnitState(hysteria2PortHopUnitName) == "active",
		"next_step":               "refresh all clients before optionally running the separate port-hop deactivate command",
		"subscription_publish":    commit.SubscriptionPublish,
	}})
}
