package release

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const ManifestSchemaVersion = 2

type Manifest struct {
	SchemaVersion int       `json:"schema_version"`
	ReleaseID     string    `json:"release_id"`
	SigningKeyID  string    `json:"signing_key_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	Assets        []Asset   `json:"assets"`
}

type Asset struct {
	ID             string `json:"id"`
	Version        string `json:"version"`
	Channel        string `json:"channel,omitempty"`
	File           string `json:"file"`
	Size           int64  `json:"size"`
	SHA256         string `json:"sha256"`
	SourceURL      string `json:"source_url"`
	SourceRepo     string `json:"source_repo,omitempty"`
	SourceRef      string `json:"source_ref,omitempty"`
	SourceCommit   string `json:"source_commit,omitempty"`
	StateSchemaMin int    `json:"state_schema_min,omitempty"`
}

func GenerateKeyPair() (publicKey, privateKey string, err error) {
	publicBytes, privateBytes, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return base64.RawStdEncoding.EncodeToString(publicBytes), base64.RawStdEncoding.EncodeToString(privateBytes), nil
}

func Sign(manifestBytes []byte, privateKeyBase64 string) (string, error) {
	privateKey, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(privateKeyBase64))
	if err != nil {
		return "", fmt.Errorf("decode private key: %w", err)
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("invalid private key length: %d", len(privateKey))
	}
	signature := ed25519.Sign(ed25519.PrivateKey(privateKey), manifestBytes)
	return base64.RawStdEncoding.EncodeToString(signature), nil
}

func VerifySignature(manifestBytes []byte, signatureBase64, publicKeyBase64 string) error {
	publicKey, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(publicKeyBase64))
	if err != nil {
		return fmt.Errorf("decode public key: %w", err)
	}
	signature, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(signatureBase64))
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key length: %d", len(publicKey))
	}
	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature length: %d", len(signature))
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKey), manifestBytes, signature) {
		return errors.New("release manifest signature is invalid")
	}
	return nil
}

func BuildManifest(releaseID, signingKeyID, directory string, assets []Asset) (Manifest, error) {
	if !validKeyID(signingKeyID) {
		return Manifest{}, errors.New("signing key id is required for manifest schema 2")
	}
	manifest := Manifest{
		SchemaVersion: ManifestSchemaVersion,
		ReleaseID:     releaseID,
		SigningKeyID:  signingKeyID,
		CreatedAt:     time.Now().UTC().Truncate(time.Second),
		Assets:        make([]Asset, len(assets)),
	}
	copy(manifest.Assets, assets)
	seen := map[string]bool{}
	for index := range manifest.Assets {
		asset := &manifest.Assets[index]
		if err := validateAssetName(asset.File); err != nil {
			return Manifest{}, err
		}
		if seen[asset.File] {
			return Manifest{}, fmt.Errorf("duplicate asset file: %s", asset.File)
		}
		seen[asset.File] = true
		path := filepath.Join(directory, asset.File)
		info, err := os.Stat(path)
		if err != nil {
			return Manifest{}, fmt.Errorf("stat asset %s: %w", asset.File, err)
		}
		if !info.Mode().IsRegular() {
			return Manifest{}, fmt.Errorf("asset is not a regular file: %s", asset.File)
		}
		asset.Size = info.Size()
		asset.SHA256, err = FileSHA256(path)
		if err != nil {
			return Manifest{}, err
		}
	}
	sort.Slice(manifest.Assets, func(i, j int) bool { return manifest.Assets[i].ID < manifest.Assets[j].ID })
	return manifest, nil
}

func Marshal(manifest Manifest) ([]byte, error) {
	bytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(bytes, '\n'), nil
}

func VerifyDirectory(directory, manifestPath, signaturePath, trustMaterial string) (Manifest, error) {
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("read release manifest: %w", err)
	}
	signatureBytes, err := os.ReadFile(signaturePath)
	if err != nil {
		return Manifest{}, fmt.Errorf("read release signature: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse release manifest: %w", err)
	}
	if manifest.SchemaVersion != 1 && manifest.SchemaVersion != ManifestSchemaVersion {
		return Manifest{}, fmt.Errorf("unsupported manifest schema: %d", manifest.SchemaVersion)
	}
	policy, err := ParseTrustPolicy(trustMaterial)
	if err != nil {
		return Manifest{}, err
	}
	if err := policy.VerifyManifest(manifest, manifestBytes, string(signatureBytes)); err != nil {
		return Manifest{}, err
	}
	if len(manifest.Assets) == 0 {
		return Manifest{}, errors.New("release manifest has no assets")
	}
	seenIDs := map[string]bool{}
	seenFiles := map[string]bool{}
	for _, asset := range manifest.Assets {
		if asset.ID == "" || seenIDs[asset.ID] {
			return Manifest{}, fmt.Errorf("invalid or duplicate asset id: %q", asset.ID)
		}
		seenIDs[asset.ID] = true
		if err := validateAssetName(asset.File); err != nil {
			return Manifest{}, err
		}
		if seenFiles[asset.File] {
			return Manifest{}, fmt.Errorf("duplicate asset file: %s", asset.File)
		}
		seenFiles[asset.File] = true
		path := filepath.Join(directory, asset.File)
		info, err := os.Stat(path)
		if err != nil {
			return Manifest{}, fmt.Errorf("stat asset %s: %w", asset.File, err)
		}
		if info.Size() != asset.Size {
			return Manifest{}, fmt.Errorf("asset size mismatch for %s", asset.File)
		}
		digest, err := FileSHA256(path)
		if err != nil {
			return Manifest{}, err
		}
		if !strings.EqualFold(digest, asset.SHA256) {
			return Manifest{}, fmt.Errorf("asset digest mismatch for %s", asset.File)
		}
	}
	allowedFiles := map[string]bool{
		filepath.ToSlash(relativePathWithin(directory, manifestPath)):  true,
		filepath.ToSlash(relativePathWithin(directory, signaturePath)): true,
	}
	for file := range seenFiles {
		allowedFiles[file] = true
	}
	if err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == directory {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("release bundle contains a symlink: %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("release bundle contains a non-regular file: %s", path)
		}
		relative, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if !allowedFiles[relative] {
			return fmt.Errorf("release bundle contains an unsigned file: %s", relative)
		}
		return nil
	}); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func FindAsset(manifest Manifest, id string) (Asset, error) {
	for _, asset := range manifest.Assets {
		if asset.ID == id {
			return asset, nil
		}
	}
	return Asset{}, fmt.Errorf("asset not found in manifest: %s", id)
}

func FileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func validateAssetName(name string) error {
	if name == "" || strings.Contains(name, `\\`) || filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return fmt.Errorf("unsafe asset file name: %q", name)
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
	if clean == "." || clean != name || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return fmt.Errorf("unsafe asset file name: %q", name)
	}
	return nil
}

func relativePathWithin(directory, path string) string {
	relative, err := filepath.Rel(directory, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return ""
	}
	return relative
}
