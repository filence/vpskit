package render

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"vpskit.local/vpskit/internal/model"
)

func testValues() model.RuntimeValues {
	return model.RuntimeValues{
		Node:              model.NodeMetadata{ID: "node-main", DisplayName: "Personal-JP-01", EnabledInSubscription: true},
		ClientRevision:    7,
		RealityEnabled:    true,
		Hysteria2Enabled:  true,
		ConnectHost:       "node.example.com",
		Domain:            "node.example.com",
		RealityServerName: "www.example.com",
		TCPPort:           443,
		UDPPort:           443,
		RealityUUID:       "11111111-2222-4333-8444-555555555555",
		RealityPrivateKey: "MPWnZh-8Lcud4TULcibJKQ9oovNzGoBzdS0Br6LQ-W0",
		RealityPublicKey:  "esdj56PLQGb8O3gFsAw-LanV7ZVgIiEFod2siUTCqiY",
		RealityShortID:    "aabbccddeeff0011",
		Hysteria2Password: "password",
		CertificatePath:   "/managed/cert.crt",
		KeyPath:           "/managed/cert.key",
	}
}

func TestServerConfigSupportsEachProfile(t *testing.T) {
	for _, test := range []struct {
		name             string
		realityEnabled   bool
		hysteria2Enabled bool
		wantInbounds     int
	}{
		{name: "balanced", realityEnabled: true, hysteria2Enabled: true, wantInbounds: 1},
		{name: "reality-only", realityEnabled: true, wantInbounds: 0},
		{name: "hysteria2-only", hysteria2Enabled: true, wantInbounds: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			values := testValues()
			values.RealityEnabled = test.realityEnabled
			values.Hysteria2Enabled = test.hysteria2Enabled
			configuration, err := ServerConfig(values)
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(configuration, &decoded); err != nil {
				t.Fatal(err)
			}
			if got := len(decoded["inbounds"].([]any)); got != test.wantInbounds {
				t.Fatalf("expected %d inbounds, got %d", test.wantInbounds, got)
			}
		})
	}
}

func TestServerConfigAllowsEmptyProfileForXrayOnly(t *testing.T) {
	values := testValues()
	values.RealityEnabled = false
	values.Hysteria2Enabled = false
	configuration, err := ServerConfig(values)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(configuration, &decoded); err != nil {
		t.Fatal(err)
	}
	if got := len(decoded["inbounds"].([]any)); got != 0 {
		t.Fatalf("expected no sing-box inbounds, got %d", got)
	}
}

func TestServerConfigIsJSON(t *testing.T) {
	configuration, err := ServerConfig(testValues())
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(configuration, &decoded); err != nil {
		t.Fatal(err)
	}
	inbounds, ok := decoded["inbounds"].([]any)
	if !ok || len(inbounds) != 1 {
		t.Fatalf("expected one Hysteria2 inbound, got %#v", decoded["inbounds"])
	}
}

func TestXrayRealityServerConfigIsJSON(t *testing.T) {
	configuration, err := XrayRealityServerConfig(testValues())
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(configuration, &decoded); err != nil {
		t.Fatal(err)
	}
	inbounds, ok := decoded["inbounds"].([]any)
	if !ok || len(inbounds) != 1 {
		t.Fatalf("expected one REALITY inbound, got %#v", decoded["inbounds"])
	}
}

func TestMihomoDoesNotDisableCertificateVerification(t *testing.T) {
	configuration, err := Mihomo(testValues())
	if err != nil {
		t.Fatal(err)
	}
	output := string(configuration)
	if strings.Contains(output, "skip-cert-verify: true") {
		t.Fatal("Mihomo output must not disable certificate verification")
	}
	if !strings.Contains(output, "skip-cert-verify: false") {
		t.Fatal("Mihomo output should state certificate verification explicitly")
	}
}

func TestShareLinksContainBothProtocols(t *testing.T) {
	output := string(ShareLinks(testValues()))
	if !strings.Contains(output, "vless://") || !strings.Contains(output, "hysteria2://") {
		t.Fatalf("missing expected share links: %s", output)
	}
}

func TestSalamanderRendersAcrossHysteria2Artifacts(t *testing.T) {
	values := testValues()
	values.Hysteria2Obfuscation = "salamander"
	values.Hysteria2ObfuscationPassword = "obfuscation-password"

	serverConfig, err := ServerConfig(values)
	if err != nil {
		t.Fatal(err)
	}
	var server map[string]any
	if err := json.Unmarshal(serverConfig, &server); err != nil {
		t.Fatal(err)
	}
	inbound := server["inbounds"].([]any)[0].(map[string]any)
	obfs := inbound["obfs"].(map[string]any)
	if obfs["type"] != "salamander" || obfs["password"] != values.Hysteria2ObfuscationPassword {
		t.Fatalf("server Salamander fields missing: %#v", obfs)
	}

	clientConfig, err := SingBoxHysteria2Client(values, 2081)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(clientConfig), "\"obfs\"") || !strings.Contains(string(clientConfig), "salamander") {
		t.Fatalf("sing-box client Salamander fields missing: %s", clientConfig)
	}
	mihomoConfig, err := Mihomo(values)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mihomoConfig), "obfs: salamander") || !strings.Contains(string(mihomoConfig), "obfs-password: obfuscation-password") {
		t.Fatalf("Mihomo Salamander fields missing: %s", mihomoConfig)
	}
	shareLink := string(ShareLinks(values))
	if !strings.Contains(shareLink, "obfs=salamander") || !strings.Contains(shareLink, "obfs-password=obfuscation-password") {
		t.Fatalf("share-link Salamander fields missing: %s", shareLink)
	}
}

func TestMihomoRejectsUnsupportedHysteria2Obfuscation(t *testing.T) {
	values := testValues()
	values.Hysteria2Obfuscation = "gecko"
	values.Hysteria2ObfuscationPassword = "password"
	if _, err := Mihomo(values); err == nil {
		t.Fatal("expected unsupported Hysteria2 obfuscation to be rejected")
	}
}

func TestShareLinksBracketIPv6Authorities(t *testing.T) {
	values := testValues()
	values.ConnectHost = "2001:db8::10"
	output := string(ShareLinks(values))
	if !strings.Contains(output, "@[2001:db8::10]:443?") {
		t.Fatalf("IPv6 share-link authority is not bracketed: %s", output)
	}
}

func TestDisabledProtocolIsAbsentFromAllExports(t *testing.T) {
	values := testValues()
	values.Hysteria2Enabled = false
	configuration, err := Mihomo(values)
	if err != nil {
		t.Fatal(err)
	}
	if output := string(configuration); strings.Contains(output, "Personal-JP-01-Hysteria2") {
		t.Fatalf("disabled Hysteria2 remained in Mihomo output: %s", output)
	}
	if output := string(ShareLinks(values)); strings.Contains(output, "hysteria2://") {
		t.Fatalf("disabled Hysteria2 remained in share links: %s", output)
	}
}

func TestMihomoRenderingIsDeterministicAndUsesNodeMetadata(t *testing.T) {
	values := testValues()
	first, err := Mihomo(values)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Mihomo(values)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("Mihomo rendering must be byte deterministic")
	}
	output := string(first)
	if !strings.Contains(output, "Personal-JP-01-Reality") || !strings.Contains(output, "# Client revision: 7") {
		t.Fatalf("Mihomo metadata was not rendered: %s", output)
	}
}

func TestMihomoGoldenProfiles(t *testing.T) {
	for _, profile := range []struct {
		name    string
		reality bool
		hy2     bool
	}{{"balanced", true, true}, {"reality-only", true, false}, {"hysteria2-only", false, true}} {
		t.Run(profile.name, func(t *testing.T) {
			values := testValues()
			values.RealityEnabled = profile.reality
			values.Hysteria2Enabled = profile.hy2
			configuration, err := Mihomo(values)
			if err != nil {
				t.Fatal(err)
			}
			golden, err := os.ReadFile(filepath.Join("testdata", "mihomo-"+profile.name+".golden.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if string(configuration) != string(golden) {
				t.Fatalf("Mihomo %s output drifted\n--- got ---\n%s\n--- want ---\n%s", profile.name, configuration, golden)
			}
		})
	}
}

func TestMihomoEscapesUnicodeAndYAMLKeywords(t *testing.T) {
	values := testValues()
	values.Node.DisplayName = "东京: yes"
	values.Hysteria2Password = "no: #secret"
	configuration, err := Mihomo(values)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(configuration), "东京: yes-Reality") {
		t.Fatalf("Unicode display name is missing: %s", configuration)
	}
}

func TestMihomoACL4SSRProfilesIncludeExpectedProviders(t *testing.T) {
	for _, test := range []struct {
		profile string
		count   int
		antiAD  bool
	}{
		{model.RulesProfileACL4SSR, 20, false},
		{model.RulesProfileACL4SSRAntiAD, 21, true},
	} {
		t.Run(test.profile, func(t *testing.T) {
			values := testValues()
			values.RulesProfile = test.profile
			values.RulesetRevision = 3
			configuration, err := Mihomo(values)
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]any
			if err := yaml.Unmarshal(configuration, &decoded); err != nil {
				t.Fatal(err)
			}
			providers, ok := decoded["rule-providers"].(map[string]any)
			if !ok || len(providers) != test.count {
				t.Fatalf("unexpected provider count: %#v", providers)
			}
			_, hasAntiAD := providers["anti-AD"]
			if hasAntiAD != test.antiAD {
				t.Fatalf("anti-AD presence=%t, want %t", hasAntiAD, test.antiAD)
			}
			if !strings.Contains(string(configuration), "enhanced-mode: fake-ip") || !strings.Contains(string(configuration), "sniffer:") {
				t.Fatalf("ACL4SSR profile lacks fake-ip DNS or Sniffer: %s", configuration)
			}
			sniffer, ok := decoded["sniffer"].(map[string]any)
			if !ok {
				t.Fatalf("sniffer must be a mapping: %#v", decoded["sniffer"])
			}
			sniff, ok := sniffer["sniff"].(map[string]any)
			if !ok {
				t.Fatalf("sniffer.sniff must be a mapping: %#v", sniffer["sniff"])
			}
			for _, protocol := range []string{"HTTP", "TLS", "QUIC"} {
				if _, ok := sniff[protocol].(map[string]any); !ok {
					t.Fatalf("sniffer.%s must be an object with ports, got %#v", protocol, sniff[protocol])
				}
			}
		})
	}
}

func TestClientArtifactSetDeclaresCompatibilityAndDigests(t *testing.T) {
	set, err := ClientArtifactSet(testValues())
	if err != nil {
		t.Fatal(err)
	}
	if set.SchemaVersion != 1 || set.NodeID != "node-main" || set.ClientRevision != 7 || len(set.Artifacts) != 4 {
		t.Fatalf("unexpected client artifact set: %#v", set)
	}
	for _, item := range set.Artifacts {
		if item.SHA256 == "" || item.RendererVersion < 1 || item.CompatibilityProfile == "" {
			t.Fatalf("artifact lacks renderer metadata: %#v", item)
		}
	}
}

func TestMihomoManagedRuleSourcesUseAuthenticatedBaseURL(t *testing.T) {
	values := testValues()
	values.RulesProfile = model.RulesProfileACL4SSRAntiAD
	values.RulesSourceMode = model.RulesSourceManaged
	values.RuleProviderBaseURL = "https://sub.example/s/read-token/rules"
	content, err := Mihomo(values)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, expected := range []string{
		"https://sub.example/s/read-token/rules/acl4ssr-openai",
		"https://sub.example/s/read-token/rules/anti-ad",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("managed rules configuration is missing %q:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "raw.githubusercontent.com/ACL4SSR") || strings.Contains(text, "https://anti-ad.net/") {
		t.Fatalf("managed rules configuration still contains an upstream URL:\n%s", text)
	}
}

func TestMihomoUserRulesPrecedeAntiAD(t *testing.T) {
	values := testValues()
	values.RulesProfile = model.RulesProfileACL4SSRAntiAD
	values.UserRules = []model.UserRule{
		{Source: model.UserRuleSourceWhitelist, Type: model.UserRuleTypeDomain, Value: "captcha.example.com", Policy: model.UserRulePolicyDirect},
		{Source: model.UserRuleSourceCustom, Type: model.UserRuleTypeDomainSuffix, Value: "example.org", Policy: model.UserRulePolicyProxy},
		{Source: model.UserRuleSourceCustom, Type: model.UserRuleTypeIPCIDR, Value: "198.51.100.0/24", Policy: model.UserRulePolicyReject},
	}
	configuration, err := Mihomo(values)
	if err != nil {
		t.Fatal(err)
	}
	output := string(configuration)
	for _, expected := range []string{
		"DOMAIN,captcha.example.com,DIRECT",
		"DOMAIN-SUFFIX,example.org,Proxy",
		"IP-CIDR,198.51.100.0/24,REJECT,no-resolve",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("missing user rule %q:\n%s", expected, output)
		}
		if strings.Index(output, expected) > strings.Index(output, "RULE-SET,anti-AD,REJECT") {
			t.Fatalf("user rule must precede anti-AD: %q\n%s", expected, output)
		}
	}
}

func TestMihomoRuleSourcesAreStableAndComplete(t *testing.T) {
	sources, err := MihomoRuleSources(model.RulesProfileACL4SSRAntiAD)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 20 {
		t.Fatalf("unexpected managed rule source count: %d", len(sources))
	}
	seen := map[string]bool{}
	for _, source := range sources {
		if seen[source.Target] || source.Target == "" || source.URL == "" || source.MediaType == "" {
			t.Fatalf("invalid source: %#v", source)
		}
		seen[source.Target] = true
	}
	if !seen["anti-ad"] || !seen["google-all-domain"] || !seen["acl4ssr-openai"] {
		t.Fatalf("expected source targets missing: %#v", seen)
	}
}

func TestRendererCapabilitiesTrackPinnedWindowsClients(t *testing.T) {
	capabilities := RendererCapabilities()
	joined := ""
	for _, capability := range capabilities {
		joined += capability.CompatibilityProfile + " " + capability.MinimumTestedClientVersion + "\n"
	}
	for _, wanted := range []string{"mihomo-1.19.29", "Clash Verge Rev 2.5.2", "v2rayn-7.23.1", "v2rayN 7.23.1"} {
		if !strings.Contains(joined, wanted) {
			t.Fatalf("missing compatibility baseline %q in %s", wanted, joined)
		}
	}
}

func TestMihomoPinnedBinaryParsesConfig(t *testing.T) {
	binary := os.Getenv("VPSKIT_MIHOMO_TEST_BIN")
	if binary == "" {
		t.Skip("VPSKIT_MIHOMO_TEST_BIN is not set")
	}
	configuration, err := Mihomo(testValues())
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	path := filepath.Join(directory, "mihomo.yaml")
	if err := os.WriteFile(path, configuration, 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(binary, "-t", "-d", directory, "-f", path).CombinedOutput()
	if err != nil {
		t.Fatalf("Mihomo rejected generated configuration: %v: %s", err, output)
	}
}

func TestSingBoxParsesClientConfigs(t *testing.T) {
	binary := os.Getenv("VPSKIT_SING_BOX_TEST_BIN")
	if binary == "" {
		t.Skip("VPSKIT_SING_BOX_TEST_BIN is not set")
	}
	keyOutput, err := exec.Command(binary, "generate", "reality-keypair").CombinedOutput()
	if err != nil {
		t.Fatalf("generate Reality key pair: %v: %s", err, keyOutput)
	}
	values := testValues()
	for _, line := range strings.Split(string(keyOutput), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PrivateKey:") {
			values.RealityPrivateKey = strings.TrimSpace(strings.TrimPrefix(line, "PrivateKey:"))
		}
		if strings.HasPrefix(line, "PublicKey:") {
			values.RealityPublicKey = strings.TrimSpace(strings.TrimPrefix(line, "PublicKey:"))
		}
	}
	configurations := map[string][]byte{}
	configurations["reality.json"], err = SingBoxRealityClient(values, 2080)
	if err != nil {
		t.Fatal(err)
	}
	configurations["hysteria2.json"], err = SingBoxHysteria2Client(values, 2081)
	if err != nil {
		t.Fatal(err)
	}
	for name, configuration := range configurations {
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, configuration, 0o600); err != nil {
			t.Fatal(err)
		}
		output, err := exec.Command(binary, "check", "-c", path).CombinedOutput()
		if err != nil {
			t.Fatalf("sing-box rejected %s: %v: %s", name, err, output)
		}
	}
}

func TestXrayParsesRealityServerConfig(t *testing.T) {
	xrayBinary := os.Getenv("VPSKIT_XRAY_TEST_BIN")
	singBoxBinary := os.Getenv("VPSKIT_SING_BOX_TEST_BIN")
	if xrayBinary == "" || singBoxBinary == "" {
		t.Skip("VPSKIT_XRAY_TEST_BIN and VPSKIT_SING_BOX_TEST_BIN are not set")
	}
	keyOutput, err := exec.Command(singBoxBinary, "generate", "reality-keypair").CombinedOutput()
	if err != nil {
		t.Fatalf("generate Reality key pair: %v: %s", err, keyOutput)
	}
	values := testValues()
	values.Hysteria2Enabled = false
	for _, line := range strings.Split(string(keyOutput), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PrivateKey:") {
			values.RealityPrivateKey = strings.TrimSpace(strings.TrimPrefix(line, "PrivateKey:"))
		}
		if strings.HasPrefix(line, "PublicKey:") {
			values.RealityPublicKey = strings.TrimSpace(strings.TrimPrefix(line, "PublicKey:"))
		}
	}
	configuration, err := XrayRealityServerConfig(values)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "xray-reality.json")
	if err := os.WriteFile(path, configuration, 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(xrayBinary, "run", "-test", "-config", path).CombinedOutput()
	if err != nil {
		t.Fatalf("Xray rejected REALITY server configuration: %v: %s", err, output)
	}
}
