package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"vpskit.local/vpskit/internal/platform"
)

type orphanCandidate struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

func runOrphan(arguments []string) error {
	if len(arguments) != 1 || arguments[0] != "scan" {
		return errors.New("usage: vpskit orphan scan")
	}
	if !platform.IsRoot() {
		return errors.New("orphan scan requires root privileges")
	}
	ownership, err := readOwnershipDocument()
	if err != nil {
		return err
	}
	ownedFiles, err := ownedPathList(ownership, "files", uninstallAllowedFiles)
	if err != nil {
		return err
	}
	ownedDirectories, err := ownedPathList(ownership, "directories", uninstallAllowedDirectories)
	if err != nil {
		return err
	}
	ownedServices, err := ownedPathList(ownership, "services", uninstallAllowedServices)
	if err != nil {
		return err
	}

	missing := make([]string, 0)
	for _, path := range append(append([]string{}, ownedFiles...), ownedDirectories...) {
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			missing = append(missing, path)
		} else if err != nil {
			return fmt.Errorf("inspect owned path %s: %w", path, err)
		}
	}
	for _, service := range ownedServices {
		unitPath := serviceUnitFilePath(service)
		if _, err := os.Lstat(unitPath); errors.Is(err, os.ErrNotExist) {
			missing = append(missing, unitPath)
		} else if err != nil {
			return fmt.Errorf("inspect owned service %s: %w", service, err)
		}
	}

	candidates := make([]orphanCandidate, 0)
	for _, scope := range orphanScanScopes() {
		found, err := unexpectedDirectChildren(scope.parent, scope.expected)
		if err != nil {
			return err
		}
		candidates = append(candidates, found...)
	}
	for _, pattern := range []struct {
		glob     string
		expected map[string]bool
		reason   string
	}{
		{filepath.Join("/usr/local/bin", "vpskit*"), pathSet(installedBinary), "untracked VPSKit-like executable"},
		{filepath.Join("/etc/systemd/system", "vpskit-*.service"), pathSet(serviceUnitPath, xrayServiceUnitPath, certificateRenewServiceUnitPath), "untracked VPSKit-like systemd service"},
		{filepath.Join("/etc/systemd/system", "vpskit-*.timer"), pathSet(certificateRenewTimerUnitPath), "untracked VPSKit-like systemd timer"},
	} {
		matches, err := filepath.Glob(pattern.glob)
		if err != nil {
			return err
		}
		for _, match := range matches {
			if !pattern.expected[match] {
				candidates = append(candidates, orphanCandidate{Path: match, Reason: pattern.reason})
			}
		}
	}

	sort.Strings(missing)
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Path < candidates[j].Path })
	return printJSON(commandResult{Command: "orphan scan", Status: "PASS", Detail: map[string]any{
		"mode":               "READ_ONLY",
		"needs_review":       len(missing) > 0 || len(candidates) > 0,
		"missing_owned":      missing,
		"orphan_candidates":  candidates,
		"automatic_deletion": false,
	}})
}

type orphanScanScope struct {
	parent   string
	expected map[string]bool
}

func orphanScanScopes() []orphanScanScope {
	return []orphanScanScope{
		{configRoot, pathSet(generatedRoot, exportRoot, versionsLockPath)},
		{stateRoot, pathSet(statePath, ownershipPath, secretRoot, certificateRoot, backupRoot, transactionRoot, legoStateRoot, filepath.Join(stateRoot, "locks"))},
		{secretRoot, pathSet(secretPath, cloudflareEnvPath, acmeEnvPath)},
		{certificateRoot, pathSet(managedCertificate, managedKey)},
		{logRoot, pathSet(auditLogPath)},
		{"/usr/local/lib/vpskit", pathSet(filepath.Dir(installedSingBox))},
		{filepath.Dir(installedSingBox), pathSet(installedSingBox, installedXray, installedLego)},
	}
}

func unexpectedDirectChildren(parent string, expected map[string]bool) ([]orphanCandidate, error) {
	entries, err := os.ReadDir(parent)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan orphan scope %s: %w", parent, err)
	}
	candidates := make([]orphanCandidate, 0)
	for _, entry := range entries {
		path := filepath.Join(parent, entry.Name())
		if !expected[path] {
			candidates = append(candidates, orphanCandidate{Path: path, Reason: "unexpected entry in managed scope"})
		}
	}
	return candidates, nil
}

func pathSet(values ...string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func serviceUnitFilePath(service string) string {
	switch service {
	case serviceUnitName:
		return serviceUnitPath
	case xrayServiceUnitName:
		return xrayServiceUnitPath
	case certificateRenewServiceUnitName:
		return certificateRenewServiceUnitPath
	case certificateRenewTimerUnitName:
		return certificateRenewTimerUnitPath
	default:
		return filepath.Join("/etc/systemd/system", service)
	}
}
