package app

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/fsutil"
	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/platform"
)

const certificateRenewalWindow = 30 * 24 * time.Hour

type certificateDetails struct {
	Domain          string
	CertificatePath string
	KeyPath         string
	NotBefore       time.Time
	NotAfter        time.Time
	DaysRemaining   int64
	RenewalDue      bool
	CertificateHash string
}

func runCertificate(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: vpskit cert <status|renew>")
	}
	switch arguments[0] {
	case "status":
		if len(arguments) != 1 {
			return errors.New("usage: vpskit cert status")
		}
		return certificateStatus()
	case "renew":
		flags := flag.NewFlagSet("cert renew", flag.ContinueOnError)
		force := flags.Bool("force", false, "force an ACME renewal even when the certificate is outside the renewal window")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		return renewCertificate(*force)
	default:
		return errors.New("usage: vpskit cert <status|renew>")
	}
}

func certificateStatus() error {
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	if state.Hysteria2.ID == "" {
		return errors.New("certificate lifecycle is unavailable because no Hysteria2 instance exists")
	}
	if err := validateManagedCertificateState(state); err != nil {
		return err
	}
	details, err := readCertificateDetails(state.Domain)
	if err != nil {
		return err
	}
	timerActive := exec.Command("systemctl", "is-active", "--quiet", certificateRenewTimerUnitName).Run() == nil
	return printJSON(commandResult{Command: "cert status", Status: "PASS", Detail: map[string]any{
		"domain":               details.Domain,
		"certificate_path":     details.CertificatePath,
		"key_path":             details.KeyPath,
		"not_before":           details.NotBefore.UTC(),
		"not_after":            details.NotAfter.UTC(),
		"days_remaining":       details.DaysRemaining,
		"renewal_window_days":  int(certificateRenewalWindow / (24 * time.Hour)),
		"renewal_due":          details.RenewalDue,
		"certificate_sha256":   details.CertificateHash,
		"renewal_timer_active": timerActive,
		"service_unit":         certificateRenewServiceUnitName,
		"renewal_timer_unit":   certificateRenewTimerUnitName,
	}})
}

func renewCertificate(force bool) (returnErr error) {
	if !platform.IsRoot() {
		return errors.New("certificate renewal requires root privileges")
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	if state.Hysteria2.ID == "" {
		return errors.New("certificate lifecycle is unavailable because no Hysteria2 instance exists")
	}
	if err := validateManagedCertificateState(state); err != nil {
		return err
	}
	details, err := readCertificateDetails(state.Domain)
	if err != nil {
		return err
	}
	if !force && !details.RenewalDue {
		return printJSON(commandResult{Command: "cert renew", Status: "PASS", Detail: map[string]any{
			"result":              "SKIPPED",
			"domain":              details.Domain,
			"not_after":           details.NotAfter.UTC(),
			"days_remaining":      details.DaysRemaining,
			"renewal_window_days": int(certificateRenewalWindow / (24 * time.Hour)),
		}})
	}
	token, err := readCloudflareToken()
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
	if acmeConfig.Server == zeroSSLACMEServer && acmeConfig.EABKID != "" {
		if err := writeACMEConfig(acmeConfig); err != nil {
			return fmt.Errorf("persist ZeroSSL EAB configuration: %w", err)
		}
	}
	lock, err := platform.AcquireProcessLock(filepath.Join(stateRoot, "locks", "vpskit.lock"))
	if err != nil {
		return err
	}
	defer lock.Close()

	transactionID := "TX-" + time.Now().UTC().Format("20060102-150405") + "-cert-renew-" + randomHex(3)
	transactionDirectory := filepath.Join(transactionRoot, transactionID)
	stagingDirectory := filepath.Join(transactionDirectory, "staging")
	stagingLego := filepath.Join(stagingDirectory, "lego")
	backupDirectory := filepath.Join(transactionDirectory, "backup")
	backupCertificate := filepath.Join(backupDirectory, "hysteria2.crt")
	backupKey := filepath.Join(backupDirectory, "hysteria2.key")
	backupLego := filepath.Join(backupDirectory, "lego")
	if err := os.MkdirAll(stagingLego, 0o700); err != nil {
		return fmt.Errorf("create certificate renewal staging: %w", err)
	}
	if err := os.MkdirAll(backupDirectory, 0o700); err != nil {
		return fmt.Errorf("create certificate renewal backup: %w", err)
	}
	_ = os.Chmod(transactionDirectory, 0o700)
	_ = os.Chmod(stagingDirectory, 0o700)
	_ = os.Chmod(backupDirectory, 0o700)
	if err := fsutil.CopyFileAtomic(managedCertificate, backupCertificate, 0o640); err != nil {
		return err
	}
	if err := fsutil.CopyFileAtomic(managedKey, backupKey, 0o640); err != nil {
		return err
	}
	if err := copyManagedTree(legoStateRoot, stagingLego); err != nil {
		return err
	}

	activated := false
	restartAttempted := false
	committed := false
	defer func() {
		if committed {
			return
		}
		rollbackError := rollbackCertificateRenewal(activated, restartAttempted, backupCertificate, backupKey, backupLego)
		status := "ROLLED_BACK"
		if rollbackError != nil {
			status = "ROLLBACK_FAILED"
		}
		record := map[string]any{
			"schema_version": 1,
			"transaction_id": transactionID,
			"command":        "cert renew",
			"status":         status,
			"failed_at":      time.Now().UTC(),
			"domain":         state.Domain,
			"error":          sanitizeError(returnErr, token),
		}
		if rollbackError != nil {
			record["rollback_error"] = sanitizeError(rollbackError, token)
		}
		_ = writeTransactionRecord(transactionDirectory, record)
		_ = fsutil.RemoveManagedTree(stagingDirectory, transactionDirectory)
		_ = fsutil.RemoveManagedTree(backupDirectory, transactionDirectory)
		if returnErr == nil && rollbackError != nil {
			returnErr = rollbackError
		}
	}()

	if err := runLegoRenewal(state.Domain, stagingLego, token, acmeConfig, force); err != nil {
		return err
	}
	issuedCertificate := filepath.Join(stagingLego, "certificates", state.Domain+".crt")
	issuedKey := filepath.Join(stagingLego, "certificates", state.Domain+".key")
	if err := validateCertificate(issuedCertificate, issuedKey, state.Domain); err != nil {
		return fmt.Errorf("renewed certificate validation failed: %w", err)
	}
	activated = true
	if err := activateCertificateRenewal(transactionID, issuedCertificate, issuedKey, stagingLego, backupLego); err != nil {
		return err
	}
	if err := setManagedCertificateOwnership(); err != nil {
		return fmt.Errorf("restore managed certificate ownership after activation: %w", err)
	}
	if output, err := runCommand(installedSingBox, "check", "-c", serverConfigPath); err != nil {
		return fmt.Errorf("sing-box validation after certificate renewal failed: %w: %s", err, sanitizeText(output, token))
	}
	restartAttempted = true
	if output, err := runCommand("systemctl", "restart", serviceUnitName); err != nil {
		return fmt.Errorf("restart after certificate renewal failed: %w: %s", err, sanitizeText(output, token))
	}
	if err := waitForManagedServiceState(state, 30*time.Second); err != nil {
		return fmt.Errorf("health check after certificate renewal failed: %w", err)
	}
	newDetails, err := readCertificateDetails(state.Domain)
	if err != nil {
		return err
	}
	record := map[string]any{
		"schema_version":     1,
		"transaction_id":     transactionID,
		"command":            "cert renew",
		"status":             "COMMITTED",
		"committed_at":       time.Now().UTC(),
		"domain":             state.Domain,
		"certificate_sha256": newDetails.CertificateHash,
		"not_after":          newDetails.NotAfter.UTC(),
		"config_sha256":      state.ConfigSHA256,
		"forced":             force,
	}
	if err := writeTransactionRecord(transactionDirectory, record); err != nil {
		return err
	}
	committed = true
	_ = fsutil.RemoveManagedTree(stagingDirectory, transactionDirectory)
	_ = fsutil.RemoveManagedTree(backupDirectory, transactionDirectory)
	_ = appendAudit(map[string]any{
		"time":           time.Now().UTC(),
		"transaction_id": transactionID,
		"command":        "cert renew",
		"status":         "COMMITTED",
	})
	return printJSON(commandResult{Command: "cert renew", Status: "PASS", Detail: map[string]any{
		"result":             "RENEWED",
		"transaction_id":     transactionID,
		"domain":             state.Domain,
		"certificate_sha256": newDetails.CertificateHash,
		"not_after":          newDetails.NotAfter.UTC(),
	}})
}

func readInstalledState() (model.State, error) {
	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		return model.State{}, fmt.Errorf("read state: %w", err)
	}
	state, err := decodeInstalledState(stateBytes)
	if err != nil {
		return model.State{}, err
	}
	if state.Reality.ID == "" && state.Hysteria2.ID == "" {
		return model.State{}, errors.New("installed state has no managed inbound instances")
	}
	return state, nil
}

func validateManagedCertificateState(state model.State) error {
	if state.Domain == "" || state.Hysteria2.CertificatePath != managedCertificate || state.Hysteria2.KeyPath != managedKey {
		return errors.New("state does not describe the managed Hysteria2 certificate")
	}
	return nil
}

func readCertificateDetails(domain string) (certificateDetails, error) {
	pair, err := tls.LoadX509KeyPair(managedCertificate, managedKey)
	if err != nil {
		return certificateDetails{}, fmt.Errorf("load managed certificate pair: %w", err)
	}
	if len(pair.Certificate) == 0 {
		return certificateDetails{}, errors.New("managed certificate has no leaf certificate")
	}
	certificate, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return certificateDetails{}, fmt.Errorf("parse managed certificate: %w", err)
	}
	if err := certificate.VerifyHostname(domain); err != nil {
		return certificateDetails{}, fmt.Errorf("managed certificate does not match %s: %w", domain, err)
	}
	certificateBytes, err := os.ReadFile(managedCertificate)
	if err != nil {
		return certificateDetails{}, err
	}
	days := int64(time.Until(certificate.NotAfter).Hours() / 24)
	return certificateDetails{
		Domain:          domain,
		CertificatePath: managedCertificate,
		KeyPath:         managedKey,
		NotBefore:       certificate.NotBefore,
		NotAfter:        certificate.NotAfter,
		DaysRemaining:   days,
		RenewalDue:      time.Until(certificate.NotAfter) <= certificateRenewalWindow,
		CertificateHash: sha256Hex(certificateBytes),
	}, nil
}

func readCloudflareToken() (string, error) {
	if token := strings.TrimSpace(os.Getenv("VPSKIT_CF_DNS_API_TOKEN")); token != "" {
		if strings.ContainsAny(token, "\r\n\x00") {
			return "", errors.New("cloudflare token contains invalid control characters")
		}
		return token, nil
	}
	content, err := os.ReadFile(cloudflareEnvPath)
	if err != nil {
		return "", fmt.Errorf("read Cloudflare secret file: %w", err)
	}
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "CF_DNS_API_TOKEN=") {
			token := strings.TrimSpace(strings.TrimPrefix(line, "CF_DNS_API_TOKEN="))
			if token == "" || strings.ContainsAny(token, "\r\n\x00") {
				break
			}
			return token, nil
		}
	}
	return "", errors.New("cloudflare token is missing from the managed secret file")
}

func legoRenewArguments(domain, legoStatePath string, acmeConfig acmeConfig, force bool) []string {
	arguments := acmeConfig.legoRunArguments(domain, legoStatePath)
	arguments = append(arguments, "--renew-days", "30")
	if force {
		arguments = append(arguments, "--renew-force")
	}
	return arguments
}

func runLegoRenewal(domain, legoStatePath, token string, acmeConfig acmeConfig, force bool) error {
	command := exec.Command(installedLego, legoRenewArguments(domain, legoStatePath, acmeConfig, force)...)
	command.Env = acmeConfig.legoEnvironment(os.Environ(), token)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("lego DNS-01 renewal failed: %w: %s", err, sanitizeText(string(output), token))
	}
	return nil
}

func activateCertificateRenewal(transactionID, issuedCertificate, issuedKey, stagingLego, backupLego string) error {
	temporaryCertificate := managedCertificate + ".vpskit-renew-" + transactionID
	temporaryKey := managedKey + ".vpskit-renew-" + transactionID
	defer os.Remove(temporaryCertificate)
	defer os.Remove(temporaryKey)
	if err := fsutil.CopyFileAtomic(issuedCertificate, temporaryCertificate, 0o640); err != nil {
		return err
	}
	if err := fsutil.CopyFileAtomic(issuedKey, temporaryKey, 0o640); err != nil {
		return err
	}
	if err := os.Rename(temporaryCertificate, managedCertificate); err != nil {
		return fmt.Errorf("activate renewed certificate: %w", err)
	}
	if err := os.Rename(temporaryKey, managedKey); err != nil {
		return fmt.Errorf("activate renewed key: %w", err)
	}
	if err := os.Rename(legoStateRoot, backupLego); err != nil {
		return fmt.Errorf("stage previous lego state: %w", err)
	}
	if err := os.Rename(stagingLego, legoStateRoot); err != nil {
		return fmt.Errorf("activate renewed lego state: %w", err)
	}
	return nil
}

func rollbackCertificateRenewal(activated, restartAttempted bool, backupCertificate, backupKey, backupLego string) error {
	if !activated {
		return nil
	}
	var rollbackErrors []string
	if err := fsutil.CopyFileAtomic(backupCertificate, managedCertificate, 0o640); err != nil {
		rollbackErrors = append(rollbackErrors, "restore certificate: "+err.Error())
	}
	if err := fsutil.CopyFileAtomic(backupKey, managedKey, 0o640); err != nil {
		rollbackErrors = append(rollbackErrors, "restore key: "+err.Error())
	}
	if err := setManagedCertificateOwnership(); err != nil {
		rollbackErrors = append(rollbackErrors, "restore certificate ownership: "+err.Error())
	}
	if _, err := os.Stat(backupLego); err == nil {
		if err := fsutil.RemoveManagedTree(legoStateRoot, stateRoot); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollbackErrors = append(rollbackErrors, "remove renewed lego state: "+err.Error())
		}
		if err := os.Rename(backupLego, legoStateRoot); err != nil {
			rollbackErrors = append(rollbackErrors, "restore lego state: "+err.Error())
		}
	}
	if restartAttempted {
		if output, err := runCommand("systemctl", "restart", serviceUnitName); err != nil {
			rollbackErrors = append(rollbackErrors, "restart restored service: "+sanitizeText(output, ""))
		}
	}
	if len(rollbackErrors) > 0 {
		return errors.New(strings.Join(rollbackErrors, "; "))
	}
	return nil
}

func setManagedCertificateOwnership() error {
	account, err := user.Lookup("vpskit")
	if err != nil {
		return fmt.Errorf("lookup vpskit account: %w", err)
	}
	gid, err := strconv.Atoi(account.Gid)
	if err != nil {
		return fmt.Errorf("parse vpskit group id: %w", err)
	}
	for _, path := range []string{managedCertificate, managedKey} {
		if err := os.Chown(path, 0, gid); err != nil {
			return fmt.Errorf("chown %s: %w", path, err)
		}
		if err := os.Chmod(path, 0o640); err != nil {
			return fmt.Errorf("chmod %s: %w", path, err)
		}
	}
	return nil
}

func copyManagedTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := destination
		if relative != "." {
			target = filepath.Join(destination, relative)
		}
		if entry.IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			return os.Chmod(target, 0o700)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink in managed lego state: %s", path)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("refusing non-regular file in managed lego state: %s", path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return fsutil.CopyFileAtomic(path, target, info.Mode().Perm())
	})
}

func writeTransactionRecord(directory string, record map[string]any) error {
	bytes, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(filepath.Join(directory, "transaction.json"), append(bytes, '\n'), 0o600)
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func certificateRenewServiceUnit() string {
	return `[Unit]
Description=VPSKit managed certificate renewal
After=network-online.target nss-lookup.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/vpskit cert renew
NoNewPrivileges=true
PrivateTmp=true
PrivateDevices=true
ProtectHome=true
ProtectSystem=strict
ReadWritePaths=/var/lib/vpskit /var/log/vpskit
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
`
}

func certificateRenewTimerUnit() string {
	return `[Unit]
Description=VPSKit daily certificate renewal timer

[Timer]
OnCalendar=*-*-* 03:17:00
RandomizedDelaySec=1h
Persistent=true
Unit=vpskit-certificate-renew.service

[Install]
WantedBy=timers.target
`
}
