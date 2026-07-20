package release

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSignedManifestRoundTrip(t *testing.T) {
	directory := t.TempDir()
	assetPath := filepath.Join(directory, "vpskit")
	if err := os.WriteFile(assetPath, []byte("test-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	publicKey, privateKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	keyID, err := PublicKeyID(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := BuildManifest("v0.1.0-test", keyID, directory, []Asset{{ID: "vpskit", Version: "test", File: "vpskit"}})
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := Sign(manifestBytes, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(directory, "release-manifest.json")
	signaturePath := filepath.Join(directory, "release-manifest.sig")
	if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(signaturePath, []byte(signature+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyDirectory(directory, manifestPath, signaturePath, publicKey); err != nil {
		t.Fatal(err)
	}
}

func TestManifestRejectsTraversal(t *testing.T) {
	_, err := BuildManifest("test", "ed25519-sha256:0000000000000000000000000000000000000000000000000000000000000000", t.TempDir(), []Asset{{ID: "bad", File: "../bad"}})
	if err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
}

func TestManifestAllowsSafeNestedLicense(t *testing.T) {
	directory := t.TempDir()
	licensePath := filepath.Join(directory, "licenses", "dependency.LICENSE")
	if err := os.MkdirAll(filepath.Dir(licensePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(licensePath, []byte("license"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := BuildManifest("test", "ed25519-sha256:0000000000000000000000000000000000000000000000000000000000000000", directory, []Asset{{ID: "license", File: "licenses/dependency.LICENSE"}})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Assets[0].File != "licenses/dependency.LICENSE" {
		t.Fatalf("nested asset path changed: %#v", manifest.Assets[0])
	}
}

func TestUnsignedBundleFileIsRejected(t *testing.T) {
	directory := t.TempDir()
	assetPath := filepath.Join(directory, "vpskit")
	if err := os.WriteFile(assetPath, []byte("original"), 0o755); err != nil {
		t.Fatal(err)
	}
	publicKey, privateKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	keyID, err := PublicKeyID(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := BuildManifest("v0.1.0-test", keyID, directory, []Asset{{ID: "vpskit", Version: "test", File: "vpskit"}})
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := Sign(manifestBytes, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(directory, "release-manifest.json")
	signaturePath := filepath.Join(directory, "release-manifest.sig")
	if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(signaturePath, []byte(signature+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "unsigned.txt"), []byte("unsigned"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyDirectory(directory, manifestPath, signaturePath, publicKey); err == nil {
		t.Fatal("unsigned bundle file was incorrectly accepted")
	}
}

func TestTamperedAssetIsRejected(t *testing.T) {
	directory := t.TempDir()
	assetPath := filepath.Join(directory, "vpskit")
	if err := os.WriteFile(assetPath, []byte("original"), 0o755); err != nil {
		t.Fatal(err)
	}
	publicKey, privateKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	keyID, err := PublicKeyID(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := BuildManifest("v0.1.0-test", keyID, directory, []Asset{{ID: "vpskit", Version: "test", File: "vpskit"}})
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := Sign(manifestBytes, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(directory, "release-manifest.json")
	signaturePath := filepath.Join(directory, "release-manifest.sig")
	if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(signaturePath, []byte(signature+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(assetPath, []byte("tampered"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyDirectory(directory, manifestPath, signaturePath, publicKey); err == nil {
		t.Fatal("tampered asset was incorrectly accepted")
	}
}
