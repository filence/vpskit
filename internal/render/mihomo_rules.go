package render

import (
	"fmt"
	"sort"
	"strings"

	"vpskit.local/vpskit/internal/model"
)

// MihomoRuleSource is an upstream rule file which can be captured by VPSKit
// and published through the owner's authenticated subscription endpoint.
// Target is URL-safe and stable across revisions.
type MihomoRuleSource struct {
	Name      string
	Target    string
	URL       string
	MediaType string
}

type mihomoDNS struct {
	Enable            bool                `yaml:"enable"`
	EnhancedMode      string              `yaml:"enhanced-mode"`
	CacheAlgorithm    string              `yaml:"cache-algorithm"`
	DefaultNameserver []string            `yaml:"default-nameserver"`
	Nameserver        []string            `yaml:"nameserver"`
	DirectNameserver  []string            `yaml:"direct-nameserver"`
	NameserverPolicy  map[string][]string `yaml:"nameserver-policy"`
	FakeIPFilter      []string            `yaml:"fake-ip-filter"`
}

type mihomoSniffer struct {
	Enable bool                            `yaml:"enable"`
	Sniff  map[string]mihomoSniffingConfig `yaml:"sniff"`
}

type mihomoSniffingConfig struct {
	Ports []string `yaml:"ports"`
}

type mihomoRuleProvider struct {
	Type     string   `yaml:"type"`
	Behavior string   `yaml:"behavior"`
	Format   string   `yaml:"format,omitempty"`
	URL      string   `yaml:"url,omitempty"`
	Path     string   `yaml:"path,omitempty"`
	Interval int      `yaml:"interval,omitempty"`
	Proxy    string   `yaml:"proxy,omitempty"`
	Payload  []string `yaml:"payload,omitempty"`
}

type mihomoRuleProfile struct {
	DNS       *mihomoDNS
	Sniffer   *mihomoSniffer
	Providers map[string]mihomoRuleProvider
	Rules     []string
}

func newMihomoRuleProfile(profile string) (mihomoRuleProfile, error) {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "", model.RulesProfileMinimal:
		return mihomoRuleProfile{Rules: []string{
			"DOMAIN-SUFFIX,local,DIRECT",
			"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
			"IP-CIDR,172.16.0.0/12,DIRECT,no-resolve",
			"IP-CIDR,192.168.0.0/16,DIRECT,no-resolve",
			"MATCH,Proxy",
		}}, nil
	case model.RulesProfileACL4SSR, model.RulesProfileACL4SSRAntiAD:
		withAntiAD := strings.EqualFold(strings.TrimSpace(profile), model.RulesProfileACL4SSRAntiAD)
		return acl4SSRRuleProfile(withAntiAD), nil
	default:
		return mihomoRuleProfile{}, fmt.Errorf("unsupported Mihomo rules profile %q", profile)
	}
}

func userRuleLines(userRules []model.UserRule) ([]string, error) {
	ordered := append([]model.UserRule(nil), userRules...)
	sort.Slice(ordered, func(left, right int) bool {
		leftRank, rightRank := userRuleTypeRank(ordered[left].Type), userRuleTypeRank(ordered[right].Type)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if ordered[left].Value != ordered[right].Value {
			return ordered[left].Value < ordered[right].Value
		}
		return ordered[left].Policy < ordered[right].Policy
	})
	lines := make([]string, 0, len(ordered))
	for _, rule := range ordered {
		switch rule.Type {
		case model.UserRuleTypeDomain:
			lines = append(lines, "DOMAIN,"+rule.Value+","+rule.Policy)
		case model.UserRuleTypeDomainSuffix:
			lines = append(lines, "DOMAIN-SUFFIX,"+rule.Value+","+rule.Policy)
		case model.UserRuleTypeIPCIDR:
			lines = append(lines, "IP-CIDR,"+rule.Value+","+rule.Policy+",no-resolve")
		default:
			return nil, fmt.Errorf("unsupported user rule type %q", rule.Type)
		}
	}
	return lines, nil
}

func userRuleTypeRank(ruleType string) int {
	switch ruleType {
	case model.UserRuleTypeDomain:
		return 0
	case model.UserRuleTypeDomainSuffix:
		return 1
	case model.UserRuleTypeIPCIDR:
		return 2
	default:
		return 3
	}
}

// MihomoRuleSources returns the complete remote source set used by a profile.
// Inline providers such as Google-Antigravity deliberately do not appear here.
func MihomoRuleSources(profile string) ([]MihomoRuleSource, error) {
	ruleProfile, err := newMihomoRuleProfile(profile)
	if err != nil {
		return nil, err
	}
	sources := make([]MihomoRuleSource, 0, len(ruleProfile.Providers))
	for name, provider := range ruleProfile.Providers {
		if provider.Type != "http" || strings.TrimSpace(provider.URL) == "" {
			continue
		}
		mediaType := "text/plain; charset=utf-8"
		if provider.Format == "mrs" {
			mediaType = "application/octet-stream"
		}
		sources = append(sources, MihomoRuleSource{
			Name: name, Target: mihomoRuleTarget(name), URL: provider.URL, MediaType: mediaType,
		})
	}
	sort.Slice(sources, func(left, right int) bool { return sources[left].Target < sources[right].Target })
	return sources, nil
}

func mihomoRuleTarget(name string) string {
	var result strings.Builder
	for _, value := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case value >= 'a' && value <= 'z', value >= '0' && value <= '9':
			result.WriteRune(value)
		default:
			result.WriteByte('-')
		}
	}
	return strings.Trim(result.String(), "-")
}

func useManagedMihomoRuleSources(ruleProfile *mihomoRuleProfile, profile, baseURL string) error {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return fmt.Errorf("managed rules profile %q requires a subscription rule URL", profile)
	}
	sources, err := MihomoRuleSources(profile)
	if err != nil {
		return err
	}
	for _, source := range sources {
		provider := ruleProfile.Providers[source.Name]
		provider.URL = baseURL + "/" + source.Target
		ruleProfile.Providers[source.Name] = provider
	}
	return nil
}

func acl4SSRRuleProfile(withAntiAD bool) mihomoRuleProfile {
	providers := map[string]mihomoRuleProvider{
		"ACL4SSR-LocalAreaNetwork": acl4SSRProvider("LocalAreaNetwork.list"),
		"ACL4SSR-UnBan":            acl4SSRProvider("UnBan.list"),
		"ACL4SSR-Gemini":           acl4SSRProvider("Ruleset/Gemini.list"),
		"ACL4SSR-SteamCN":          acl4SSRProvider("Ruleset/SteamCN.list"),
		"ACL4SSR-Telegram":         acl4SSRProvider("Telegram.list"),
		"ACL4SSR-AI":               acl4SSRProvider("Ruleset/AI.list"),
		"ACL4SSR-OpenAi":           acl4SSRProvider("Ruleset/OpenAi.list"),
		"ACL4SSR-Github":           acl4SSRProvider("Ruleset/Github.list"),
		"ACL4SSR-YouTube":          acl4SSRProvider("Ruleset/YouTube.list"),
		"ACL4SSR-ProxyMedia":       acl4SSRProvider("ProxyMedia.list"),
		"ACL4SSR-Bing":             acl4SSRProvider("Bing.list"),
		"ACL4SSR-OneDrive":         acl4SSRProvider("OneDrive.list"),
		"ACL4SSR-Microsoft":        acl4SSRProvider("Microsoft.list"),
		"ACL4SSR-Apple":            acl4SSRProvider("Apple.list"),
		"ACL4SSR-ChinaDomain":      acl4SSRProvider("ChinaDomain.list"),
		"ACL4SSR-ChinaCompanyIp":   acl4SSRProvider("ChinaCompanyIp.list"),
		"ACL4SSR-ProxyGFWlist":     acl4SSRProvider("ProxyGFWlist.list"),
		"Google-Antigravity": {Type: "file", Behavior: "classical", Format: "yaml", Path: "./ruleset/vpskit-google-antigravity.yaml", Payload: []string{
			"DOMAIN-SUFFIX,antigravity.google", "DOMAIN-SUFFIX,pkg.dev",
		}},
		"Google-All-Domain": {Type: "http", Behavior: "domain", Format: "mrs", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/google.mrs", Path: "./ruleset/google-all-domain.mrs", Interval: 86400, Proxy: "Proxy"},
		"Google-All-IP":     {Type: "http", Behavior: "ipcidr", Format: "mrs", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geoip/google.mrs", Path: "./ruleset/google-all-ip.mrs", Interval: 86400, Proxy: "Proxy"},
	}
	if withAntiAD {
		providers["anti-AD"] = mihomoRuleProvider{Type: "http", Behavior: "domain", Format: "mrs", URL: "https://anti-ad.net/mihomo.mrs", Path: "./ruleset/anti-ad.mrs", Interval: 86400, Proxy: "Proxy"}
	}
	rules := []string{
		"RULE-SET,ACL4SSR-LocalAreaNetwork,DIRECT",
		"RULE-SET,Google-Antigravity,Proxy",
		"RULE-SET,ACL4SSR-Gemini,Proxy",
		"RULE-SET,ACL4SSR-YouTube,Proxy",
		"RULE-SET,Google-All-Domain,Proxy",
		"RULE-SET,Google-All-IP,Proxy,no-resolve",
		"RULE-SET,ACL4SSR-UnBan,DIRECT",
	}
	if withAntiAD {
		rules = append(rules, "RULE-SET,anti-AD,REJECT")
	}
	rules = append(rules,
		"RULE-SET,ACL4SSR-Telegram,Proxy", "RULE-SET,ACL4SSR-AI,Proxy", "RULE-SET,ACL4SSR-OpenAi,Proxy", "RULE-SET,ACL4SSR-Github,Proxy", "RULE-SET,ACL4SSR-ProxyMedia,Proxy", "RULE-SET,ACL4SSR-Bing,Proxy",
		"RULE-SET,ACL4SSR-OneDrive,DIRECT", "RULE-SET,ACL4SSR-Microsoft,DIRECT", "RULE-SET,ACL4SSR-Apple,DIRECT", "RULE-SET,ACL4SSR-SteamCN,DIRECT",
		"RULE-SET,ACL4SSR-ChinaDomain,DIRECT", "RULE-SET,ACL4SSR-ProxyGFWlist,Proxy", "RULE-SET,ACL4SSR-ChinaCompanyIp,DIRECT", "GEOIP,CN,DIRECT", "MATCH,Proxy",
	)
	return mihomoRuleProfile{DNS: acl4SSRDNS(), Sniffer: defaultMihomoSniffer(), Providers: providers, Rules: rules}
}

func acl4SSRProvider(path string) mihomoRuleProvider {
	name := strings.ReplaceAll(strings.ToLower(path), "/", "-")
	return mihomoRuleProvider{Type: "http", Behavior: "classical", Format: "text", URL: "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/" + path, Path: "./ruleset/acl4ssr-" + name, Interval: 86400, Proxy: "Proxy"}
}

func acl4SSRDNS() *mihomoDNS {
	return &mihomoDNS{Enable: true, EnhancedMode: "fake-ip", CacheAlgorithm: "arc",
		DefaultNameserver: []string{"223.5.5.5", "119.29.29.29"},
		Nameserver:        []string{"https://1.1.1.1/dns-query#Proxy", "https://8.8.8.8/dns-query#Proxy"},
		DirectNameserver:  []string{"https://dns.alidns.com/dns-query#DIRECT", "https://doh.pub/dns-query#DIRECT"},
		NameserverPolicy: map[string][]string{
			"rule-set:Google-Antigravity":  {"https://1.1.1.1/dns-query#Proxy", "https://8.8.8.8/dns-query#Proxy"},
			"rule-set:ACL4SSR-Gemini":      {"https://1.1.1.1/dns-query#Proxy", "https://8.8.8.8/dns-query#Proxy"},
			"rule-set:Google-All-Domain":   {"https://1.1.1.1/dns-query#Proxy", "https://8.8.8.8/dns-query#Proxy"},
			"rule-set:ACL4SSR-ChinaDomain": {"https://dns.alidns.com/dns-query#DIRECT", "https://doh.pub/dns-query#DIRECT"},
		},
		FakeIPFilter: []string{"*.lan", "*.local", "localhost", "+.msftconnecttest.com", "+.msftncsi.com", "rule-set:ACL4SSR-LocalAreaNetwork"},
	}
}

func defaultMihomoSniffer() *mihomoSniffer {
	return &mihomoSniffer{Enable: true, Sniff: map[string]mihomoSniffingConfig{
		"HTTP": {Ports: []string{"80", "8080-8880"}},
		"TLS":  {Ports: []string{"443", "8443"}},
		"QUIC": {Ports: []string{"443", "8443"}},
	}}
}
