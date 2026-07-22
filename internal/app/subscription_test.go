package app

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestSubscriptionDocumentsRequireBoundIdentityAndOpaqueSecrets(t *testing.T) {
	credential := testSubscriptionCredential()
	configuration, secrets, err := subscriptionDocuments("node-main", credential)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.NodeID != "node-main" || configuration.Backend != "workers" || !configuration.AutoPublish {
		t.Fatalf("unexpected subscription configuration: %#v", configuration)
	}
	if secrets.ReadToken != credential.ReadToken || secrets.NodePublishSecret != credential.NodePublishSecret {
		t.Fatal("credential values were not preserved in the secret document")
	}
	credential.NodeID = "other-node"
	if _, _, err := subscriptionDocuments("node-main", credential); err == nil {
		t.Fatal("a credential for another node was accepted")
	}
	credential = testSubscriptionCredential()
	credential.ReadToken = "x"
	if _, _, err := subscriptionDocuments("node-main", credential); err == nil {
		t.Fatal("a low-entropy read token was accepted")
	}
}

func TestConfigureAndRemoveSubscriptionFilesAreOwnershipAware(t *testing.T) {
	root := useTemporarySubscriptionPaths(t)
	configuration, secrets, err := subscriptionDocuments("node-main", testSubscriptionCredential())
	if err != nil {
		t.Fatal(err)
	}
	if err := configureSubscriptionFiles(configuration, secrets); err != nil {
		t.Fatal(err)
	}
	loadedConfiguration, loadedSecrets, err := readSubscriptionDocuments()
	if err != nil {
		t.Fatal(err)
	}
	if loadedConfiguration.Endpoint != configuration.Endpoint || loadedSecrets.ReadToken != secrets.ReadToken {
		t.Fatalf("subscription files did not round trip: %#v %#v", loadedConfiguration, loadedSecrets)
	}
	for _, path := range []string{subscriptionConfigPath, subscriptionSecretPath, publicationStatePath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("managed subscription file was not created: %s: %v", path, err)
		}
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(subscriptionSecretPath)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("subscription secret permissions are not 0600: %v %#o", err, info.Mode().Perm())
		}
	}
	ownership, err := readSubscriptionOwnership()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{subscriptionConfigPath, subscriptionSecretPath, publicationStatePath} {
		if !ownershipContains(ownership, "files", path) {
			t.Fatalf("ownership does not include %s: %#v", path, ownership)
		}
	}
	if err := removeSubscriptionFiles(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{subscriptionConfigPath, subscriptionSecretPath, publicationStatePath} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("subscription file remained after removal: %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "ownership.json")); err != nil {
		t.Fatal("ownership document was removed with subscription files")
	}
	ownership, err = readSubscriptionOwnership()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{subscriptionConfigPath, subscriptionSecretPath, publicationStatePath} {
		if ownershipContains(ownership, "files", path) {
			t.Fatalf("ownership still includes removed path %s", path)
		}
	}
}

func TestPublicationFailurePreservesLastCommittedRevision(t *testing.T) {
	useTemporarySubscriptionPaths(t)
	previous := publicationState{
		SchemaVersion: subscriptionSchemaVersion,
		Status:        "COMMITTED", PublicationID: "node-main-r0007", ClientRevision: 7,
		RulesetRevision: 3, PublishedAt: time.Date(2026, 7, 21, 1, 2, 3, 0, time.UTC),
	}
	if err := writePublicationState(previous); err != nil {
		t.Fatal(err)
	}
	if err := recordPublicationFailure(errors.New("temporary remote failure")); err != nil {
		t.Fatal(err)
	}
	actual, err := readPublicationState()
	if err != nil {
		t.Fatal(err)
	}
	if actual.Status != "DEGRADED" || actual.PublicationID != previous.PublicationID || actual.ClientRevision != previous.ClientRevision || actual.RulesetRevision != previous.RulesetRevision {
		t.Fatalf("failure erased the last committed revision: %#v", actual)
	}
	if actual.LastError == "" || actual.CheckedAt.IsZero() {
		t.Fatalf("failure state lacks diagnostics: %#v", actual)
	}
}

func TestSubscriptionCredentialReaderRejectsTrailingJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	credential := testSubscriptionCredential()
	content, err := json.Marshal(credential)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(content, []byte("\n{}\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSubscriptionCredentialInput(path); err == nil {
		t.Fatal("multiple JSON values in a sensitive credential file were accepted")
	}
}

func TestAutoPublishIsNoOpWhenNotConfigured(t *testing.T) {
	useTemporarySubscriptionPaths(t)
	result := attemptAutoPublishSubscription(t.Context())
	if result.Status != "NOT_CONFIGURED" || result.PublicationID != "" || result.LastError != "" {
		t.Fatalf("unexpected unconfigured auto-publish result: %#v", result)
	}
}

func testSubscriptionCredential() subscriptionCredentialInput {
	return subscriptionCredentialInput{
		SchemaVersion: subscriptionSchemaVersion,
		Endpoint:      "https://sub.example.com",
		NodeID:        "node-main",
		NodePublishSecret: base64.RawURLEncoding.EncodeToString(
			bytes.Repeat([]byte{0x11}, 32),
		),
		ReadToken: base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x22}, 32)),
	}
}

func useTemporarySubscriptionPaths(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	originalConfigPath := subscriptionConfigPath
	originalSecretPath := subscriptionSecretPath
	originalPublicationPath := publicationStatePath
	originalOwnershipPath := subscriptionOwnershipPath
	originalLockPath := subscriptionLockPath
	subscriptionConfigPath = filepath.Join(root, "subscription.json")
	subscriptionSecretPath = filepath.Join(root, "secrets", "subscription.json")
	publicationStatePath = filepath.Join(root, "subscription-state.json")
	subscriptionOwnershipPath = filepath.Join(root, "ownership.json")
	subscriptionLockPath = filepath.Join(root, "locks", "vpskit.lock")
	t.Cleanup(func() {
		subscriptionConfigPath = originalConfigPath
		subscriptionSecretPath = originalSecretPath
		publicationStatePath = originalPublicationPath
		subscriptionOwnershipPath = originalOwnershipPath
		subscriptionLockPath = originalLockPath
	})
	if err := os.MkdirAll(filepath.Dir(subscriptionLockPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(subscriptionSecretPath), 0o700); err != nil {
		t.Fatal(err)
	}
	ownership := []byte("{\n  \"files\": [],\n  \"directories\": [],\n  \"services\": []\n}\n")
	if err := os.WriteFile(subscriptionOwnershipPath, ownership, 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func ownershipContains(document map[string]any, key, wanted string) bool {
	values, _ := document[key].([]any)
	for _, item := range values {
		if item == wanted {
			return true
		}
	}
	return false
}
