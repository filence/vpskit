package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/release"
)

type versionsLockDocument struct {
	SchemaVersion    int                 `json:"schema_version"`
	GeneratedAt      time.Time           `json:"generated_at"`
	TemplateRevision int                 `json:"template_revision"`
	Assets           []versionsLockAsset `json:"assets"`
}

type versionsLockAsset struct {
	ID             string                    `json:"id"`
	Version        string                    `json:"version"`
	Channel        string                    `json:"channel"`
	SourceRepo     string                    `json:"source_repo"`
	SourceRef      string                    `json:"source_ref"`
	SourceCommit   string                    `json:"source_commit"`
	SourceURL      string                    `json:"source_url"`
	SourceArchive  versionsLockSourceArchive `json:"source_archive"`
	InstalledFile  string                    `json:"installed_file"`
	BinarySHA256   string                    `json:"binary_sha256"`
	VerifiedAt     string                    `json:"verified_at"`
	StateSchemaMin int                       `json:"state_schema_min"`
}

type versionsLockSourceArchive struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

func validateVersionsLock(bundleDir string, lockManifest, singBoxManifest, xrayManifest, legoManifest release.Asset) (versionsLockDocument, error) {
	bytes, err := os.ReadFile(filepath.Join(bundleDir, lockManifest.File))
	if err != nil {
		return versionsLockDocument{}, fmt.Errorf("read versions lock: %w", err)
	}
	var document versionsLockDocument
	if err := json.Unmarshal(bytes, &document); err != nil {
		return versionsLockDocument{}, fmt.Errorf("parse versions lock: %w", err)
	}
	if document.SchemaVersion != 1 || document.TemplateRevision < 1 || document.GeneratedAt.IsZero() {
		return versionsLockDocument{}, errors.New("versions lock header is invalid")
	}
	if len(document.Assets) != 3 {
		return versionsLockDocument{}, fmt.Errorf("versions lock must contain exactly sing-box, xray and lego, got %d assets", len(document.Assets))
	}
	locked := make(map[string]versionsLockAsset, len(document.Assets))
	for _, asset := range document.Assets {
		if asset.ID == "" || locked[asset.ID].ID != "" {
			return versionsLockDocument{}, fmt.Errorf("versions lock contains duplicate asset %q", asset.ID)
		}
		if err := validateLockedAsset(asset); err != nil {
			return versionsLockDocument{}, err
		}
		locked[asset.ID] = asset
	}
	if err := matchLockedAsset(locked["sing-box"], singBoxManifest); err != nil {
		return versionsLockDocument{}, err
	}
	if err := matchLockedAsset(locked["xray"], xrayManifest); err != nil {
		return versionsLockDocument{}, err
	}
	if err := matchLockedAsset(locked["lego"], legoManifest); err != nil {
		return versionsLockDocument{}, err
	}
	return document, nil
}

func validateLockedAsset(asset versionsLockAsset) error {
	if asset.ID != "sing-box" && asset.ID != "xray" && asset.ID != "lego" {
		return fmt.Errorf("versions lock contains unsupported asset %q", asset.ID)
	}
	if asset.Version == "" || asset.Channel != "stable" || asset.SourceRepo == "" || asset.SourceRef == "" || asset.SourceURL == "" {
		return fmt.Errorf("versions lock asset %s has incomplete source identity", asset.ID)
	}
	if !coreCommitPattern.MatchString(asset.SourceCommit) {
		return fmt.Errorf("versions lock asset %s has invalid source commit", asset.ID)
	}
	if asset.SourceArchive.Name == "" || filepath.Base(asset.SourceArchive.Name) != asset.SourceArchive.Name || asset.SourceArchive.Size <= 0 || !isSHA256(asset.SourceArchive.SHA256) {
		return fmt.Errorf("versions lock asset %s has invalid source archive identity", asset.ID)
	}
	if asset.InstalledFile != asset.ID || !isSHA256(asset.BinarySHA256) || asset.VerifiedAt == "" || asset.StateSchemaMin < 1 {
		return fmt.Errorf("versions lock asset %s has invalid installed identity", asset.ID)
	}
	return nil
}

func matchLockedAsset(locked versionsLockAsset, manifest release.Asset) error {
	if locked.ID == "" {
		return fmt.Errorf("versions lock is missing asset %s", manifest.ID)
	}
	if locked.Version != manifest.Version || !strings.EqualFold(locked.BinarySHA256, manifest.SHA256) {
		return fmt.Errorf("versions lock does not match signed manifest asset %s", manifest.ID)
	}
	if manifest.SourceURL != "" && locked.SourceURL != manifest.SourceURL {
		return fmt.Errorf("versions lock source URL does not match signed manifest asset %s", manifest.ID)
	}
	if manifest.SourceRepo != "" && locked.SourceRepo != manifest.SourceRepo {
		return fmt.Errorf("versions lock source repository does not match signed manifest asset %s", manifest.ID)
	}
	if manifest.SourceRef != "" && locked.SourceRef != manifest.SourceRef {
		return fmt.Errorf("versions lock source ref does not match signed manifest asset %s", manifest.ID)
	}
	if manifest.SourceCommit != "" && locked.SourceCommit != manifest.SourceCommit {
		return fmt.Errorf("versions lock source commit does not match signed manifest asset %s", manifest.ID)
	}
	return nil
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range strings.ToLower(value) {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}
