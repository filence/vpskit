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
		inbound := map[string]any{
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
		}
		if values.Hysteria2Obfuscation != "" {
			inbound["obfs"] = map[string]any{
				"type":     values.Hysteria2Obfuscation,
				"password": values.Hysteria2ObfuscationPassword,
			}
		}
		inbounds = append(inbounds, inbound)
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
	outbound := map[string]any{
		"type":        "hysteria2",
		"tag":         "proxy",
		"server":      values.ConnectHost,
		"server_port": values.UDPPort,
		"password":    values.Hysteria2Password,
		"tls": map[string]any{
			"enabled":     true,
			"server_name": values.Domain,
		},
	}
	if values.Hysteria2Obfuscation != "" {
		outbound["obfs"] = map[string]any{
			"type":     values.Hysteria2Obfuscation,
			"password": values.Hysteria2ObfuscationPassword,
		}
	}
	if values.Hysteria2PortHoppingEnabled {
		outbound["server_ports"] = []string{values.Hysteria2PortRange}
		outbound["hop_interval"] = fmt.Sprintf("%ds", values.Hysteria2HopIntervalSeconds)
		delete(outbound, "server_port")
	}
	configuration := clientBase(socksPort, outbound)
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

func ShareLinks(values model.RuntimeValues) []byte {
	realityName, hysteria2Name := protocolDisplayNames(values.Node)
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
			"vless://%s@%s?%s#%s",
			url.PathEscape(values.RealityUUID), net.JoinHostPort(values.ConnectHost, strconv.Itoa(values.TCPPort)), realityQuery.Encode(), url.PathEscape(realityName),
		))
	}
	if values.Hysteria2Enabled {
		hy2Query := url.Values{}
		hy2Query.Set("sni", values.Domain)
		if values.Hysteria2Obfuscation != "" {
			hy2Query.Set("obfs", values.Hysteria2Obfuscation)
			hy2Query.Set("obfs-password", values.Hysteria2ObfuscationPassword)
		}
		if values.Hysteria2PortHoppingEnabled {
			hy2Query.Set("ports", values.Hysteria2PortRange)
			hy2Query.Set("hop-interval", strconv.Itoa(values.Hysteria2HopIntervalSeconds))
		}
		links = append(links, fmt.Sprintf(
			"hysteria2://%s@%s?%s#%s",
			url.PathEscape(values.Hysteria2Password), net.JoinHostPort(values.ConnectHost, strconv.Itoa(values.UDPPort)), hy2Query.Encode(), url.PathEscape(hysteria2Name),
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
