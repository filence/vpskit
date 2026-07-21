package app

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"vpskit.local/vpskit/internal/fsutil"
	"vpskit.local/vpskit/internal/platform"
)

func runUninstall(arguments []string) error {
	flags := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	yes := flags.Bool("yes", false, "confirm removal of VPSKit-managed files and services")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if !*yes {
		return errors.New("uninstall requires explicit --yes confirmation")
	}
	if !platform.IsRoot() {
		return errors.New("uninstall requires root privileges")
	}
	ownership, err := readOwnershipDocument()
	if err != nil {
		return err
	}
	files, err := ownedPathList(ownership, "files", uninstallAllowedFiles)
	if err != nil {
		return err
	}
	directories, err := ownedPathList(ownership, "directories", uninstallAllowedDirectories)
	if err != nil {
		return err
	}
	services, err := ownedPathList(ownership, "services", uninstallAllowedServices)
	if err != nil {
		return err
	}
	lock, err := platform.AcquireProcessLock(stateRoot + "/locks/vpskit.lock")
	if err != nil {
		return err
	}
	defer lock.Close()
	certificateReferences, err := externalCertificateReferences("/etc/systemd/system")
	if err != nil {
		return err
	}
	preserveCertificate := len(certificateReferences) > 0
	if preserveCertificate {
		files = withoutPaths(files, managedCertificate, managedKey)
		directories = withoutPaths(directories, certificateRoot, stateRoot)
	}

	for _, service := range services {
		if output, stopErr := runCommand("systemctl", "disable", "--now", service); stopErr != nil && !strings.Contains(output, "not loaded") {
			return fmt.Errorf("stop managed service %s: %w: %s", service, stopErr, output)
		}
	}
	for _, path := range files {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove managed file %s: %w", path, err)
		}
	}
	sort.Slice(directories, func(i, j int) bool { return len(directories[i]) > len(directories[j]) })
	for _, path := range directories {
		if err := fsutil.RemoveManagedTree(path, path); err != nil {
			return err
		}
	}
	if preserveCertificate {
		_ = os.Remove(stateRoot)
	}
	if output, reloadErr := runCommand("systemctl", "daemon-reload"); reloadErr != nil {
		return fmt.Errorf("systemd daemon-reload failed: %w: %s", reloadErr, output)
	}
	if output, userErr := runCommand("userdel", "vpskit"); userErr != nil && !strings.Contains(output, "does not exist") {
		return fmt.Errorf("remove vpskit user: %w: %s", userErr, output)
	}
	if output, groupErr := runCommand("groupdel", "vpskit"); groupErr != nil && !strings.Contains(output, "does not exist") {
		return fmt.Errorf("remove vpskit group: %w: %s", groupErr, output)
	}
	return printJSON(commandResult{Command: "uninstall", Status: "PASS", Detail: map[string]any{
		"removed_files":          len(files),
		"removed_directories":    len(directories),
		"stopped_services":       len(services),
		"certificate_preserved":  preserveCertificate,
		"certificate_references": certificateReferences,
	}})
}

func externalCertificateReferences(systemdDirectory string) ([]string, error) {
	references := make([]string, 0)
	patterns := []string{"*.service", "*.timer", "*.socket"}
	managedUnits := pathSet(serviceUnitPath, xrayServiceUnitPath, certificateRenewServiceUnitPath, certificateRenewTimerUnitPath)
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(systemdDirectory, pattern))
		if err != nil {
			return nil, err
		}
		for _, path := range matches {
			resolved := path
			if target, err := filepath.EvalSymlinks(path); err == nil {
				resolved = target
			}
			if managedUnits[path] || managedUnits[resolved] {
				continue
			}
			info, err := os.Stat(resolved)
			if err != nil || !info.Mode().IsRegular() || info.Size() > 1024*1024 {
				continue
			}
			bytes, err := os.ReadFile(resolved)
			if err != nil {
				return nil, fmt.Errorf("read systemd unit %s: %w", path, err)
			}
			text := string(bytes)
			if strings.Contains(text, managedCertificate) || strings.Contains(text, managedKey) {
				references = append(references, path)
			}
		}
	}
	sort.Strings(references)
	return references, nil
}

func withoutPaths(values []string, excluded ...string) []string {
	blocked := pathSet(excluded...)
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if !blocked[value] {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

var uninstallAllowedFiles = map[string]bool{
	installedBinary:                 true,
	installedSingBox:                true,
	installedXray:                   true,
	installedLego:                   true,
	versionsLockPath:                true,
	serviceUnitPath:                 true,
	xrayServiceUnitPath:             true,
	certificateRenewServiceUnitPath: true,
	certificateRenewTimerUnitPath:   true,
	serverConfigPath:                true,
	xrayServerConfigPath:            true,
	managedCertificate:              true,
	managedKey:                      true,
	statePath:                       true,
	ownershipPath:                   true,
	secretPath:                      true,
	cloudflareEnvPath:               true,
	acmeEnvPath:                     true,
	subscriptionConfigPath:          true,
	subscriptionSecretPath:          true,
	publicationStatePath:            true,
}

var uninstallAllowedDirectories = map[string]bool{
	configRoot:              true,
	exportRoot:              true,
	stateRoot:               true,
	secretRoot:              true,
	certificateRoot:         true,
	legoStateRoot:           true,
	backupRoot:              true,
	transactionRoot:         true,
	logRoot:                 true,
	"/usr/local/lib/vpskit": true,
}

var uninstallAllowedServices = map[string]bool{
	serviceUnitName:                 true,
	xrayServiceUnitName:             true,
	certificateRenewServiceUnitName: true,
	certificateRenewTimerUnitName:   true,
}

func ownedPathList(document map[string]any, key string, allowed map[string]bool) ([]string, error) {
	raw, ok := document[key].([]any)
	if !ok {
		return nil, fmt.Errorf("ownership field %s is invalid", key)
	}
	seen := map[string]bool{}
	values := make([]string, 0, len(raw))
	for _, value := range raw {
		path, ok := value.(string)
		if !ok || !allowed[path] || seen[path] {
			return nil, fmt.Errorf("ownership field %s contains unmanaged path: %v", key, value)
		}
		seen[path] = true
		values = append(values, path)
	}
	return values, nil
}
