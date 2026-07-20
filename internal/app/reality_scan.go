package app

import (
	"crypto/ecdh"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	realityTLSScanTimeout = 10 * time.Second
	realityReadyTimeout   = 5 * time.Second
)

var defaultRealityScanTargets = []string{
	"www.amazon.com:443",
	"aws.amazon.com:443",
	"www.cloudflare.com:443",
	"dl.google.com:443",
	"www.samsung.com:443",
	"www.nvidia.com:443",
	"www.amd.com:443",
	"www.intel.com:443",
	"www.sony.com:443",
	"www.microsoft.com:443",
}

// RealityScanResult deliberately separates fast TLS candidate discovery from
// a real sing-box Reality handshake. TLS characteristics alone produced a
// false positive for www.microsoft.com during the first VPS deployment.
type RealityScanResult struct {
	Target              string   `json:"target"`
	Host                string   `json:"host"`
	IP                  string   `json:"ip,omitempty"`
	Port                int      `json:"port"`
	TLSCandidate        bool     `json:"tls_candidate"`
	RealityVerified     bool     `json:"reality_verified"`
	TLSVersion          string   `json:"tls_version,omitempty"`
	ALPN                string   `json:"alpn,omitempty"`
	KeyExchange         string   `json:"key_exchange,omitempty"`
	CertificateValid    bool     `json:"certificate_valid"`
	CertificateSubject  string   `json:"certificate_subject,omitempty"`
	CertificateIssuer   string   `json:"certificate_issuer,omitempty"`
	CertificateNotAfter string   `json:"certificate_not_after,omitempty"`
	ServerNames         []string `json:"server_names,omitempty"`
	LatencyMS           int64    `json:"latency_ms,omitempty"`
	TLSReason           string   `json:"tls_reason,omitempty"`
	RealityReason       string   `json:"reality_reason,omitempty"`
}

func runReality(arguments []string) error {
	if len(arguments) == 0 || arguments[0] != "scan" {
		return errors.New("usage: vpskit reality scan [--targets <domain[:port],...>] [--sing-box <path>] [--tls-only]")
	}
	flags := flag.NewFlagSet("reality scan", flag.ContinueOnError)
	targets := flags.String("targets", "", "comma-separated public domain candidates; built-in candidates are used when empty")
	singBoxPath := flags.String("sing-box", installedSingBox, "sing-box binary used for end-to-end Reality verification")
	tlsOnly := flags.Bool("tls-only", false, "only perform the fast TLS candidate scan")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	resolvedBinary := strings.TrimSpace(*singBoxPath)
	if !*tlsOnly {
		info, err := os.Stat(resolvedBinary)
		if err != nil {
			return fmt.Errorf("inspect sing-box binary: %w", err)
		}
		if info.IsDir() {
			return errors.New("sing-box path points to a directory")
		}
	}
	results, err := scanRealityTargets(*targets, resolvedBinary, !*tlsOnly)
	if err != nil {
		return err
	}
	verified := 0
	for _, result := range results {
		if result.RealityVerified {
			verified++
		}
	}
	return printJSON(commandResult{Command: "reality scan", Status: "PASS", Detail: map[string]any{
		"verification_mode": map[bool]string{true: "tls-only", false: "tls+sing-box-reality"}[*tlsOnly],
		"verified":          verified,
		"results":           results,
	}})
}

func scanRealityTargets(targetsCSV, singBoxPath string, verifyReality bool) ([]*RealityScanResult, error) {
	targets := make([]string, 0, len(defaultRealityScanTargets))
	seen := make(map[string]bool)
	if strings.TrimSpace(targetsCSV) == "" {
		targets = append(targets, defaultRealityScanTargets...)
	} else {
		for _, raw := range strings.Split(targetsCSV, ",") {
			if value := strings.TrimSpace(raw); value != "" {
				targets = append(targets, value)
			}
		}
	}
	if len(targets) == 0 {
		return nil, errors.New("no Reality scan targets were provided")
	}
	if len(targets) > 32 {
		return nil, errors.New("at most 32 explicit Reality domain targets may be scanned at once")
	}

	results := make([]*RealityScanResult, 0, len(targets))
	for _, target := range targets {
		host, port, err := splitRealityDomainTarget(target)
		if err != nil {
			return nil, err
		}
		normalized := net.JoinHostPort(host, strconv.Itoa(port))
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		result := scanRealityTLS(host, port)
		if verifyReality && result.TLSCandidate {
			if err := verifyRealityLoopback(singBoxPath, host, port); err != nil {
				result.RealityReason = err.Error()
			} else {
				result.RealityVerified = true
			}
		}
		results = append(results, result)
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].RealityVerified != results[j].RealityVerified {
			return results[i].RealityVerified
		}
		if results[i].TLSCandidate != results[j].TLSCandidate {
			return results[i].TLSCandidate
		}
		if results[i].LatencyMS == results[j].LatencyMS {
			return results[i].Target < results[j].Target
		}
		return results[i].LatencyMS < results[j].LatencyMS
	})
	return results, nil
}

func splitRealityDomainTarget(target string) (string, int, error) {
	value := strings.ToLower(strings.TrimSpace(target))
	if value == "" {
		return "", 0, errors.New("reality target is required")
	}
	host := value
	port := 443
	if strings.Contains(value, ":") {
		parsedHost, parsedPort, err := net.SplitHostPort(value)
		if err != nil {
			return "", 0, fmt.Errorf("invalid Reality target %q: %w", target, err)
		}
		host = strings.ToLower(strings.TrimSpace(parsedHost))
		valuePort, err := strconv.Atoi(parsedPort)
		if err != nil || valuePort < 1 || valuePort > 65535 {
			return "", 0, fmt.Errorf("invalid Reality target port in %q", target)
		}
		port = valuePort
	}
	if net.ParseIP(host) != nil || !validDomain(host) {
		return "", 0, fmt.Errorf("invalid public Reality target domain: %q", host)
	}
	return host, port, nil
}

func scanRealityTLS(host string, port int) *RealityScanResult {
	result := &RealityScanResult{Target: net.JoinHostPort(host, strconv.Itoa(port)), Host: host, Port: port}
	dialer := &net.Dialer{Timeout: realityTLSScanTimeout}
	started := time.Now()
	connection, err := tls.DialWithDialer(dialer, "tcp", result.Target, &tls.Config{
		ServerName:       host,
		MinVersion:       tls.VersionTLS12,
		NextProtos:       []string{"h2", "http/1.1"},
		CurvePreferences: []tls.CurveID{tls.X25519, tls.X25519MLKEM768},
	})
	if err != nil {
		result.TLSReason = "TLS handshake or certificate validation failed: " + err.Error()
		return result
	}
	defer connection.Close()
	result.LatencyMS = time.Since(started).Milliseconds()
	if remoteHost, _, err := net.SplitHostPort(connection.RemoteAddr().String()); err == nil {
		result.IP = remoteHost
	}
	state := connection.ConnectionState()
	result.TLSVersion = tlsVersionName(state.Version)
	result.ALPN = state.NegotiatedProtocol
	result.KeyExchange = realityKeyExchangeName(state.CurveID)
	result.CertificateValid = len(state.VerifiedChains) > 0
	if len(state.PeerCertificates) > 0 {
		leaf := state.PeerCertificates[0]
		result.CertificateSubject = leaf.Subject.CommonName
		if len(leaf.Issuer.Organization) > 0 {
			result.CertificateIssuer = leaf.Issuer.Organization[0]
		} else {
			result.CertificateIssuer = leaf.Issuer.CommonName
		}
		result.CertificateNotAfter = leaf.NotAfter.UTC().Format(time.RFC3339)
		result.ServerNames = filterRealityServerNames(leaf.DNSNames)
	}
	result.TLSCandidate = isRealityTLSCandidate(state.Version, state.NegotiatedProtocol, state.CurveID, result.CertificateValid)
	if !result.TLSCandidate {
		switch {
		case state.Version != tls.VersionTLS13:
			result.TLSReason = "target did not negotiate TLS 1.3"
		case state.NegotiatedProtocol != "h2":
			result.TLSReason = "target did not negotiate HTTP/2 (h2)"
		case state.CurveID != tls.X25519 && state.CurveID != tls.X25519MLKEM768:
			result.TLSReason = "target did not negotiate X25519 or X25519MLKEM768"
		case !result.CertificateValid:
			result.TLSReason = "target certificate is not trusted"
		}
	}
	return result
}

func isRealityTLSCandidate(version uint16, alpn string, curve tls.CurveID, certificateValid bool) bool {
	return version == tls.VersionTLS13 &&
		alpn == "h2" &&
		(curve == tls.X25519 || curve == tls.X25519MLKEM768) &&
		certificateValid
}

func tlsVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS13:
		return "1.3"
	case tls.VersionTLS12:
		return "1.2"
	case tls.VersionTLS11:
		return "1.1"
	case tls.VersionTLS10:
		return "1.0"
	default:
		return "unknown"
	}
}

func realityKeyExchangeName(curve tls.CurveID) string {
	switch curve {
	case tls.X25519:
		return "X25519"
	case tls.X25519MLKEM768:
		return "X25519MLKEM768"
	case tls.CurveP256:
		return "P-256"
	case tls.CurveP384:
		return "P-384"
	case tls.CurveP521:
		return "P-521"
	default:
		return fmt.Sprintf("0x%04x", uint16(curve))
	}
}

func filterRealityServerNames(names []string) []string {
	result := make([]string, 0, len(names))
	for _, name := range names {
		value := strings.ToLower(strings.TrimSpace(name))
		if value == "" || strings.HasPrefix(value, "*.") || !validDomain(value) {
			continue
		}
		result = append(result, value)
	}
	return result
}

func preflightRealityTarget(serverName, singBoxPath string) (*RealityScanResult, error) {
	host, port, err := splitRealityDomainTarget(serverName)
	if err != nil {
		return nil, err
	}
	result := scanRealityTLS(host, port)
	if !result.TLSCandidate {
		return result, fmt.Errorf("reality target TLS candidate preflight failed for %s: %s", host, result.TLSReason)
	}
	if err := verifyRealityLoopback(singBoxPath, host, port); err != nil {
		result.RealityReason = err.Error()
		return result, fmt.Errorf("reality end-to-end preflight failed for %s: %w", host, err)
	}
	result.RealityVerified = true
	return result, nil
}

func verifyRealityLoopback(singBoxPath, host string, targetPort int) error {
	if strings.TrimSpace(singBoxPath) == "" {
		return errors.New("sing-box binary path is empty")
	}
	info, err := os.Stat(singBoxPath)
	if err != nil {
		return fmt.Errorf("inspect sing-box binary: %w", err)
	}
	if info.IsDir() {
		return errors.New("sing-box path points to a directory")
	}

	root, err := os.MkdirTemp("", "vpskit-reality-verify-")
	if err != nil {
		return fmt.Errorf("create Reality verification directory: %w", err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		_ = os.RemoveAll(root)
		return fmt.Errorf("secure Reality verification directory: %w", err)
	}
	defer os.RemoveAll(root)

	serverPort, err := reserveLoopbackPort()
	if err != nil {
		return err
	}
	clientPort, err := reserveLoopbackPort()
	if err != nil {
		return err
	}
	for clientPort == serverPort {
		clientPort, err = reserveLoopbackPort()
		if err != nil {
			return err
		}
	}
	serverConfig, clientConfig, err := buildRealityVerificationConfigs(host, targetPort, serverPort, clientPort)
	if err != nil {
		return err
	}
	serverPath := filepath.Join(root, "server.json")
	clientPath := filepath.Join(root, "client.json")
	if err := writePrivateJSON(serverPath, serverConfig); err != nil {
		return err
	}
	if err := writePrivateJSON(clientPath, clientConfig); err != nil {
		return err
	}
	if output, err := runCommand(singBoxPath, "check", "-c", serverPath); err != nil {
		return fmt.Errorf("temporary Reality server config check failed: %w: %s", err, strings.TrimSpace(output))
	}
	if output, err := runCommand(singBoxPath, "check", "-c", clientPath); err != nil {
		return fmt.Errorf("temporary Reality client config check failed: %w: %s", err, strings.TrimSpace(output))
	}

	server, err := startManagedRealityProcess(singBoxPath, serverPath, filepath.Join(root, "server.log"))
	if err != nil {
		return fmt.Errorf("start temporary Reality server: %w", err)
	}
	defer server.stop()
	if err := waitForLoopbackPort(serverPort, realityReadyTimeout); err != nil {
		return fmt.Errorf("temporary Reality server did not become ready: %w", err)
	}

	client, err := startManagedRealityProcess(singBoxPath, clientPath, filepath.Join(root, "client.log"))
	if err != nil {
		return fmt.Errorf("start temporary Reality client: %w", err)
	}
	defer client.stop()
	if err := waitForLoopbackPort(clientPort, realityReadyTimeout); err != nil {
		return fmt.Errorf("temporary Reality client did not become ready: %w", err)
	}
	if err := probeHTTPSViaMixedProxy(clientPort); err != nil {
		return fmt.Errorf("proxy request through the temporary Reality tunnel failed: %w", err)
	}
	return nil
}

func buildRealityVerificationConfigs(host string, targetPort, serverPort, clientPort int) (map[string]any, map[string]any, error) {
	privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate temporary Reality key pair: %w", err)
	}
	privateValue := base64.RawURLEncoding.EncodeToString(privateKey.Bytes())
	publicValue := base64.RawURLEncoding.EncodeToString(privateKey.PublicKey().Bytes())
	uuid := randomUUID()
	shortID := randomHex(8)
	server := map[string]any{
		"log": map[string]any{"level": "warn"},
		"inbounds": []any{map[string]any{
			"type": "vless", "tag": "reality-verify-server", "listen": "127.0.0.1", "listen_port": serverPort,
			"users": []any{map[string]any{"uuid": uuid, "flow": "xtls-rprx-vision"}},
			"tls": map[string]any{
				"enabled": true, "server_name": host,
				"reality": map[string]any{
					"enabled": true, "handshake": map[string]any{"server": host, "server_port": targetPort},
					"private_key": privateValue, "short_id": []string{shortID}, "max_time_difference": "1m",
				},
			},
		}},
		"outbounds": []any{map[string]any{"type": "direct", "tag": "direct"}},
		"route":     map[string]any{"final": "direct"},
	}
	client := map[string]any{
		"log":      map[string]any{"level": "warn"},
		"inbounds": []any{map[string]any{"type": "mixed", "listen": "127.0.0.1", "listen_port": clientPort}},
		"outbounds": []any{
			map[string]any{
				"type": "vless", "tag": "reality-verify-client", "server": "127.0.0.1", "server_port": serverPort,
				"uuid": uuid, "flow": "xtls-rprx-vision", "network": "tcp",
				"tls": map[string]any{
					"enabled": true, "server_name": host,
					"utls":    map[string]any{"enabled": true, "fingerprint": "chrome"},
					"reality": map[string]any{"enabled": true, "public_key": publicValue, "short_id": shortID},
				},
			},
			map[string]any{"type": "direct", "tag": "direct"},
		},
		"route": map[string]any{"final": "reality-verify-client"},
	}
	return server, client, nil
}

func writePrivateJSON(path string, value any) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("render temporary Reality config: %w", err)
	}
	content = append(content, '\n')
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("write temporary Reality config: %w", err)
	}
	return os.Chmod(path, 0o600)
}

func reserveLoopbackPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("reserve temporary loopback port: %w", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

type managedRealityProcess struct {
	command *exec.Cmd
	done    chan error
}

func startManagedRealityProcess(singBoxPath, configPath, logPath string) (*managedRealityProcess, error) {
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	command := exec.Command(singBoxPath, "run", "-c", configPath)
	command.Stdout = logFile
	command.Stderr = logFile
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		return nil, err
	}
	_ = logFile.Close()
	process := &managedRealityProcess{command: command, done: make(chan error, 1)}
	go func() {
		process.done <- command.Wait()
	}()
	return process, nil
}

func (process *managedRealityProcess) stop() {
	if process == nil || process.command == nil || process.command.Process == nil {
		return
	}
	select {
	case <-process.done:
		return
	default:
	}
	_ = process.command.Process.Signal(os.Interrupt)
	select {
	case <-process.done:
	case <-time.After(2 * time.Second):
		_ = process.command.Process.Kill()
		<-process.done
	}
}

func waitForLoopbackPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", address, 150*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("listener readiness timeout")
}

func probeHTTPSViaMixedProxy(port int) error {
	proxyURL := &url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(port))}
	transport := &http.Transport{
		Proxy:               http.ProxyURL(proxyURL),
		DisableKeepAlives:   true,
		TLSHandshakeTimeout: 8 * time.Second,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 12 * time.Second}
	probeURLs := []string{"https://api.ipify.org", "https://www.cloudflare.com/cdn-cgi/trace"}
	errorsSeen := make([]string, 0, len(probeURLs))
	for _, probeURL := range probeURLs {
		response, err := client.Get(probeURL)
		if err != nil {
			errorsSeen = append(errorsSeen, err.Error())
			continue
		}
		_, _ = io.CopyN(io.Discard, response.Body, 1)
		_ = response.Body.Close()
		if response.StatusCode >= 200 && response.StatusCode < 400 {
			return nil
		}
		errorsSeen = append(errorsSeen, response.Status)
	}
	return errors.New(strings.Join(errorsSeen, "; "))
}
