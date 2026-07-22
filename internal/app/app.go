package app

import (
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/fsutil"
	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/platform"
	"vpskit.local/vpskit/internal/release"
	"vpskit.local/vpskit/internal/render"
)

const (
	configRoot                      = "/etc/vpskit"
	versionsLockPath                = "/etc/vpskit/versions.lock"
	generatedRoot                   = "/etc/vpskit/generated"
	exportRoot                      = "/etc/vpskit/exports"
	stateRoot                       = "/var/lib/vpskit"
	statePath                       = "/var/lib/vpskit/state.json"
	ownershipPath                   = "/var/lib/vpskit/ownership.json"
	secretRoot                      = "/var/lib/vpskit/secrets"
	secretPath                      = "/var/lib/vpskit/secrets/instances.json"
	cloudflareEnvPath               = "/var/lib/vpskit/secrets/cloudflare.env"
	acmeEnvPath                     = "/var/lib/vpskit/secrets/acme.env"
	certificateRoot                 = "/var/lib/vpskit/certificates"
	backupRoot                      = "/var/lib/vpskit/backups"
	transactionRoot                 = "/var/lib/vpskit/transactions"
	rulesCacheRoot                  = "/var/lib/vpskit/rules"
	hysteria2UDPBufferStateRoot     = "/var/lib/vpskit/hysteria2"
	hysteria2UDPBufferStatePath     = "/var/lib/vpskit/hysteria2/udp-buffer.json"
	hysteria2UDPBufferSysctlPath    = "/etc/sysctl.d/70-vpskit-hysteria2-udp-buffer.conf"
	legoStateRoot                   = "/var/lib/vpskit/lego"
	logRoot                         = "/var/log/vpskit"
	auditLogPath                    = "/var/log/vpskit/audit.jsonl"
	installedBinary                 = "/usr/local/bin/vpskit"
	installedSingBox                = "/usr/local/lib/vpskit/bin/sing-box"
	installedXray                   = "/usr/local/lib/vpskit/bin/xray"
	installedLego                   = "/usr/local/lib/vpskit/bin/lego"
	serviceUnitPath                 = "/etc/systemd/system/vpskit-sing-box.service"
	serviceUnitName                 = "vpskit-sing-box.service"
	xrayServiceUnitPath             = "/etc/systemd/system/vpskit-xray.service"
	xrayServiceUnitName             = "vpskit-xray.service"
	certificateRenewServiceUnitPath = "/etc/systemd/system/vpskit-certificate-renew.service"
	certificateRenewServiceUnitName = "vpskit-certificate-renew.service"
	certificateRenewTimerUnitPath   = "/etc/systemd/system/vpskit-certificate-renew.timer"
	certificateRenewTimerUnitName   = "vpskit-certificate-renew.timer"
	serverConfigPath                = "/etc/vpskit/generated/sing-box.json"
	xrayServerConfigPath            = "/etc/vpskit/generated/xray.json"
	managedCertificate              = "/var/lib/vpskit/certificates/hysteria2.crt"
	managedKey                      = "/var/lib/vpskit/certificates/hysteria2.key"
)

var (
	subscriptionConfigPath    = "/etc/vpskit/subscription.json"
	subscriptionSecretPath    = "/var/lib/vpskit/secrets/subscription.json"
	publicationStatePath      = "/var/lib/vpskit/subscription-state.json"
	subscriptionOwnershipPath = ownershipPath
	subscriptionLockPath      = "/var/lib/vpskit/locks/vpskit.lock"
)

type InstallOptions struct {
	BundleDir           string
	NodeID              string
	NodeName            string
	Provider            string
	Country             string
	City                string
	NodePriority        int
	Domain              string
	ConnectHost         string
	RealityTarget       string
	TCPPort             int
	UDPPort             int
	ExistingCertificate string
	ExistingKey         string
	PublicKeyBase64     string
	VPSKitVersion       string
}

type commandResult struct {
	Command string `json:"command"`
	Status  string `json:"status"`
	Detail  any    `json:"detail,omitempty"`
}

func Run(arguments []string, version, publicKeyBase64 string) error {
	if len(arguments) == 0 {
		return usageError()
	}
	if shouldAutoRecover(arguments[0]) && platform.IsRoot() {
		if _, err := recoverIncompleteMutation(); err != nil {
			return err
		}
	}
	switch arguments[0] {
	case "version":
		fmt.Printf("vpskit %s\n", version)
		return nil
	case "bundle":
		return runBundle(arguments[1:], publicKeyBase64)
	case "preflight":
		return runPreflight(arguments[1:])
	case "reality":
		return runReality(arguments[1:])
	case "cert":
		return runCertificate(arguments[1:])
	case "update":
		return runUpdate(arguments[1:], publicKeyBase64)
	case "backup":
		return runBackup(arguments[1:])
	case "restore":
		return runRestore(arguments[1:])
	case "rollback":
		return runRollback(arguments[1:])
	case "uninstall":
		return runUninstall(arguments[1:])
	case "orphan":
		return runOrphan(arguments[1:])
	case "recover":
		return runRecover(arguments[1:])
	case "install":
		return runInstall(arguments[1:], version, publicKeyBase64)
	case "instance":
		return runInstance(arguments[1:])
	case "node":
		return runNode(arguments[1:])
	case "migrate":
		return runMigrate(arguments[1:])
	case "cleanup":
		return runCleanup(arguments[1:])
	case "system":
		return runSystem(arguments[1:])
	case "security":
		return runSecurity(arguments[1:])
	case "rules":
		return runRules(arguments[1:])
	case "hysteria2":
		return runHysteria2(arguments[1:])
	case "support":
		return runSupport(arguments[1:])
	case "subscription":
		return runSubscription(arguments[1:])
	case "menu":
		return runMenu(arguments[1:], version, publicKeyBase64)
	case "status":
		return runStatus("status")
	case "doctor":
		return runDoctor(arguments[1:])
	case "export":
		return runExport(arguments[1:])
	default:
		return usageError()
	}
}

func usageError() error {
	return errors.New("usage: vpskit <version|bundle verify|reality scan|cert status|cert renew|update self|update core|backup|restore <backup-id> --yes|rollback <transaction-id> --yes|recover|orphan scan|uninstall --yes|preflight|install balanced|instance <enable|disable|modify|delete>|node <show|modify>|rules <show|plan|apply>|hysteria2 <inspect|salamander|performance|udp-buffer>|migrate <check|plan|apply>|cleanup <plan|apply>|system <inspect|updates>|security fail2ban <status|plan|apply|remove>|support bundle|subscription <plan|configure|publish|status|rotate-read-token|revoke-read-token|rollback|remove>|menu|status|doctor|export>")
}

func runBundle(arguments []string, publicKeyBase64 string) error {
	if len(arguments) == 0 || arguments[0] != "verify" {
		return errors.New("usage: vpskit bundle verify --dir <bundle-directory>")
	}
	flags := flag.NewFlagSet("bundle verify", flag.ContinueOnError)
	directory := flags.String("dir", "", "bundle directory")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if *directory == "" {
		return errors.New("--dir is required")
	}
	if publicKeyBase64 == "" {
		return errors.New("this build has no embedded release public key")
	}
	manifest, err := verifyBundle(*directory, publicKeyBase64)
	if err != nil {
		return err
	}
	return printJSON(commandResult{Command: "bundle verify", Status: "PASS", Detail: map[string]any{
		"release_id": manifest.ReleaseID,
		"assets":     len(manifest.Assets),
	}})
}

func runPreflight(arguments []string) error {
	flags := flag.NewFlagSet("preflight", flag.ContinueOnError)
	tcpPort := flags.Int("tcp-port", 443, "Reality TCP port")
	udpPort := flags.Int("udp-port", 443, "Hysteria2 UDP port")
	realityTarget := flags.String("reality-server-name", "", "Reality TLS target")
	singBoxPath := flags.String("sing-box", "", "sing-box binary used for an end-to-end Reality verification")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	resolvedSingBox := strings.TrimSpace(*singBoxPath)
	if resolvedSingBox == "" {
		if _, err := os.Stat(installedSingBox); err == nil {
			resolvedSingBox = installedSingBox
		}
	}
	report, err := collectPreflight(*tcpPort, *udpPort, *realityTarget, resolvedSingBox)
	if err != nil {
		return err
	}
	return printJSON(commandResult{Command: "preflight", Status: "PASS", Detail: report})
}

func runInstall(arguments []string, version, publicKeyBase64 string) error {
	if len(arguments) == 0 {
		return errors.New("usage: vpskit install <balanced|reality-only> [options]")
	}
	switch arguments[0] {
	case "balanced":
		flags := flag.NewFlagSet("install balanced", flag.ContinueOnError)
		bundleDir := flags.String("bundle-dir", "", "verified bundle directory")
		domain := flags.String("domain", "", "Hysteria2 certificate domain")
		connectHost := flags.String("connect-host", "", "client connection host")
		realityTarget := flags.String("reality-server-name", "", "Reality TLS target")
		tcpPort := flags.Int("tcp-port", 443, "Reality TCP port")
		udpPort := flags.Int("udp-port", 443, "Hysteria2 UDP port")
		nodeID := flags.String("node-id", "node-main", "stable node identifier")
		nodeName := flags.String("node-name", "VPSKit", "client display name prefix")
		provider := flags.String("provider", "", "optional VPS provider label")
		country := flags.String("country", "", "optional ISO country code")
		city := flags.String("city", "", "optional city label")
		nodePriority := flags.Int("node-priority", 100, "subscription node priority")
		existingCertificate := flags.String("existing-certificate", "", "existing trusted certificate PEM to import")
		existingKey := flags.String("existing-key", "", "private key PEM for --existing-certificate")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		options := InstallOptions{
			BundleDir:           *bundleDir,
			NodeID:              strings.ToLower(strings.TrimSpace(*nodeID)),
			NodeName:            strings.TrimSpace(*nodeName),
			Provider:            strings.TrimSpace(*provider),
			Country:             strings.ToUpper(strings.TrimSpace(*country)),
			City:                strings.TrimSpace(*city),
			NodePriority:        *nodePriority,
			Domain:              strings.ToLower(strings.TrimSpace(*domain)),
			ConnectHost:         strings.TrimSpace(*connectHost),
			RealityTarget:       strings.ToLower(strings.TrimSpace(*realityTarget)),
			TCPPort:             *tcpPort,
			UDPPort:             *udpPort,
			ExistingCertificate: strings.TrimSpace(*existingCertificate),
			ExistingKey:         strings.TrimSpace(*existingKey),
			PublicKeyBase64:     publicKeyBase64,
			VPSKitVersion:       version,
		}
		return installBalanced(options)
	case "reality-only":
		flags := flag.NewFlagSet("install reality-only", flag.ContinueOnError)
		bundleDir := flags.String("bundle-dir", "", "verified bundle directory")
		connectHost := flags.String("connect-host", "", "client connection host")
		realityTarget := flags.String("reality-server-name", "", "Reality TLS target")
		tcpPort := flags.Int("tcp-port", 443, "Reality TCP port")
		nodeID := flags.String("node-id", "node-main", "stable node identifier")
		nodeName := flags.String("node-name", "VPSKit", "client display name prefix")
		provider := flags.String("provider", "", "optional VPS provider label")
		country := flags.String("country", "", "optional ISO country code")
		city := flags.String("city", "", "optional city label")
		nodePriority := flags.Int("node-priority", 100, "subscription node priority")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		options := InstallOptions{
			BundleDir:       *bundleDir,
			NodeID:          strings.ToLower(strings.TrimSpace(*nodeID)),
			NodeName:        strings.TrimSpace(*nodeName),
			Provider:        strings.TrimSpace(*provider),
			Country:         strings.ToUpper(strings.TrimSpace(*country)),
			City:            strings.TrimSpace(*city),
			NodePriority:    *nodePriority,
			ConnectHost:     strings.TrimSpace(*connectHost),
			RealityTarget:   strings.ToLower(strings.TrimSpace(*realityTarget)),
			TCPPort:         *tcpPort,
			PublicKeyBase64: publicKeyBase64,
			VPSKitVersion:   version,
		}
		return installRealityOnly(options)
	default:
		return fmt.Errorf("unsupported install profile %q; use balanced or reality-only", arguments[0])
	}
}

func verifyBundle(directory, publicKeyBase64 string) (release.Manifest, error) {
	manifestPath := filepath.Join(directory, "release-manifest.json")
	signaturePath := filepath.Join(directory, "release-manifest.sig")
	return release.VerifyDirectory(directory, manifestPath, signaturePath, publicKeyBase64)
}

func installBalanced(options InstallOptions) (returnErr error) {
	if err := validateInstallOptions(options); err != nil {
		return err
	}
	if !platform.IsRoot() {
		return errors.New("install requires root privileges")
	}
	if options.PublicKeyBase64 == "" {
		return errors.New("this build has no embedded release public key")
	}
	if _, err := os.Stat(statePath); err == nil {
		return errors.New("VPSKit is already installed; refusing an initial-install overwrite")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect existing state: %w", err)
	}
	if _, err := os.Stat(serviceUnitPath); err == nil {
		return errors.New("an unmanaged vpskit-sing-box.service already exists")
	}
	if _, err := os.Stat(xrayServiceUnitPath); err == nil {
		return errors.New("an unmanaged vpskit-xray.service already exists")
	}
	manifest, err := verifyBundle(options.BundleDir, options.PublicKeyBase64)
	if err != nil {
		return fmt.Errorf("verify signed bundle: %w", err)
	}
	vpskitAsset, err := release.FindAsset(manifest, "vpskit")
	if err != nil {
		return err
	}
	singBoxAsset, err := release.FindAsset(manifest, "sing-box")
	if err != nil {
		return err
	}
	xrayAsset, err := release.FindAsset(manifest, "xray")
	if err != nil {
		return err
	}
	legoAsset, err := release.FindAsset(manifest, "lego")
	if err != nil {
		return err
	}
	versionsLockAsset, err := release.FindAsset(manifest, "versions-lock")
	if err != nil {
		return err
	}
	if _, err := validateVersionsLock(options.BundleDir, versionsLockAsset, singBoxAsset, xrayAsset, legoAsset); err != nil {
		return err
	}
	preflight, err := collectPreflight(options.TCPPort, options.UDPPort, options.RealityTarget, filepath.Join(options.BundleDir, singBoxAsset.File))
	if err != nil {
		return fmt.Errorf("preflight failed: %w", err)
	}
	token := strings.TrimSpace(os.Getenv("VPSKIT_CF_DNS_API_TOKEN"))
	if token == "" {
		return errors.New("VPSKIT_CF_DNS_API_TOKEN is required")
	}
	if strings.ContainsAny(token, "\r\n\x00") {
		return errors.New("cloudflare token contains invalid control characters")
	}
	processLock, err := platform.AcquireProcessLock(filepath.Join(stateRoot, "locks", "vpskit.lock"))
	if err != nil {
		return err
	}
	defer processLock.Close()

	transactionID := "TX-" + time.Now().UTC().Format("20060102-150405") + "-" + randomHex(3)
	transactionDirectory := filepath.Join(transactionRoot, transactionID)
	stagingDirectory := filepath.Join(transactionDirectory, "staging")
	stagingLego := filepath.Join(stagingDirectory, "lego")
	if err := os.MkdirAll(stagingDirectory, 0o700); err != nil {
		return fmt.Errorf("create transaction staging: %w", err)
	}
	_ = os.Chmod(transactionDirectory, 0o700)
	_ = os.Chmod(stagingDirectory, 0o700)
	committed := false
	userCreated := false
	systemMutated := false
	defer func() {
		if committed {
			return
		}
		_ = fsutil.RemoveManagedTree(stagingDirectory, transactionDirectory)
		rollbackError := rollbackInitialInstall(userCreated, systemMutated)
		failure := map[string]any{
			"schema_version": 1,
			"transaction_id": transactionID,
			"status":         "ROLLED_BACK",
			"failed_at":      time.Now().UTC(),
			"error":          sanitizeError(returnErr, token),
		}
		if rollbackError != nil {
			failure["status"] = "ROLLBACK_FAILED"
			failure["rollback_error"] = rollbackError.Error()
		}
		if bytes, marshalErr := json.MarshalIndent(failure, "", "  "); marshalErr == nil {
			_ = fsutil.WriteFileAtomic(filepath.Join(transactionDirectory, "transaction.json"), append(bytes, '\n'), 0o600)
		}
		if returnErr == nil && rollbackError != nil {
			returnErr = rollbackError
		}
	}()

	values, err := generateRuntimeValues(options, filepath.Join(stagingDirectory, "hysteria2.crt"), filepath.Join(stagingDirectory, "hysteria2.key"), filepath.Join(options.BundleDir, singBoxAsset.File))
	if err != nil {
		return err
	}
	acmeConfig, err := readACMEConfig()
	if err != nil {
		return err
	}
	acmeConfig, err = populateZeroSSLEAB(acmeConfig)
	if err != nil {
		return err
	}
	issuedCertificate := filepath.Join(stagingLego, "certificates", options.Domain+".crt")
	issuedKey := filepath.Join(stagingLego, "certificates", options.Domain+".key")
	certificateProvider := "acme-dns-01"
	if options.ExistingCertificate != "" {
		certificateProvider = "existing-files"
		if err := validateCertificate(options.ExistingCertificate, options.ExistingKey, options.Domain); err != nil {
			return fmt.Errorf("validate existing certificate: %w", err)
		}
		if err := os.MkdirAll(stagingLego, 0o700); err != nil {
			return err
		}
		if err := fsutil.CopyFileAtomic(options.ExistingCertificate, values.CertificatePath, 0o600); err != nil {
			return err
		}
		if err := fsutil.CopyFileAtomic(options.ExistingKey, values.KeyPath, 0o600); err != nil {
			return err
		}
	} else {
		if err := issueCertificate(filepath.Join(options.BundleDir, legoAsset.File), stagingLego, options.Domain, token, acmeConfig); err != nil {
			return err
		}
		if err := validateCertificate(issuedCertificate, issuedKey, options.Domain); err != nil {
			return err
		}
		if err := fsutil.CopyFileAtomic(issuedCertificate, values.CertificatePath, 0o600); err != nil {
			return err
		}
		if err := fsutil.CopyFileAtomic(issuedKey, values.KeyPath, 0o600); err != nil {
			return err
		}
	}
	serverConfig, err := render.ServerConfig(values)
	if err != nil {
		return err
	}
	stagingConfig := filepath.Join(stagingDirectory, "sing-box.json")
	if err := fsutil.WriteFileAtomic(stagingConfig, serverConfig, 0o600); err != nil {
		return err
	}
	if output, err := runCommand(filepath.Join(options.BundleDir, singBoxAsset.File), "check", "-c", stagingConfig); err != nil {
		return fmt.Errorf("sing-box staging check failed: %w: %s", err, sanitizeText(output, token))
	}
	xrayConfig, err := render.XrayRealityServerConfig(values)
	if err != nil {
		return err
	}
	stagingXrayConfig := filepath.Join(stagingDirectory, "xray.json")
	if err := fsutil.WriteFileAtomic(stagingXrayConfig, xrayConfig, 0o600); err != nil {
		return err
	}
	if output, err := runCommand(filepath.Join(options.BundleDir, xrayAsset.File), "run", "-test", "-config", stagingXrayConfig); err != nil {
		return fmt.Errorf("xray staging check failed: %w: %s", err, sanitizeText(output, token))
	}

	userCreated, err = ensureServiceUser()
	if err != nil {
		return err
	}
	systemMutated = true
	if err := installManagedFiles(options, manifest, vpskitAsset, singBoxAsset, xrayAsset, legoAsset, versionsLockAsset, values, serverConfig, xrayConfig, stagingLego, transactionID, acmeConfig); err != nil {
		return err
	}
	if output, err := runCommand(installedSingBox, "check", "-c", serverConfigPath); err != nil {
		return fmt.Errorf("installed sing-box check failed: %w: %s", err, output)
	}
	if output, err := runCommand(installedXray, "run", "-test", "-config", xrayServerConfigPath); err != nil {
		return fmt.Errorf("installed Xray check failed: %w: %s", err, output)
	}
	if err := startService(options.TCPPort, options.UDPPort); err != nil {
		return err
	}
	if err := fsutil.RemoveManagedTree(stagingDirectory, transactionDirectory); err != nil {
		return fmt.Errorf("remove sensitive transaction staging: %w", err)
	}

	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		return fmt.Errorf("read committed state: %w", err)
	}
	commitRecord := map[string]any{
		"schema_version":       1,
		"transaction_id":       transactionID,
		"status":               "COMMITTED",
		"committed_at":         time.Now().UTC(),
		"state_sha256":         sha256Bytes(stateBytes),
		"preflight":            preflight,
		"certificate_provider": certificateProvider,
	}
	commitBytes, err := json.MarshalIndent(commitRecord, "", "  ")
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(transactionDirectory, "transaction.json"), append(commitBytes, '\n'), 0o600); err != nil {
		return err
	}
	committed = true
	_ = appendAudit(map[string]any{
		"time":           time.Now().UTC(),
		"transaction_id": transactionID,
		"command":        "install balanced",
		"status":         "COMMITTED",
	})
	return printJSON(commandResult{Command: "install balanced", Status: "PASS", Detail: map[string]any{
		"transaction_id":       transactionID,
		"core_versions":        map[string]string{"sing-box": singBoxAsset.Version, "xray": xrayAsset.Version},
		"tcp_listener":         fmt.Sprintf("tcp/%d", options.TCPPort),
		"udp_listener":         fmt.Sprintf("udp/%d", options.UDPPort),
		"firewall":             "manual/noop",
		"exports":              exportRoot,
		"certificate_provider": certificateProvider,
	}})
}

func validateInstallOptions(options InstallOptions) error {
	if options.BundleDir == "" {
		return errors.New("--bundle-dir is required")
	}
	if !validDomain(options.Domain) {
		return fmt.Errorf("invalid certificate domain: %q", options.Domain)
	}
	if net.ParseIP(options.ConnectHost) == nil && !validDomain(strings.ToLower(options.ConnectHost)) {
		return fmt.Errorf("invalid connect host: %q", options.ConnectHost)
	}
	if net.ParseIP(options.RealityTarget) != nil || !validDomain(options.RealityTarget) {
		return fmt.Errorf("invalid Reality server name: %q", options.RealityTarget)
	}
	for name, port := range map[string]int{"tcp": options.TCPPort, "udp": options.UDPPort} {
		if port < 1 || port > 65535 {
			return fmt.Errorf("invalid %s port: %d", name, port)
		}
	}
	if (options.ExistingCertificate == "") != (options.ExistingKey == "") {
		return errors.New("--existing-certificate and --existing-key must be provided together")
	}
	if err := validateNodeMetadata(nodeMetadataFromOptions(options)); err != nil {
		return err
	}
	for name, path := range map[string]string{"existing certificate": options.ExistingCertificate, "existing key": options.ExistingKey} {
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			return fmt.Errorf("%s path must be absolute", name)
		}
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("inspect %s: %w", name, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("%s must be a regular non-symlink file", name)
		}
	}
	return nil
}

func collectPreflight(tcpPort, udpPort int, realityTarget, singBoxPath string) (map[string]any, error) {
	return collectProfilePreflight(true, tcpPort, true, udpPort, realityTarget, singBoxPath)
}

func collectProfilePreflight(realityEnabled bool, tcpPort int, hysteria2Enabled bool, udpPort int, realityTarget, singBoxPath string) (map[string]any, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return nil, fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}
	osRelease, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return nil, fmt.Errorf("read /etc/os-release: %w", err)
	}
	osText := string(osRelease)
	if !(strings.Contains(osText, "ID=debian") && (strings.Contains(osText, "VERSION_ID=\"12\"") || strings.Contains(osText, "VERSION_ID=\"13\""))) &&
		!(strings.Contains(osText, "ID=ubuntu") && strings.Contains(osText, "VERSION_ID=\"24.04\"")) {
		return nil, errors.New("operating system is outside the target matrix")
	}
	if _, err := runCommand("systemctl", "--version"); err != nil {
		return nil, fmt.Errorf("systemd preflight: %w", err)
	}
	if realityEnabled {
		tcpListener, err := net.Listen("tcp", fmt.Sprintf("[::]:%d", tcpPort))
		if err != nil {
			return nil, fmt.Errorf("TCP port %d is unavailable: %w", tcpPort, err)
		}
		_ = tcpListener.Close()
	}
	if hysteria2Enabled {
		udpAddress := &net.UDPAddr{IP: net.ParseIP("::"), Port: udpPort}
		udpListener, err := net.ListenUDP("udp", udpAddress)
		if err != nil {
			return nil, fmt.Errorf("UDP port %d is unavailable: %w", udpPort, err)
		}
		_ = udpListener.Close()
	}
	var realityScan *RealityScanResult
	if realityTarget != "" {
		if strings.TrimSpace(singBoxPath) == "" {
			return nil, errors.New("--sing-box is required to perform the Reality end-to-end preflight")
		}
		realityScan, err = preflightRealityTarget(realityTarget, singBoxPath)
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"os":                   strings.TrimSpace(extractOSPrettyName(osText)),
		"architecture":         runtime.GOARCH,
		"systemd":              true,
		"tcp_port_available":   map[bool]any{true: tcpPort, false: "disabled"}[realityEnabled],
		"udp_port_available":   map[bool]any{true: udpPort, false: "disabled"}[hysteria2Enabled],
		"reality_target":       realityTarget,
		"reality_scan":         realityScan,
		"firewall_provider":    "manual/noop",
		"cloud_security_group": "NEEDS_USER_CHECK",
	}, nil
}

func generateRuntimeValues(options InstallOptions, certificatePath, keyPath, singBoxPath string) (model.RuntimeValues, error) {
	keyOutput, err := runCommand(singBoxPath, "generate", "reality-keypair")
	if err != nil {
		return model.RuntimeValues{}, fmt.Errorf("generate Reality key pair: %w", err)
	}
	privateKey := extractPrefixedValue(keyOutput, "PrivateKey:")
	publicKey := extractPrefixedValue(keyOutput, "PublicKey:")
	if privateKey == "" || publicKey == "" {
		return model.RuntimeValues{}, errors.New("sing-box returned an incomplete Reality key pair")
	}
	if err := validateRealityKeyPair(privateKey, publicKey); err != nil {
		return model.RuntimeValues{}, fmt.Errorf("sing-box returned an invalid Reality key pair: %w", err)
	}
	return model.RuntimeValues{
		Node:              nodeMetadataFromOptions(options),
		ClientRevision:    1,
		RealityEnabled:    true,
		Hysteria2Enabled:  true,
		ConnectHost:       options.ConnectHost,
		Domain:            options.Domain,
		RealityServerName: options.RealityTarget,
		TCPPort:           options.TCPPort,
		UDPPort:           options.UDPPort,
		RealityUUID:       randomUUID(),
		RealityPrivateKey: privateKey,
		RealityPublicKey:  publicKey,
		RealityShortID:    randomHex(8),
		Hysteria2Password: randomBase64(24),
		CertificatePath:   certificatePath,
		KeyPath:           keyPath,
	}, nil
}

func validateRealityKeyPair(privateValue, publicValue string) error {
	privateBytes, err := base64.RawURLEncoding.DecodeString(privateValue)
	if err != nil {
		return fmt.Errorf("decode private key: %w", err)
	}
	publicBytes, err := base64.RawURLEncoding.DecodeString(publicValue)
	if err != nil {
		return fmt.Errorf("decode public key: %w", err)
	}
	curve := ecdh.X25519()
	privateKey, err := curve.NewPrivateKey(privateBytes)
	if err != nil {
		return fmt.Errorf("parse private key: %w", err)
	}
	publicKey, err := curve.NewPublicKey(publicBytes)
	if err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}
	if !privateKey.PublicKey().Equal(publicKey) {
		return errors.New("public key does not match private key")
	}
	return nil
}

func issueCertificate(legoPath, legoStatePath, domain, token string, acmeConfig acmeConfig) error {
	if err := os.MkdirAll(legoStatePath, 0o700); err != nil {
		return fmt.Errorf("create lego staging directory: %w", err)
	}
	arguments := acmeConfig.legoRunArguments(domain, legoStatePath)
	command := exec.Command(legoPath, arguments...)
	command.Env = acmeConfig.legoEnvironment(os.Environ(), token)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("lego DNS-01 issuance failed: %w: %s", err, sanitizeText(string(output), token))
	}
	return nil
}

func legoRunArguments(domain, legoStatePath string) []string {
	return acmeConfig{Server: defaultACMEServer}.legoRunArguments(domain, legoStatePath)
}

func validateCertificate(certificatePath, keyPath, domain string) error {
	pair, err := tls.LoadX509KeyPair(certificatePath, keyPath)
	if err != nil {
		return fmt.Errorf("load issued certificate pair: %w", err)
	}
	if len(pair.Certificate) == 0 {
		return errors.New("issued certificate has no leaf certificate")
	}
	certificate, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return fmt.Errorf("parse issued certificate: %w", err)
	}
	if err := certificate.VerifyHostname(domain); err != nil {
		return fmt.Errorf("issued certificate does not match %s: %w", domain, err)
	}
	if time.Until(certificate.NotAfter) < 24*time.Hour {
		return errors.New("issued certificate expires in less than 24 hours")
	}
	return nil
}

func ensureServiceUser() (bool, error) {
	if err := exec.Command("id", "-u", "vpskit").Run(); err == nil {
		return false, nil
	}
	output, err := runCommand("useradd", "--system", "--home", "/nonexistent", "--shell", "/usr/sbin/nologin", "vpskit")
	if err != nil {
		return false, fmt.Errorf("create vpskit service user: %w: %s", err, output)
	}
	return true, nil
}

func installManagedFiles(options InstallOptions, manifest release.Manifest, vpskitAsset, singBoxAsset, xrayAsset, legoAsset, versionsLockAsset release.Asset, values model.RuntimeValues, serverConfig, xrayConfig []byte, stagingLego, transactionID string, acmeConfig acmeConfig) error {
	directories := []struct {
		path string
		mode os.FileMode
	}{
		{configRoot, 0o750}, {generatedRoot, 0o750}, {exportRoot, 0o700},
		{stateRoot, 0o750}, {secretRoot, 0o700}, {certificateRoot, 0o750},
		{logRoot, 0o750}, {filepath.Dir(installedSingBox), 0o755},
	}
	for _, directory := range directories {
		if err := os.MkdirAll(directory.path, directory.mode); err != nil {
			return fmt.Errorf("create managed directory %s: %w", directory.path, err)
		}
		if err := os.Chmod(directory.path, directory.mode); err != nil {
			return err
		}
	}
	if err := fsutil.CopyFileAtomic(filepath.Join(options.BundleDir, vpskitAsset.File), installedBinary, 0o755); err != nil {
		return err
	}
	if err := fsutil.CopyFileAtomic(filepath.Join(options.BundleDir, singBoxAsset.File), installedSingBox, 0o755); err != nil {
		return err
	}
	if err := fsutil.CopyFileAtomic(filepath.Join(options.BundleDir, xrayAsset.File), installedXray, 0o755); err != nil {
		return err
	}
	if err := fsutil.CopyFileAtomic(filepath.Join(options.BundleDir, legoAsset.File), installedLego, 0o755); err != nil {
		return err
	}
	if err := fsutil.CopyFileAtomic(filepath.Join(options.BundleDir, versionsLockAsset.File), versionsLockPath, 0o644); err != nil {
		return err
	}
	if err := fsutil.CopyFileAtomic(values.CertificatePath, managedCertificate, 0o640); err != nil {
		return err
	}
	if err := fsutil.CopyFileAtomic(values.KeyPath, managedKey, 0o640); err != nil {
		return err
	}
	if err := fsutil.RemoveManagedTree(legoStateRoot, stateRoot); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(stagingLego, legoStateRoot); err != nil {
		return fmt.Errorf("activate lego state: %w", err)
	}
	_ = os.Chmod(legoStateRoot, 0o700)

	finalValues := values
	finalValues.CertificatePath = managedCertificate
	finalValues.KeyPath = managedKey
	finalServerConfig, err := render.ServerConfig(finalValues)
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(serverConfigPath, finalServerConfig, 0o640); err != nil {
		return err
	}
	finalXrayConfig, err := render.XrayRealityServerConfig(finalValues)
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(xrayServerConfigPath, finalXrayConfig, 0o640); err != nil {
		return err
	}
	clientSet, err := render.ClientArtifactSet(finalValues)
	if err != nil {
		return err
	}
	if err := publishStaticClientArtifacts(clientSet); err != nil {
		return err
	}
	secrets := model.Secrets{
		SchemaVersion:     model.SchemaVersion,
		RealityUUID:       finalValues.RealityUUID,
		RealityPrivateKey: finalValues.RealityPrivateKey,
		Hysteria2Password: finalValues.Hysteria2Password,
	}
	secretBytes, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(secretPath, append(secretBytes, '\n'), 0o600); err != nil {
		return err
	}
	token := strings.TrimSpace(os.Getenv("VPSKIT_CF_DNS_API_TOKEN"))
	if err := fsutil.WriteFileAtomic(cloudflareEnvPath, []byte("CF_DNS_API_TOKEN="+token+"\n"), 0o600); err != nil {
		return err
	}
	if err := writeACMEConfig(acmeConfig); err != nil {
		return err
	}
	configHash := sha256Bytes(finalServerConfig)
	realityConfigHash := sha256Bytes(finalXrayConfig)
	state := model.State{
		SchemaVersion:     model.SchemaVersion,
		VPSKitVersion:     options.VPSKitVersion,
		TransactionID:     transactionID,
		InstalledAt:       time.Now().UTC(),
		Profile:           "balanced",
		Node:              nodeMetadataFromOptions(options),
		ConnectHost:       finalValues.ConnectHost,
		Domain:            finalValues.Domain,
		RealityServerName: finalValues.RealityServerName,
		Core: model.CoreState{
			ID:           "sing-box",
			Version:      singBoxAsset.Version,
			Channel:      coreAssetChannel(singBoxAsset),
			Path:         installedSingBox,
			SHA256:       singBoxAsset.SHA256,
			SourceURL:    singBoxAsset.SourceURL,
			SourceRef:    singBoxAsset.SourceRef,
			SourceCommit: singBoxAsset.SourceCommit,
		},
		RealityCore: model.CoreState{
			ID:           "xray",
			Version:      xrayAsset.Version,
			Channel:      coreAssetChannel(xrayAsset),
			Path:         installedXray,
			SHA256:       xrayAsset.SHA256,
			SourceURL:    xrayAsset.SourceURL,
			SourceRef:    xrayAsset.SourceRef,
			SourceCommit: xrayAsset.SourceCommit,
		},
		Reality:             model.RealityState{Enabled: true, ID: "reality-main", ListenPort: finalValues.TCPPort, UUIDRef: "secret://reality-main/uuid", PrivateKeyRef: "secret://reality-main/private-key", PublicKey: finalValues.RealityPublicKey, ShortID: finalValues.RealityShortID},
		Hysteria2:           model.Hysteria2State{Enabled: true, ID: "hy2-backup", ListenPort: finalValues.UDPPort, PasswordRef: "secret://hy2-backup/password", CertificatePath: managedCertificate, KeyPath: managedKey, CertificateDNS: finalValues.Domain, CertificateAuthority: acmeConfig.Server},
		Firewall:            model.FirewallState{Provider: "manual/noop", Status: "no active local firewall detected during install"},
		ConfigSHA256:        configHash,
		RealityConfigSHA256: realityConfigHash,
		ConfigRevision:      1,
		Exports: []model.ExportState{
			{Format: "sing-box-reality", Path: filepath.Join(exportRoot, "sing-box-reality.json")},
			{Format: "sing-box-hysteria2", Path: filepath.Join(exportRoot, "sing-box-hysteria2.json")},
			{Format: "mihomo", Path: filepath.Join(exportRoot, "mihomo.yaml")},
			{Format: "share-link", Path: filepath.Join(exportRoot, "share-links.txt")},
		},
	}
	stateBytes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(statePath, append(stateBytes, '\n'), 0o600); err != nil {
		return err
	}
	ownership := map[string]any{
		"schema_version": 1,
		"transaction_id": transactionID,
		"files":          []string{installedBinary, installedSingBox, installedXray, installedLego, versionsLockPath, serviceUnitPath, xrayServiceUnitPath, certificateRenewServiceUnitPath, certificateRenewTimerUnitPath, serverConfigPath, xrayServerConfigPath, managedCertificate, managedKey, statePath, ownershipPath, secretPath, cloudflareEnvPath, acmeEnvPath},
		"directories":    []string{configRoot, exportRoot, stateRoot, secretRoot, certificateRoot, legoStateRoot, logRoot, "/usr/local/lib/vpskit"},
		"services":       []string{serviceUnitName, xrayServiceUnitName, certificateRenewServiceUnitName, certificateRenewTimerUnitName},
		"firewall_rules": []string{},
	}
	ownershipBytes, err := json.MarshalIndent(ownership, "", "  ")
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(ownershipPath, append(ownershipBytes, '\n'), 0o600); err != nil {
		return err
	}
	if err := setServiceFileOwnership(); err != nil {
		return err
	}
	if err := verifyServiceReadAccess(); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(serviceUnitPath, []byte(systemdUnit()), 0o644); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(xrayServiceUnitPath, []byte(xraySystemdUnit()), 0o644); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(certificateRenewServiceUnitPath, []byte(certificateRenewServiceUnit()), 0o644); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(certificateRenewTimerUnitPath, []byte(certificateRenewTimerUnit()), 0o644); err != nil {
		return err
	}
	_ = manifest
	_ = serverConfig
	_ = xrayConfig
	return nil
}

func setServiceFileOwnership() error {
	account, err := user.Lookup("vpskit")
	if err != nil {
		return fmt.Errorf("lookup vpskit user: %w", err)
	}
	uid, err := strconv.Atoi(account.Uid)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(account.Gid)
	if err != nil {
		return err
	}
	for _, path := range serviceGroupPaths() {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return fmt.Errorf("inspect service path %s: %w", path, err)
		}
		if err := os.Chown(path, 0, gid); err != nil {
			return fmt.Errorf("set service group on %s: %w", path, err)
		}
	}
	_ = uid
	return nil
}

func setServiceDirectoryModes() error {
	for _, directory := range []struct {
		path string
		mode os.FileMode
	}{
		{configRoot, 0o750},
		{generatedRoot, 0o750},
		{stateRoot, 0o750},
		{certificateRoot, 0o750},
	} {
		if _, err := os.Stat(directory.path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return fmt.Errorf("inspect service directory %s: %w", directory.path, err)
		}
		if err := os.Chmod(directory.path, directory.mode); err != nil {
			return fmt.Errorf("set service directory mode on %s: %w", directory.path, err)
		}
	}
	return nil
}

func serviceGroupPaths() []string {
	return []string{configRoot, generatedRoot, stateRoot, certificateRoot, serverConfigPath, xrayServerConfigPath, managedCertificate, managedKey}
}

func verifyServiceReadAccess() error {
	checks := []struct {
		mode string
		path string
	}{
		{"-x", configRoot},
		{"-x", generatedRoot},
		{"-x", stateRoot},
		{"-x", certificateRoot},
		{"-r", serverConfigPath},
		{"-r", xrayServerConfigPath},
		{"-r", managedCertificate},
		{"-r", managedKey},
	}
	for _, check := range checks {
		if _, err := os.Stat(check.path); errors.Is(err, os.ErrNotExist) && (check.path == certificateRoot || check.path == managedCertificate || check.path == managedKey) {
			continue
		} else if err != nil {
			return fmt.Errorf("inspect service access path %s: %w", check.path, err)
		}
		if output, err := runCommand("runuser", "-u", "vpskit", "--", "test", check.mode, check.path); err != nil {
			return fmt.Errorf("service user access check failed for %s: %w: %s", check.path, err, output)
		}
	}
	return nil
}

func systemdUnit() string {
	return `[Unit]
Description=VPSKit managed sing-box service
Documentation=https://sing-box.sagernet.org/
After=network-online.target nss-lookup.target
Wants=network-online.target

[Service]
Type=simple
User=vpskit
Group=vpskit
ExecStart=/usr/local/lib/vpskit/bin/sing-box run -c /etc/vpskit/generated/sing-box.json
Restart=on-failure
RestartSec=3s
LimitNOFILE=65535
NoNewPrivileges=true
PrivateTmp=true
PrivateDevices=true
ProtectHome=true
ProtectSystem=strict
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictSUIDSGID=true
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_BIND_SERVICE
MemoryAccounting=true
TasksMax=256

[Install]
WantedBy=multi-user.target
`
}

func xraySystemdUnit() string {
	return `[Unit]
Description=VPSKit managed Xray REALITY service
Documentation=https://github.com/XTLS/Xray-core
After=network-online.target nss-lookup.target
Wants=network-online.target

[Service]
Type=simple
User=vpskit
Group=vpskit
ExecStart=/usr/local/lib/vpskit/bin/xray run -config /etc/vpskit/generated/xray.json
Restart=on-failure
RestartSec=3s
LimitNOFILE=65535
NoNewPrivileges=true
PrivateTmp=true
PrivateDevices=true
ProtectHome=true
ProtectSystem=strict
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictSUIDSGID=true
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_BIND_SERVICE
MemoryAccounting=true
TasksMax=256

[Install]
WantedBy=multi-user.target
`
}

func startService(tcpPort, udpPort int) error {
	return startServiceProfile(true, tcpPort, true, udpPort, true)
}

func startServiceProfile(realityEnabled bool, tcpPort int, hysteria2Enabled bool, udpPort int, enableCertificateTimer bool) error {
	if output, err := runCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("systemd daemon-reload failed: %w: %s", err, output)
	}
	if err := reconcileManagedService(xrayServiceUnitName, realityEnabled); err != nil {
		return err
	}
	if err := reconcileManagedService(serviceUnitName, hysteria2Enabled); err != nil {
		return err
	}
	if enableCertificateTimer {
		if output, err := runCommand("systemctl", "enable", "--now", certificateRenewTimerUnitName); err != nil {
			return fmt.Errorf("certificate renewal timer start failed: %w: %s", err, output)
		}
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if healthCheckProfile(realityEnabled, tcpPort, hysteria2Enabled, udpPort) == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	xrayOutput, _ := runCommand("journalctl", "-u", xrayServiceUnitName, "-n", "20", "--no-pager")
	singBoxOutput, _ := runCommand("journalctl", "-u", serviceUnitName, "-n", "20", "--no-pager")
	return fmt.Errorf("service health check failed: xray=%s sing-box=%s", xrayOutput, singBoxOutput)
}

func reconcileManagedService(name string, enabled bool) error {
	if enabled {
		if output, err := runCommand("systemctl", "enable", "--now", name); err != nil {
			return fmt.Errorf("start %s: %w: %s", name, err, output)
		}
		return nil
	}
	if output, err := runCommand("systemctl", "disable", "--now", name); err != nil && !strings.Contains(output, "not loaded") && !strings.Contains(output, "does not exist") {
		return fmt.Errorf("stop disabled %s: %w: %s", name, err, output)
	}
	return nil
}

func restartManagedServices(state model.State) error {
	if output, err := runCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("systemd daemon-reload failed: %w: %s", err, output)
	}
	if err := restartManagedService(xrayServiceUnitName, state.Reality.Enabled); err != nil {
		return err
	}
	if err := restartManagedService(serviceUnitName, state.Hysteria2.Enabled); err != nil {
		return err
	}
	return nil
}

func restartManagedService(name string, enabled bool) error {
	for _, arguments := range managedServiceRestartSteps(name, enabled) {
		output, err := runCommand("systemctl", arguments...)
		if err == nil {
			continue
		}
		if !enabled && (strings.Contains(output, "not loaded") || strings.Contains(output, "does not exist")) {
			continue
		}
		return fmt.Errorf("apply service state for %s: %w: %s", name, err, output)
	}
	return nil
}

func managedServiceRestartSteps(name string, enabled bool) [][]string {
	if enabled {
		return [][]string{{"enable", name}, {"restart", name}}
	}
	return [][]string{{"disable", "--now", name}}
}

func healthCheckState(state model.State) error {
	return healthCheckProfile(state.Reality.Enabled, state.Reality.ListenPort, state.Hysteria2.Enabled, state.Hysteria2.ListenPort)
}

func healthCheckProfile(realityEnabled bool, tcpPort int, hysteria2Enabled bool, udpPort int) error {
	if realityEnabled {
		if err := exec.Command("systemctl", "is-active", "--quiet", xrayServiceUnitName).Run(); err != nil {
			return errors.New("xray REALITY service is not active")
		}
		tcpOutput, err := runCommand("ss", "-ltnH")
		if err != nil || !listenerOutputHasPort(tcpOutput, tcpPort) {
			return fmt.Errorf("tcp listener %d is missing", tcpPort)
		}
	}
	if hysteria2Enabled {
		if err := exec.Command("systemctl", "is-active", "--quiet", serviceUnitName).Run(); err != nil {
			return errors.New("sing-box Hysteria2 service is not active")
		}
		udpOutput, err := runCommand("ss", "-lunH")
		if err != nil || !listenerOutputHasPort(udpOutput, udpPort) {
			return fmt.Errorf("UDP listener %d is missing", udpPort)
		}
	}
	return nil
}

func waitForManagedServiceState(state model.State, timeout time.Duration) error {
	return waitForManagedServiceProfile(state.Reality.Enabled, state.Reality.ListenPort, state.Hysteria2.Enabled, state.Hysteria2.ListenPort, timeout)
}

func waitForManagedServiceProfile(realityEnabled bool, tcpPort int, hysteria2Enabled bool, udpPort int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := healthCheckProfile(realityEnabled, tcpPort, hysteria2Enabled, udpPort); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = errors.New("service health check timed out")
	}
	return lastErr
}

func rollbackInitialInstall(userCreated, systemMutated bool) error {
	var rollbackErrors []string
	if systemMutated {
		_, _ = runCommand("systemctl", "disable", "--now", certificateRenewTimerUnitName)
		_, _ = runCommand("systemctl", "disable", "--now", serviceUnitName)
		_, _ = runCommand("systemctl", "disable", "--now", xrayServiceUnitName)
		for _, path := range []string{serviceUnitPath, xrayServiceUnitPath, certificateRenewServiceUnitPath, certificateRenewTimerUnitPath, installedBinary} {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				rollbackErrors = append(rollbackErrors, err.Error())
			}
		}
		for _, target := range []struct {
			path string
			root string
		}{
			{configRoot, configRoot},
			{"/usr/local/lib/vpskit", "/usr/local/lib/vpskit"},
			{secretRoot, stateRoot},
			{certificateRoot, stateRoot},
			{legoStateRoot, stateRoot},
			{exportRoot, configRoot},
		} {
			if err := fsutil.RemoveManagedTree(target.path, target.root); err != nil {
				rollbackErrors = append(rollbackErrors, err.Error())
			}
		}
		_ = os.Remove(statePath)
		_ = os.Remove(ownershipPath)
		_ = os.Chown(stateRoot, 0, 0)
		_ = os.Chmod(stateRoot, 0o750)
		_, _ = runCommand("systemctl", "daemon-reload")
	}
	if userCreated {
		if output, err := runCommand("userdel", "vpskit"); err != nil {
			rollbackErrors = append(rollbackErrors, output)
		}
	}
	if len(rollbackErrors) > 0 {
		return errors.New(strings.Join(rollbackErrors, "; "))
	}
	return nil
}

func runStatus(command string) error {
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	singBoxActive := exec.Command("systemctl", "is-active", "--quiet", serviceUnitName).Run() == nil
	xrayActive := exec.Command("systemctl", "is-active", "--quiet", xrayServiceUnitName).Run() == nil
	actualConfigHash := ""
	if configBytes, readErr := os.ReadFile(serverConfigPath); readErr == nil {
		actualConfigHash = sha256Bytes(configBytes)
	}
	actualRealityConfigHash := ""
	if configBytes, readErr := os.ReadFile(xrayServerConfigPath); readErr == nil {
		actualRealityConfigHash = sha256Bytes(configBytes)
	}
	coreVersion, _ := runCommand(installedSingBox, "version")
	xrayVersion, _ := runCommand(installedXray, "version")
	serviceStateMatches := singBoxActive == state.Hysteria2.Enabled && xrayActive == state.Reality.Enabled
	detail := map[string]any{
		"transaction_id":               state.TransactionID,
		"state_schema":                 state.SchemaVersion,
		"profile":                      state.Profile,
		"node":                         state.Node,
		"config_revision":              state.ConfigRevision,
		"core_channel":                 state.Core.Channel,
		"service_state_matches":        serviceStateMatches,
		"sing_box_service_active":      singBoxActive,
		"xray_service_active":          xrayActive,
		"core_version_output":          firstLine(coreVersion),
		"reality_core_version_output":  firstLine(xrayVersion),
		"config_hash_expected":         state.ConfigSHA256,
		"config_hash_actual":           actualConfigHash,
		"config_hash_matches":          strings.EqualFold(state.ConfigSHA256, actualConfigHash),
		"reality_config_hash_expected": state.RealityConfigSHA256,
		"reality_config_hash_actual":   actualRealityConfigHash,
		"reality_config_hash_matches":  strings.EqualFold(state.RealityConfigSHA256, actualRealityConfigHash),
		"reality_enabled":              state.Reality.Enabled,
		"hysteria2_enabled":            state.Hysteria2.Enabled,
		"tcp_listener":                 state.Reality.Enabled && listenerPresent("tcp", state.Reality.ListenPort),
		"udp_listener":                 state.Hysteria2.Enabled && listenerPresent("udp", state.Hysteria2.ListenPort),
		"firewall_provider":            state.Firewall.Provider,
		"cloud_security_group":         "NEEDS_USER_CHECK",
	}
	doctorFailure := false
	incomplete, incompleteErr := findIncompleteMutations()
	incompleteIDs := make([]string, 0, len(incomplete))
	incompleteCoreIDs := make([]string, 0, len(incomplete))
	for _, transaction := range incomplete {
		incompleteIDs = append(incompleteIDs, transaction.ID)
		if transaction.Command == "update core" {
			incompleteCoreIDs = append(incompleteCoreIDs, transaction.ID)
		}
	}
	detail["incomplete_transactions"] = incompleteIDs
	detail["incomplete_core_transactions"] = incompleteCoreIDs
	if incompleteErr != nil {
		detail["incomplete_transaction_scan_error"] = incompleteErr.Error()
		doctorFailure = true
	}
	if command == "doctor" {
		resources, _ := runCommand("ps", "-C", "sing-box", "-C", "xray", "-o", "pid=,rss=,%cpu=,cmd=")
		detail["resources"] = strings.TrimSpace(resources)
		if state.Reality.Enabled {
			secretBytes, secretErr := os.ReadFile(secretPath)
			var secrets model.Secrets
			if secretErr == nil {
				secretErr = json.Unmarshal(secretBytes, &secrets)
			}
			if secretErr == nil {
				secretErr = validateRealityKeyPair(secrets.RealityPrivateKey, state.Reality.PublicKey)
			}
			detail["reality_keypair_matches"] = secretErr == nil
			if secretErr != nil {
				doctorFailure = true
			}
		}
		if state.Hysteria2.Enabled {
			certificate, certErr := os.ReadFile(managedCertificate)
			if certErr == nil {
				block, _ := parseFirstCertificate(certificate)
				if block != nil {
					detail["certificate_not_after"] = block.NotAfter.UTC()
					detail["certificate_dns_names"] = block.DNSNames
				}
			}
		}
		if err := healthCheckState(state); err != nil {
			detail["health_error"] = err.Error()
			doctorFailure = true
		}
	}
	status := "PASS"
	if !serviceStateMatches || !strings.EqualFold(state.ConfigSHA256, actualConfigHash) || !strings.EqualFold(state.RealityConfigSHA256, actualRealityConfigHash) || doctorFailure {
		status = "FAIL"
	}
	return printJSON(commandResult{Command: command, Status: status, Detail: detail})
}

func runExport(arguments []string) error {
	flags := flag.NewFlagSet("export", flag.ContinueOnError)
	format := flags.String("format", "all", "all|mihomo|sing-box|link|qr|bundle")
	outputDirectory := flags.String("output-dir", "", "existing absolute directory for --format bundle")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *format == "qr" {
		if *outputDirectory != "" {
			return errors.New("--output-dir is only valid with --format bundle")
		}
		return runQRExport()
	}
	if *format == "bundle" {
		return runClientBundleExport(*outputDirectory)
	}
	if *outputDirectory != "" {
		return errors.New("--output-dir is only valid with --format bundle")
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	singBoxPaths := make([]string, 0, 2)
	if state.Reality.Enabled {
		singBoxPaths = append(singBoxPaths, filepath.Join(exportRoot, "sing-box-reality.json"))
	}
	if state.Hysteria2.Enabled {
		singBoxPaths = append(singBoxPaths, filepath.Join(exportRoot, "sing-box-hysteria2.json"))
	}
	allPaths := append([]string{filepath.Join(exportRoot, "mihomo.yaml")}, singBoxPaths...)
	allPaths = append(allPaths, filepath.Join(exportRoot, "share-links.txt"))
	paths := map[string][]string{
		"all":      allPaths,
		"mihomo":   {filepath.Join(exportRoot, "mihomo.yaml")},
		"sing-box": singBoxPaths,
		"link":     {filepath.Join(exportRoot, "share-links.txt")},
	}
	selected, ok := paths[*format]
	if !ok {
		return fmt.Errorf("unsupported export format: %s", *format)
	}
	for _, path := range selected {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("export is unavailable: %s", path)
		}
	}
	return printJSON(commandResult{Command: "export", Status: "PASS", Detail: map[string]any{
		"files":           selected,
		"config_revision": state.ConfigRevision,
	}})
}

func validDomain(value string) bool {
	if len(value) == 0 || len(value) > 253 || strings.Contains(value, "..") {
		return false
	}
	labelPattern := regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
	for _, label := range strings.Split(strings.TrimSuffix(strings.ToLower(value), "."), ".") {
		if !labelPattern.MatchString(label) {
			return false
		}
	}
	return strings.Contains(value, ".")
}

func extractOSPrettyName(osRelease string) string {
	for _, line := range strings.Split(osRelease, "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		}
	}
	return "unknown"
}

func extractPrefixedValue(output, prefix string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), prefix))
		}
	}
	return ""
}

func randomUUID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

func randomHex(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}

func randomBase64(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes)
}

func runCommand(name string, arguments ...string) (string, error) {
	command := exec.Command(name, arguments...)
	output, err := command.CombinedOutput()
	return string(output), err
}

func appendWithoutKey(environment []string, key string) []string {
	prefix := key + "="
	result := make([]string, 0, len(environment))
	for _, value := range environment {
		if !strings.HasPrefix(value, prefix) {
			result = append(result, value)
		}
	}
	return result
}

func sanitizeText(text, secret string) string {
	if secret != "" {
		text = strings.ReplaceAll(text, secret, "[REDACTED]")
	}
	return strings.TrimSpace(text)
}

func sanitizeError(err error, secret string) string {
	if err == nil {
		return "unknown error"
	}
	return sanitizeText(err.Error(), secret)
}

func listenerOutputHasPort(output string, port int) bool {
	needle := ":" + strconv.Itoa(port)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		for _, field := range fields {
			if strings.HasSuffix(strings.TrimSuffix(field, "]"), needle) || strings.Contains(field, needle+" ") {
				return true
			}
		}
	}
	return strings.Contains(output, needle)
}

func listenerPresent(network string, port int) bool {
	arguments := []string{"-ltnH"}
	if network == "udp" {
		arguments = []string{"-lunH"}
	}
	output, err := runCommand("ss", arguments...)
	return err == nil && listenerOutputHasPort(output, port)
}

func sha256Bytes(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func appendAudit(entry any) error {
	if err := os.MkdirAll(logRoot, 0o750); err != nil {
		return err
	}
	bytes, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(auditLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(bytes, '\n')); err != nil {
		return err
	}
	return file.Sync()
}

func firstLine(value string) string {
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		return strings.TrimSpace(value[:index])
	}
	return strings.TrimSpace(value)
}

func parseFirstCertificate(pemBytes []byte) (*x509.Certificate, error) {
	for len(pemBytes) > 0 {
		block, rest := pem.Decode(pemBytes)
		if block == nil {
			return nil, errors.New("no certificate PEM block found")
		}
		if block.Type == "CERTIFICATE" {
			return x509.ParseCertificate(block.Bytes)
		}
		pemBytes = rest
	}
	return nil, errors.New("no certificate PEM block found")
}
