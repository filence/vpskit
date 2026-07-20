package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"vpskit.local/vpskit/internal/model"
)

func ServerConfig(values model.RuntimeValues) ([]byte, error) {
	inbounds := make([]any, 0, 1)
	if values.Hysteria2Enabled {
		inbounds = append(inbounds,
			map[string]any{
				"type":        "hysteria2",
				"tag":         "hy2-backup",
				"listen":      "::",
				"listen_port": values.UDPPort,
				"users": []any{map[string]any{
					"name":     "default",
					"password": values.Hysteria2Password,
				}},
				"tls": map[string]any{
					"enabled":          true,
					"server_name":      values.Domain,
					"certificate_path": values.CertificatePath,
					"key_path":         values.KeyPath,
				},
			},
		)
	}
	configuration := map[string]any{
		"log": map[string]any{
			"level":     "warn",
			"timestamp": true,
		},
		"inbounds":  inbounds,
		"outbounds": []any{map[string]any{"type": "direct", "tag": "direct"}},
	}
	return marshalJSON(configuration)
}

// XrayRealityServerConfig renders the TCP REALITY listener separately from
// sing-box. This is intentional: current Mihomo and Xray clients authenticate
// successfully against Xray's REALITY server, while sing-box's server-side
// implementation is not interoperable with those clients.
func XrayRealityServerConfig(values model.RuntimeValues) ([]byte, error) {
	inbounds := make([]any, 0, 1)
	if values.RealityEnabled {
		inbounds = append(inbounds, map[string]any{
			"listen":   "::",
			"port":     values.TCPPort,
			"protocol": "vless",
			"settings": map[string]any{
				"clients": []any{map[string]any{
					"id":   values.RealityUUID,
					"flow": "xtls-rprx-vision",
				}},
				"decryption": "none",
			},
			"streamSettings": map[string]any{
				"network":  "raw",
				"security": "reality",
				"realitySettings": map[string]any{
					"show":        false,
					"target":      net.JoinHostPort(values.RealityServerName, "443"),
					"xver":        0,
					"serverNames": []string{values.RealityServerName},
					"privateKey":  values.RealityPrivateKey,
					"shortIds":    []string{values.RealityShortID},
				},
			},
			"tag": "reality-main",
		})
	}
	configuration := map[string]any{
		"log":       map[string]any{"loglevel": "warning"},
		"inbounds":  inbounds,
		"outbounds": []any{map[string]any{"protocol": "freedom", "tag": "direct"}},
	}
	return marshalJSON(configuration)
}

func SingBoxRealityClient(values model.RuntimeValues, socksPort int) ([]byte, error) {
	configuration := clientBase(socksPort, map[string]any{
		"type":        "vless",
		"tag":         "proxy",
		"server":      values.ConnectHost,
		"server_port": values.TCPPort,
		"uuid":        values.RealityUUID,
		"flow":        "xtls-rprx-vision",
		"network":     "tcp",
		"tls": map[string]any{
			"enabled":     true,
			"server_name": values.RealityServerName,
			"utls": map[string]any{
				"enabled":     true,
				"fingerprint": "chrome",
			},
			"reality": map[string]any{
				"enabled":    true,
				"public_key": values.RealityPublicKey,
				"short_id":   values.RealityShortID,
			},
		},
	})
	return marshalJSON(configuration)
}

func SingBoxHysteria2Client(values model.RuntimeValues, socksPort int) ([]byte, error) {
	configuration := clientBase(socksPort, map[string]any{
		"type":        "hysteria2",
		"tag":         "proxy",
		"server":      values.ConnectHost,
		"server_port": values.UDPPort,
		"password":    values.Hysteria2Password,
		"tls": map[string]any{
			"enabled":     true,
			"server_name": values.Domain,
		},
	})
	return marshalJSON(configuration)
}

func clientBase(socksPort int, outbound map[string]any) map[string]any {
	return map[string]any{
		"log": map[string]any{"level": "warn", "timestamp": true},
		"inbounds": []any{map[string]any{
			"type":        "mixed",
			"tag":         "mixed-in",
			"listen":      "127.0.0.1",
			"listen_port": socksPort,
		}},
		"outbounds": []any{outbound, map[string]any{"type": "direct", "tag": "direct"}},
		"route":     map[string]any{"final": "proxy", "auto_detect_interface": true},
	}
}

func Mihomo(values model.RuntimeValues) []byte {
	quote := strconv.Quote
	var output strings.Builder
	output.WriteString("mixed-port: 7890\n")
	output.WriteString("allow-lan: false\n")
	output.WriteString("mode: rule\n")
	output.WriteString("log-level: warning\n")
	output.WriteString("proxies:\n")
	proxies := make([]string, 0, 2)
	if values.RealityEnabled {
		proxies = append(proxies, "JP-Reality")
		output.WriteString("  - name: JP-Reality\n")
		output.WriteString("    type: vless\n")
		output.WriteString("    server: " + quote(values.ConnectHost) + "\n")
		output.WriteString(fmt.Sprintf("    port: %d\n", values.TCPPort))
		output.WriteString("    uuid: " + quote(values.RealityUUID) + "\n")
		output.WriteString("    network: tcp\n")
		output.WriteString("    tls: true\n")
		output.WriteString("    udp: true\n")
		output.WriteString("    flow: xtls-rprx-vision\n")
		output.WriteString("    servername: " + quote(values.RealityServerName) + "\n")
		output.WriteString("    client-fingerprint: chrome\n")
		output.WriteString("    reality-opts:\n")
		output.WriteString("      public-key: " + quote(values.RealityPublicKey) + "\n")
		output.WriteString("      short-id: " + quote(values.RealityShortID) + "\n")
	}
	if values.Hysteria2Enabled {
		proxies = append(proxies, "JP-Hysteria2")
		output.WriteString("  - name: JP-Hysteria2\n")
		output.WriteString("    type: hysteria2\n")
		output.WriteString("    server: " + quote(values.ConnectHost) + "\n")
		output.WriteString(fmt.Sprintf("    port: %d\n", values.UDPPort))
		output.WriteString("    password: " + quote(values.Hysteria2Password) + "\n")
		output.WriteString("    sni: " + quote(values.Domain) + "\n")
		output.WriteString("    skip-cert-verify: false\n")
	}
	output.WriteString("proxy-groups:\n")
	output.WriteString("  - name: Proxy\n")
	output.WriteString("    type: select\n")
	proxies = append(proxies, "DIRECT")
	output.WriteString("    proxies: [" + strings.Join(proxies, ", ") + "]\n")
	output.WriteString("rules:\n")
	output.WriteString("  - DOMAIN-SUFFIX,local,DIRECT\n")
	output.WriteString("  - IP-CIDR,10.0.0.0/8,DIRECT,no-resolve\n")
	output.WriteString("  - IP-CIDR,172.16.0.0/12,DIRECT,no-resolve\n")
	output.WriteString("  - IP-CIDR,192.168.0.0/16,DIRECT,no-resolve\n")
	output.WriteString("  - MATCH,Proxy\n")
	return []byte(output.String())
}

func ShareLinks(values model.RuntimeValues) []byte {
	links := make([]string, 0, 2)
	if values.RealityEnabled {
		realityQuery := url.Values{}
		realityQuery.Set("encryption", "none")
		realityQuery.Set("flow", "xtls-rprx-vision")
		realityQuery.Set("security", "reality")
		realityQuery.Set("sni", values.RealityServerName)
		realityQuery.Set("fp", "chrome")
		realityQuery.Set("pbk", values.RealityPublicKey)
		realityQuery.Set("sid", values.RealityShortID)
		realityQuery.Set("type", "tcp")
		links = append(links, fmt.Sprintf(
			"vless://%s@%s?%s#JP-Reality",
			url.PathEscape(values.RealityUUID), net.JoinHostPort(values.ConnectHost, strconv.Itoa(values.TCPPort)), realityQuery.Encode(),
		))
	}
	if values.Hysteria2Enabled {
		hy2Query := url.Values{}
		hy2Query.Set("sni", values.Domain)
		links = append(links, fmt.Sprintf(
			"hysteria2://%s@%s?%s#JP-Hysteria2",
			url.PathEscape(values.Hysteria2Password), net.JoinHostPort(values.ConnectHost, strconv.Itoa(values.UDPPort)), hy2Query.Encode(),
		))
	}
	return []byte(strings.Join(links, "\n") + "\n")
}

func marshalJSON(value any) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
