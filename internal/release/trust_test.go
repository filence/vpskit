package release

import (
	"strings"
	"testing"
)

func TestTrustPolicyAcceptsCurrentAndNextKeys(t *testing.T) {
	currentPublic, currentPrivate, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	nextPublic, nextPrivate, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	currentID, _ := PublicKeyID(currentPublic)
	nextID, _ := PublicKeyID(nextPublic)
	policy := TrustPolicy{SchemaVersion: 1, Keys: []TrustedKey{
		{ID: currentID, PublicKey: currentPublic},
		{ID: nextID, PublicKey: nextPublic},
	}}
	material, err := EncodeTrustPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseTrustPolicy(material)
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		id         string
		privateKey string
	}{{currentID, currentPrivate}, {nextID, nextPrivate}} {
		manifest := Manifest{SchemaVersion: 2, ReleaseID: "rotation-test", SigningKeyID: testCase.id}
		bytes, err := Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		signature, err := Sign(bytes, testCase.privateKey)
		if err != nil {
			t.Fatal(err)
		}
		if err := parsed.VerifyManifest(manifest, bytes, signature); err != nil {
			t.Fatalf("trusted rotation key %s should verify: %v", testCase.id, err)
		}
	}
}

func TestTrustPolicyRejectsRevokedSigningKey(t *testing.T) {
	publicKey, privateKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	keyID, _ := PublicKeyID(publicKey)
	policy := TrustPolicy{
		SchemaVersion: 1,
		Keys:          []TrustedKey{{ID: keyID, PublicKey: publicKey}},
		RevokedKeyIDs: []string{keyID},
	}
	manifest := Manifest{SchemaVersion: 2, ReleaseID: "revoked-test", SigningKeyID: keyID}
	bytes, _ := Marshal(manifest)
	signature, _ := Sign(bytes, privateKey)
	err = policy.VerifyManifest(manifest, bytes, signature)
	if err == nil || !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("revoked key should be rejected, got %v", err)
	}
}

func TestRotationPolicyKeepsLegacyManifestCompatibility(t *testing.T) {
	publicKey, privateKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	keyID, _ := PublicKeyID(publicKey)
	policy := TrustPolicy{SchemaVersion: 1, Keys: []TrustedKey{{ID: keyID, PublicKey: publicKey}}}
	manifest := Manifest{SchemaVersion: 1, ReleaseID: "legacy-test"}
	bytes, _ := Marshal(manifest)
	signature, _ := Sign(bytes, privateKey)
	if err := policy.VerifyManifest(manifest, bytes, signature); err != nil {
		t.Fatalf("active key should continue to verify schema 1 manifests: %v", err)
	}
}
