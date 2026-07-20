package release

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const TrustPolicySchemaVersion = 1

type TrustPolicy struct {
	SchemaVersion int          `json:"schema_version"`
	Keys          []TrustedKey `json:"keys"`
	RevokedKeyIDs []string     `json:"revoked_key_ids,omitempty"`
}

type TrustedKey struct {
	ID        string `json:"id"`
	PublicKey string `json:"public_key"`
}

func PublicKeyID(publicKeyBase64 string) (string, error) {
	publicKey, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(publicKeyBase64))
	if err != nil {
		return "", fmt.Errorf("decode public key: %w", err)
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return "", fmt.Errorf("invalid public key length: %d", len(publicKey))
	}
	digest := sha256.Sum256(publicKey)
	return "ed25519-sha256:" + hex.EncodeToString(digest[:]), nil
}

func EncodeTrustPolicy(policy TrustPolicy) (string, error) {
	if err := policy.Validate(); err != nil {
		return "", err
	}
	bytes, err := json.Marshal(policy)
	if err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(bytes), nil
}

func ParseTrustPolicy(material string) (TrustPolicy, error) {
	trimmed := strings.TrimSpace(material)
	if trimmed == "" {
		return TrustPolicy{}, errors.New("release trust policy is empty")
	}
	if keyID, err := PublicKeyID(trimmed); err == nil {
		return TrustPolicy{SchemaVersion: 1, Keys: []TrustedKey{{ID: keyID, PublicKey: trimmed}}}, nil
	}
	bytes, err := base64.RawStdEncoding.DecodeString(trimmed)
	if err != nil {
		return TrustPolicy{}, fmt.Errorf("decode release trust policy: %w", err)
	}
	var policy TrustPolicy
	if err := json.Unmarshal(bytes, &policy); err != nil {
		return TrustPolicy{}, fmt.Errorf("parse release trust policy: %w", err)
	}
	if err := policy.Validate(); err != nil {
		return TrustPolicy{}, err
	}
	return policy, nil
}

func (policy TrustPolicy) Validate() error {
	if policy.SchemaVersion != TrustPolicySchemaVersion || len(policy.Keys) == 0 {
		return errors.New("release trust policy header is invalid")
	}
	seen := make(map[string]bool, len(policy.Keys))
	for _, key := range policy.Keys {
		computedID, err := PublicKeyID(key.PublicKey)
		if err != nil {
			return err
		}
		if key.ID != computedID || seen[key.ID] {
			return fmt.Errorf("release trust policy has invalid or duplicate key id %q", key.ID)
		}
		seen[key.ID] = true
	}
	revoked := make(map[string]bool, len(policy.RevokedKeyIDs))
	for _, keyID := range policy.RevokedKeyIDs {
		if !validKeyID(keyID) || revoked[keyID] {
			return fmt.Errorf("release trust policy has invalid or duplicate revoked key id %q", keyID)
		}
		revoked[keyID] = true
	}
	return nil
}

func (policy TrustPolicy) VerifyManifest(manifest Manifest, manifestBytes []byte, signatureBase64 string) error {
	revoked := make(map[string]bool, len(policy.RevokedKeyIDs))
	for _, keyID := range policy.RevokedKeyIDs {
		revoked[keyID] = true
	}
	if manifest.SchemaVersion >= 2 {
		if !validKeyID(manifest.SigningKeyID) {
			return errors.New("manifest signing_key_id is invalid")
		}
		if revoked[manifest.SigningKeyID] {
			return fmt.Errorf("manifest signing key is revoked: %s", manifest.SigningKeyID)
		}
		for _, key := range policy.Keys {
			if key.ID == manifest.SigningKeyID {
				return VerifySignature(manifestBytes, signatureBase64, key.PublicKey)
			}
		}
		return fmt.Errorf("manifest signing key is not trusted: %s", manifest.SigningKeyID)
	}
	for _, key := range policy.Keys {
		if revoked[key.ID] {
			continue
		}
		if VerifySignature(manifestBytes, signatureBase64, key.PublicKey) == nil {
			return nil
		}
	}
	return errors.New("legacy manifest signature is not valid for an active trusted key")
}

func validKeyID(value string) bool {
	const prefix = "ed25519-sha256:"
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+64 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, prefix))
	return err == nil
}
