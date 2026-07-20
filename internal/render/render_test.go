package render

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"vpskit.local/vpskit/internal/model"
)

func testValues() model.RuntimeValues {
	return model.RuntimeValues{
		RealityEnabled:    true,
		Hysteria2Enabled:  true,
		ConnectHost:       "node.example.com",
		Domain:            "node.example.com",
		RealityServerName: "www.example.com",
		TCPPort:           443,
		UDPPort:           443,
		RealityUUID:       "11111111-2222-4333-8444-555555555555",
		RealityPrivateKey: "private",
		RealityPublicKey:  "public",
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
	output := string(Mihomo(testValues()))
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
	if output := string(Mihomo(values)); strings.Contains(output, "JP-Hysteria2") {
		t.Fatalf("disabled Hysteria2 remained in Mihomo output: %s", output)
	}
	if output := string(ShareLinks(values)); strings.Contains(output, "hysteria2://") {
		t.Fatalf("disabled Hysteria2 remained in share links: %s", output)
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
