package app

import (
	"strings"
	"testing"

	"vpskit.local/vpskit/internal/model"
)

func TestHysteria2VersionAtLeast(t *testing.T) {
	for _, test := range []struct {
		actual   string
		minimum  string
		supports bool
	}{
		{"1.13.14", "1.13.0", true},
		{"v1.14.0", "1.14.0", true},
		{"1.13.14", "1.14.0", false},
		{"1.14.0-alpha.1", "1.14.0", false},
		{"invalid", "1.14.0", false},
	} {
		if got := hysteria2VersionAtLeast(test.actual, test.minimum); got != test.supports {
			t.Fatalf("hysteria2VersionAtLeast(%q, %q) = %t, want %t", test.actual, test.minimum, got, test.supports)
		}
	}
}

func TestHysteria2SalamanderUsageRequiresExplicitConfirmation(t *testing.T) {
	err := runHysteria2Salamander([]string{"enable"})
	if err == nil || !strings.Contains(err.Error(), "requires explicit --yes") {
		t.Fatalf("unexpected Salamander enable confirmation error: %v", err)
	}
	if err := runHysteria2Salamander([]string{"plan", "--yes"}); err == nil || !strings.Contains(err.Error(), "does not accept --yes") {
		t.Fatalf("unexpected Salamander plan confirmation error: %v", err)
	}
}

func TestParseHysteria2ProcessUsage(t *testing.T) {
	usage, err := parseHysteria2ProcessUsage(" 1.5  20480\n")
	if err != nil {
		t.Fatal(err)
	}
	if usage.CPUPercent != 1.5 || usage.RSSKiB != 20480 {
		t.Fatalf("unexpected parsed usage: %#v", usage)
	}
	for _, output := range []string{"", "1.5", "-1 2", "1.5 -2", "1.5 nope"} {
		if _, err := parseHysteria2ProcessUsage(output); err == nil {
			t.Fatalf("expected parse error for %q", output)
		}
	}
}

func TestHysteria2RecommendationUsesInputsWithoutProposingWrites(t *testing.T) {
	state := model.State{Core: model.CoreState{Version: "1.13.14"}, ConfigRevision: 15}
	detail := collectHysteria2Recommendation(state, hysteria2RecommendationInput{ServerMbps: 500, ClientMbps: 300, ObservedMbps: 280, RTTMS: 50, LossPercent: 0.2})
	if detail["read_only"] != true || detail["service_restart"] != false || detail["assessment"] != "NO_SERVER_TUNING_RECOMMENDED" {
		t.Fatalf("unexpected recommendation safety result: %#v", detail)
	}
	plan := detail["test_plan"].(map[string]any)
	if plan["bottleneck_mbps"] != 300.0 || plan["conservative_test_ceiling_mbps"] != 255.0 {
		t.Fatalf("unexpected test plan: %#v", plan)
	}
	config := detail["server_config"].(map[string]any)
	if config["up_down_mbps"] != "KEEP_UNSET" || config["ignore_client_bandwidth"] != "KEEP_UNSET" {
		t.Fatalf("recommendation unexpectedly changes server configuration: %#v", config)
	}
	gate := detail["feature_gate"].(map[string]any)["bbr_profile"].(map[string]any)
	if gate["status"] != "BLOCKED" {
		t.Fatalf("expected BBR profile gate on 1.13.14: %#v", gate)
	}
}

func TestHysteria2RecommendationRequiresCompleteMeasurementForTuningAssessment(t *testing.T) {
	detail := collectHysteria2Recommendation(model.State{Core: model.CoreState{Version: "1.13.14"}}, hysteria2RecommendationInput{ServerMbps: 500, ClientMbps: 200})
	if detail["assessment"] != "MEASUREMENT_REQUIRED" {
		t.Fatalf("unexpected incomplete assessment: %#v", detail)
	}
}

func TestParseHysteria2PortRange(t *testing.T) {
	start, end, err := parseHysteria2PortRange("20000-20010")
	if err != nil || start != 20000 || end != 20010 {
		t.Fatalf("unexpected valid range: %d %d %v", start, end, err)
	}
	for _, value := range []string{"", "20000", "20010-20000", "0-2", "1-65535", "a-b"} {
		if _, _, err := parseHysteria2PortRange(value); err == nil {
			t.Fatalf("expected invalid range error for %q", value)
		}
	}
}

func TestRenderHysteria2PortHopNftUsesParseableRuleLayout(t *testing.T) {
	got := renderHysteria2PortHopNft(20000, 20010, 443)
	want := "table inet vpskit_hysteria2_port_hop {\n" +
		"  chain prerouting {\n" +
		"    type nat hook prerouting priority dstnat; policy accept;\n" +
		"    udp dport 20000-20010 redirect to :443 comment \"VPSKit managed Hysteria2 port hop\"\n" +
		"  }\n" +
		"}\n"
	if got != want {
		t.Fatalf("unexpected nft ruleset:\n%s", got)
	}
}

func TestHysteria2PortHopExportStateHasRequiredClientFields(t *testing.T) {
	state := model.State{Hysteria2: model.Hysteria2State{PortHopping: model.Hysteria2PortHoppingState{Enabled: true, RangeStart: 20000, RangeEnd: 20010, HopInterval: 30}}}
	if got := hysteria2PortRangeString(state.Hysteria2.PortHopping); got != "20000-20010" {
		t.Fatalf("unexpected client range %q", got)
	}
	if state.Hysteria2.PortHopping.HopInterval < 5 || state.Hysteria2.PortHopping.HopInterval > 3600 {
		t.Fatalf("test fixture must use the accepted hop-interval range")
	}
}

func TestHysteria2PortHopPlanRemainsBlockedWithoutImplementation(t *testing.T) {
	detail := collectHysteria2PortHopPlan(model.State{Hysteria2: model.Hysteria2State{ListenPort: 443}, Firewall: model.FirewallState{Provider: "manual/noop"}}, 20000, 20010, 30)
	if detail["read_only"] != true || detail["apply_available"] != false || detail["status"] != "BLOCKED" {
		t.Fatalf("unexpected plan state: %#v", detail)
	}
	gate := detail["implementation_gate"].(map[string]any)
	if gate["managed_redirect"] != "NOT_IMPLEMENTED" || gate["client_export"] != "NOT_IMPLEMENTED" {
		t.Fatalf("unexpected implementation gate: %#v", gate)
	}
}
