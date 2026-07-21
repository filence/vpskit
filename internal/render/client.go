package render

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"strings"

	"go.yaml.in/yaml/v3"

	"vpskit.local/vpskit/internal/artifact"
	"vpskit.local/vpskit/internal/model"
)

type ClientRenderer interface {
	Capability() artifact.Capability
	Render(model.RuntimeValues) ([]artifact.Artifact, error)
}

type MihomoRenderer struct{}
type SingBoxRenderer struct{}
type LinkRenderer struct{}

func (MihomoRenderer) Capability() artifact.Capability {
	return artifact.Capability{
		Name:                       "mihomo",
		RendererVersion:            1,
		CompatibilityProfile:       "mihomo-1.19.29",
		SupportedProtocols:         []string{"vless-reality-vision", "hysteria2"},
		MinimumTestedClientVersion: "Clash Verge Rev 2.5.2 / Mihomo 1.19.29",
		SupportsFullConfig:         true,
		SupportsNodeSubscription:   true,
		SupportsRemoteRules:        true,
		SupportsIPv6:               true,
		ValidationCommand:          "mihomo -t -d <temp-dir> -f <config>",
	}
}

func (SingBoxRenderer) Capability() artifact.Capability {
	return artifact.Capability{
		Name:                       "sing-box",
		RendererVersion:            1,
		CompatibilityProfile:       "sing-box-pinned",
		SupportedProtocols:         []string{"vless-reality-vision", "hysteria2"},
		MinimumTestedClientVersion: "sing-box pinned by release manifest",
		SupportsFullConfig:         true,
		SupportsIPv6:               true,
		ValidationCommand:          "sing-box check -c <config>",
	}
}

func (LinkRenderer) Capability() artifact.Capability {
	return artifact.Capability{
		Name:                       "share-links",
		RendererVersion:            1,
		CompatibilityProfile:       "v2rayn-7.23.1",
		SupportedProtocols:         []string{"vless-reality-vision", "hysteria2"},
		MinimumTestedClientVersion: "v2rayN 7.23.1",
		SupportsNodeSubscription:   true,
		SupportsIPv6:               true,
	}
}

func ClientRenderers() []ClientRenderer {
	return []ClientRenderer{MihomoRenderer{}, SingBoxRenderer{}, LinkRenderer{}}
}

func RendererCapabilities() []artifact.Capability {
	result := make([]artifact.Capability, 0, len(ClientRenderers()))
	for _, renderer := range ClientRenderers() {
		result = append(result, renderer.Capability())
	}
	return result
}

func ClientArtifactSet(values model.RuntimeValues) (artifact.Set, error) {
	values.Node = normalizedNodeMetadata(values.Node)
	if values.ClientRevision < 1 {
		values.ClientRevision = 1
	}
	items := make([]artifact.Artifact, 0, 4)
	for _, renderer := range ClientRenderers() {
		rendered, err := renderer.Render(values)
		if err != nil {
			return artifact.Set{}, fmt.Errorf("render %s: %w", renderer.Capability().Name, err)
		}
		items = append(items, rendered...)
	}
	return artifact.NewSet(values.Node.ID, values.ClientRevision, values.RulesetRevision, items)
}

func (renderer MihomoRenderer) Render(values model.RuntimeValues) ([]artifact.Artifact, error) {
	content, err := Mihomo(values)
	if err != nil {
		return nil, err
	}
	item, err := artifact.New("mihomo.yaml", "mihomo", "text/yaml; charset=utf-8", true, renderer.Capability(), content)
	if err != nil {
		return nil, err
	}
	return []artifact.Artifact{item}, nil
}

func (renderer SingBoxRenderer) Render(values model.RuntimeValues) ([]artifact.Artifact, error) {
	items := make([]artifact.Artifact, 0, 2)
	if values.RealityEnabled {
		content, err := SingBoxRealityClient(values, 2080)
		if err != nil {
			return nil, err
		}
		item, err := artifact.New("sing-box-reality.json", "sing-box", "application/json", true, renderer.Capability(), content)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if values.Hysteria2Enabled {
		content, err := SingBoxHysteria2Client(values, 2081)
		if err != nil {
			return nil, err
		}
		item, err := artifact.New("sing-box-hysteria2.json", "sing-box", "application/json", true, renderer.Capability(), content)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (renderer LinkRenderer) Render(values model.RuntimeValues) ([]artifact.Artifact, error) {
	if !values.RealityEnabled && !values.Hysteria2Enabled {
		return nil, errors.New("at least one protocol must be enabled")
	}
	item, err := artifact.New("share-links.txt", "share-links", "text/plain; charset=utf-8", true, renderer.Capability(), ShareLinks(values))
	if err != nil {
		return nil, err
	}
	return []artifact.Artifact{item}, nil
}

type mihomoConfig struct {
	MixedPort     int                           `yaml:"mixed-port"`
	AllowLAN      bool                          `yaml:"allow-lan"`
	Mode          string                        `yaml:"mode"`
	LogLevel      string                        `yaml:"log-level"`
	DNS           *mihomoDNS                    `yaml:"dns,omitempty"`
	Sniffer       *mihomoSniffer                `yaml:"sniffer,omitempty"`
	Proxies       []any                         `yaml:"proxies"`
	ProxyGroups   []mihomoProxyGroup            `yaml:"proxy-groups"`
	RuleProviders map[string]mihomoRuleProvider `yaml:"rule-providers,omitempty"`
	Rules         []string                      `yaml:"rules"`
}

type mihomoVLESSRealityProxy struct {
	Name              string               `yaml:"name"`
	Type              string               `yaml:"type"`
	Server            string               `yaml:"server"`
	Port              int                  `yaml:"port"`
	UUID              string               `yaml:"uuid"`
	Network           string               `yaml:"network"`
	TLS               bool                 `yaml:"tls"`
	UDP               bool                 `yaml:"udp"`
	Flow              string               `yaml:"flow"`
	ServerName        string               `yaml:"servername"`
	ClientFingerprint string               `yaml:"client-fingerprint"`
	RealityOptions    mihomoRealityOptions `yaml:"reality-opts"`
}

type mihomoRealityOptions struct {
	PublicKey string `yaml:"public-key"`
	ShortID   string `yaml:"short-id"`
}

type mihomoHysteria2Proxy struct {
	Name                 string `yaml:"name"`
	Type                 string `yaml:"type"`
	Server               string `yaml:"server"`
	Port                 int    `yaml:"port"`
	Password             string `yaml:"password"`
	SNI                  string `yaml:"sni"`
	SkipCertVerification bool   `yaml:"skip-cert-verify"`
}

type mihomoProxyGroup struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	Proxies []string `yaml:"proxies,flow"`
}

func Mihomo(values model.RuntimeValues) ([]byte, error) {
	if err := validateClientValues(values); err != nil {
		return nil, err
	}
	realityName, hysteria2Name := protocolDisplayNames(values.Node)
	proxies := make([]any, 0, 2)
	proxyNames := make([]string, 0, 3)
	if values.RealityEnabled {
		proxies = append(proxies, mihomoVLESSRealityProxy{
			Name: realityName, Type: "vless", Server: values.ConnectHost, Port: values.TCPPort,
			UUID: values.RealityUUID, Network: "tcp", TLS: true, UDP: true,
			Flow: "xtls-rprx-vision", ServerName: values.RealityServerName, ClientFingerprint: "chrome",
			RealityOptions: mihomoRealityOptions{PublicKey: values.RealityPublicKey, ShortID: values.RealityShortID},
		})
		proxyNames = append(proxyNames, realityName)
	}
	if values.Hysteria2Enabled {
		proxies = append(proxies, mihomoHysteria2Proxy{
			Name: hysteria2Name, Type: "hysteria2", Server: values.ConnectHost, Port: values.UDPPort,
			Password: values.Hysteria2Password, SNI: values.Domain, SkipCertVerification: false,
		})
		proxyNames = append(proxyNames, hysteria2Name)
	}
	proxyNames = append(proxyNames, "DIRECT")
	ruleProfile, err := newMihomoRuleProfile(values.RulesProfile)
	if err != nil {
		return nil, err
	}
	if values.RulesSourceMode == model.RulesSourceManaged {
		if err := useManagedMihomoRuleSources(&ruleProfile, values.RulesProfile, values.RuleProviderBaseURL); err != nil {
			return nil, err
		}
	}
	configuration := mihomoConfig{
		MixedPort: 7890, AllowLAN: false, Mode: "rule", LogLevel: "warning",
		DNS: ruleProfile.DNS, Sniffer: ruleProfile.Sniffer,
		Proxies: proxies, ProxyGroups: []mihomoProxyGroup{{Name: "Proxy", Type: "select", Proxies: proxyNames}},
		RuleProviders: ruleProfile.Providers, Rules: ruleProfile.Rules,
	}
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(configuration); err != nil {
		return nil, fmt.Errorf("encode Mihomo YAML: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close Mihomo YAML encoder: %w", err)
	}
	var parsed yaml.Node
	if err := yaml.Unmarshal(buffer.Bytes(), &parsed); err != nil {
		return nil, fmt.Errorf("validate rendered Mihomo YAML: %w", err)
	}
	revision := values.ClientRevision
	if revision < 1 {
		revision = 1
	}
	header := fmt.Sprintf("# Generated by VPSKit\n# Renderer: mihomo/v1\n# Client revision: %d\n", revision)
	return append([]byte(header), buffer.Bytes()...), nil
}

func validateClientValues(values model.RuntimeValues) error {
	if !values.RealityEnabled && !values.Hysteria2Enabled {
		return errors.New("at least one client protocol must be enabled")
	}
	if strings.TrimSpace(values.ConnectHost) == "" {
		return errors.New("client connection host is required")
	}
	if ip := net.ParseIP(values.ConnectHost); ip == nil && strings.ContainsAny(values.ConnectHost, " /?#") {
		return errors.New("client connection host contains invalid characters")
	}
	if values.RealityEnabled {
		if values.TCPPort < 1 || values.TCPPort > 65535 || strings.TrimSpace(values.RealityUUID) == "" || strings.TrimSpace(values.RealityPublicKey) == "" || strings.TrimSpace(values.RealityShortID) == "" || strings.TrimSpace(values.RealityServerName) == "" {
			return errors.New("enabled Reality client is missing required connection fields")
		}
	}
	if values.Hysteria2Enabled {
		if values.UDPPort < 1 || values.UDPPort > 65535 || strings.TrimSpace(values.Hysteria2Password) == "" || strings.TrimSpace(values.Domain) == "" {
			return errors.New("enabled Hysteria2 client is missing required connection fields")
		}
	}
	return nil
}

func normalizedNodeMetadata(node model.NodeMetadata) model.NodeMetadata {
	if strings.TrimSpace(node.ID) == "" {
		node.ID = "node-main"
	}
	if strings.TrimSpace(node.DisplayName) == "" {
		// Keep schema 5 exports byte-compatible at the display-name level.
		node.DisplayName = "JP"
	}
	return node
}

func protocolDisplayNames(node model.NodeMetadata) (string, string) {
	node = normalizedNodeMetadata(node)
	base := strings.TrimSpace(node.DisplayName)
	return base + "-Reality", base + "-Hysteria2"
}
