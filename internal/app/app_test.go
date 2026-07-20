package app

import (
	"archive/zip"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/release"
)

func TestClientBundleContainsRevisionManifestAndPrivateFiles(t *testing.T) {
	root := t.TempDir()
	sources := make([]string, 0, 3)
	for name, content := range map[string]string{
		"mihomo.yaml":           "proxies: []\n",
		"sing-box-reality.json": "{}\n",
		"share-links.txt":       "vless://example\n",
	} {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		sources = append(sources, path)
	}
	bundlePath := filepath.Join(root, "client.zip")
	generatedAt := time.Date(2026, 7, 20, 1, 2, 3, 0, time.UTC)
	manifest, err := writeClientBundle(bundlePath, sources, model.State{Profile: "reality-only", ConfigRevision: 7}, generatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ConfigRevision != 7 || len(manifest.Files) != 3 {
		t.Fatalf("unexpected client bundle manifest: %#v", manifest)
	}
	info, err := os.Stat(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("client bundle permissions are %o", info.Mode().Perm())
	}
	reader, err := zip.OpenReader(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	entries := make(map[string]bool)
	for _, entry := range reader.File {
		entries[entry.Name] = true
	}
	for _, name := range []string{"mihomo.yaml", "sing-box-reality.json", "share-links.txt", "manifest.json", "README.txt"} {
		if !entries[name] {
			t.Fatalf("client bundle is missing %s: %#v", name, entries)
		}
	}
	if _, err := writeClientBundle(bundlePath, sources, model.State{Profile: "reality-only", ConfigRevision: 7}, generatedAt); err == nil {
		t.Fatal("client bundle overwrite was not refused")
	}
}

func TestClientFacingChangesDescribeTargetAndPort(t *testing.T) {
	previous := model.State{
		RealityServerName: "www.amazon.com",
		Reality:           model.RealityState{Enabled: true, ID: "reality-main", ListenPort: 443},
		Hysteria2:         model.Hysteria2State{Enabled: true, ID: "hy2-backup", ListenPort: 443},
	}
	updated := previous
	updated.RealityServerName = "www.microsoft.com"
	updated.Reality.ListenPort = 8443
	changes := clientFacingChanges(previous, updated)
	if strings.Join(changes, ",") != "reality.port,reality.server_name" {
		t.Fatalf("unexpected client-facing change list: %#v", changes)
	}
}

func TestStateSchemaFourStartsAtClientConfigRevisionOne(t *testing.T) {
	state, err := decodeInstalledState([]byte(`{
  "schema_version": 4,
  "profile": "balanced",
  "reality": {"enabled": true, "id": "reality-main"},
  "hysteria2": {"enabled": true, "id": "hy2-backup"}
}`))
	if err != nil {
		t.Fatal(err)
	}
	if state.SchemaVersion != model.SchemaVersion || state.ConfigRevision != 1 {
		t.Fatalf("unexpected migrated client config revision: %#v", state)
	}
}

func TestLegoRunArgumentsUseCommandScopedFlags(t *testing.T) {
	arguments := legoRunArguments("node.example.com", "/tmp/lego")
	if len(arguments) == 0 || arguments[0] != "run" {
		t.Fatalf("lego v5 command must precede run-scoped flags: %#v", arguments)
	}
	wanted := map[string]bool{
		"--accept-tos":    false,
		"--dns":           false,
		"--dns.resolvers": false,
		"--domains":       false,
		"--path":          false,
	}
	for _, argument := range arguments[1:] {
		if _, ok := wanted[argument]; ok {
			wanted[argument] = true
		}
	}
	for flag, found := range wanted {
		if !found {
			t.Fatalf("missing lego run flag %s in %#v", flag, arguments)
		}
	}
}

func TestZeroSSLEABUsesEnvironmentInsteadOfProcessArguments(t *testing.T) {
	config := acmeConfig{
		Server:  zeroSSLACMEServer,
		Email:   "ops@example.com",
		EABKID:  "kid-value",
		EABHMAC: "hmac-value",
	}
	arguments := config.legoRunArguments("node.example.com", "/tmp/lego")
	joined := strings.Join(arguments, " ")
	for _, wanted := range []string{
		"--server " + zeroSSLACMEServer,
		"--email ops@example.com",
	} {
		if !strings.Contains(joined, wanted) {
			t.Fatalf("ZeroSSL lego arguments are missing %q: %#v", wanted, arguments)
		}
	}
	if strings.Contains(joined, "kid-value") || strings.Contains(joined, "hmac-value") || strings.Contains(joined, "--eab") {
		t.Fatalf("ZeroSSL EAB credentials must not appear in process arguments: %#v", arguments)
	}
	environment := config.legoEnvironment([]string{"PATH=/bin", "LEGO_EAB_HMAC=old"}, "cloudflare-value")
	joinedEnvironment := strings.Join(environment, "\n")
	for _, wanted := range []string{"CF_DNS_API_TOKEN=cloudflare-value", "LEGO_EAB=true", "LEGO_EAB_KID=kid-value", "LEGO_EAB_HMAC=hmac-value"} {
		if !strings.Contains(joinedEnvironment, wanted) {
			t.Fatalf("ZeroSSL lego environment is missing %q: %#v", wanted, environment)
		}
	}
	autoArguments := (acmeConfig{Server: zeroSSLACMEServer, Email: "ops@example.com"}).legoRunArguments("node.example.com", "/tmp/lego")
	if strings.Contains(strings.Join(autoArguments, " "), "--eab") {
		t.Fatalf("email-based ZeroSSL registration should let lego obtain EAB automatically: %#v", autoArguments)
	}
}

func TestReadACMEConfigAcceptsZeroSSLEABFromEnvironment(t *testing.T) {
	t.Setenv(acmeServerEnvKey, "zerossl")
	t.Setenv(acmeEmailEnvKey, "ops@example.com")
	t.Setenv(acmeEABKIDEnvKey, "kid-value")
	t.Setenv(acmeEABHMACEnvKey, "hmac-value")
	config, err := readACMEConfig()
	if err != nil {
		t.Fatalf("ZeroSSL configuration should validate: %v", err)
	}
	if config.Server != zeroSSLACMEServer || config.Email != "ops@example.com" {
		t.Fatalf("unexpected normalized ZeroSSL configuration: %#v", config)
	}

	t.Setenv(acmeEABKIDEnvKey, "")
	t.Setenv(acmeEABHMACEnvKey, "")
	if _, err := readACMEConfig(); err != nil {
		t.Fatalf("ZeroSSL should allow lego to obtain EAB from the configured email: %v", err)
	}

	t.Setenv(acmeEmailEnvKey, "")
	if _, err := readACMEConfig(); err == nil {
		t.Fatal("ZeroSSL configuration without EAB or email should fail")
	}
}

func TestValidateRealityKeyPair(t *testing.T) {
	privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	privateValue := base64.RawURLEncoding.EncodeToString(privateKey.Bytes())
	publicValue := base64.RawURLEncoding.EncodeToString(privateKey.PublicKey().Bytes())
	if err := validateRealityKeyPair(privateValue, publicValue); err != nil {
		t.Fatalf("generated key pair should validate: %v", err)
	}
	otherPrivate, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherPublic := base64.RawURLEncoding.EncodeToString(otherPrivate.PublicKey().Bytes())
	if err := validateRealityKeyPair(privateValue, otherPublic); err == nil {
		t.Fatal("mismatched Reality key pair should be rejected")
	}
}

func TestSingBoxRealityKeyPairOutput(t *testing.T) {
	singBoxPath := os.Getenv("VPSKIT_SING_BOX_TEST_BIN")
	if singBoxPath == "" {
		t.Skip("VPSKIT_SING_BOX_TEST_BIN is not set")
	}
	output, err := exec.Command(singBoxPath, "generate", "reality-keypair").CombinedOutput()
	if err != nil {
		t.Fatalf("sing-box key generation failed: %v", err)
	}
	privateValue := extractPrefixedValue(string(output), "PrivateKey:")
	publicValue := extractPrefixedValue(string(output), "PublicKey:")
	if err := validateRealityKeyPair(privateValue, publicValue); err != nil {
		t.Fatalf("sing-box generated an invalid Reality key pair: %v", err)
	}
}

func TestSystemdUnitAllowsOnlyRequiredAddressFamilies(t *testing.T) {
	const wanted = "RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK"
	unit := systemdUnit()
	if !strings.Contains(unit, wanted) {
		t.Fatalf("systemd unit is missing the required netlink address family: %s", unit)
	}
	if strings.Contains(unit, "AF_PACKET") {
		t.Fatal("systemd unit should not grant an unused packet socket address family")
	}
}

func TestServiceGroupPathsCoverManagedReadAncestors(t *testing.T) {
	grouped := make(map[string]bool)
	for _, path := range serviceGroupPaths() {
		grouped[path] = true
	}
	for _, managedPath := range []string{serverConfigPath, managedCertificate, managedKey} {
		for parent := path.Dir(managedPath); parent != "/" && parent != "."; parent = path.Dir(parent) {
			if parent == "/etc" || parent == "/var" || parent == "/var/lib" {
				continue
			}
			if !grouped[parent] {
				t.Fatalf("managed service path %s has ungrouped ancestor %s", managedPath, parent)
			}
		}
	}
}

func TestCertificateDirectoryIsPartOfServiceReadBoundary(t *testing.T) {
	grouped := make(map[string]bool)
	for _, managedPath := range serviceGroupPaths() {
		grouped[managedPath] = true
	}
	if !grouped[certificateRoot] || !grouped[managedCertificate] || !grouped[managedKey] {
		t.Fatalf("certificate read boundary is incomplete: %#v", grouped)
	}
}

func TestSplitRealityDomainTarget(t *testing.T) {
	cases := []struct {
		input string
		host  string
		port  int
	}{
		{"www.amazon.com", "www.amazon.com", 443},
		{"WWW.AMAZON.COM:8443", "www.amazon.com", 8443},
	}
	for _, testCase := range cases {
		host, port, err := splitRealityDomainTarget(testCase.input)
		if err != nil {
			t.Fatalf("splitRealityDomainTarget(%q) returned an error: %v", testCase.input, err)
		}
		if host != testCase.host || port != testCase.port {
			t.Fatalf("splitRealityDomainTarget(%q) = (%q, %d), want (%q, %d)", testCase.input, host, port, testCase.host, testCase.port)
		}
	}
	for _, input := range []string{"", "127.0.0.1:443", "10.0.0.0/24", "bad host", "example.com:0"} {
		if _, _, err := splitRealityDomainTarget(input); err == nil {
			t.Fatalf("splitRealityDomainTarget(%q) should fail", input)
		}
	}
}

func TestRealityTLSCandidateRequiresAllSignals(t *testing.T) {
	if !isRealityTLSCandidate(tls.VersionTLS13, "h2", tls.X25519, true) {
		t.Fatal("TLS 1.3, h2, X25519 and a trusted certificate should be a candidate")
	}
	if !isRealityTLSCandidate(tls.VersionTLS13, "h2", tls.X25519MLKEM768, true) {
		t.Fatal("X25519MLKEM768 should be a candidate key exchange")
	}
	for _, testCase := range []struct {
		version uint16
		alpn    string
		curve   tls.CurveID
		trusted bool
	}{
		{tls.VersionTLS12, "h2", tls.X25519, true},
		{tls.VersionTLS13, "http/1.1", tls.X25519, true},
		{tls.VersionTLS13, "h2", tls.CurveP256, true},
		{tls.VersionTLS13, "h2", tls.X25519, false},
	} {
		if isRealityTLSCandidate(testCase.version, testCase.alpn, testCase.curve, testCase.trusted) {
			t.Fatalf("incomplete TLS candidate signals unexpectedly passed: %#v", testCase)
		}
	}
}

func TestRealityVerificationConfigsBindTargetAndLoopback(t *testing.T) {
	server, client, err := buildRealityVerificationConfigs("www.amazon.com", 443, 14443, 17777)
	if err != nil {
		t.Fatal(err)
	}
	serverBytes, err := json.Marshal(server)
	if err != nil {
		t.Fatal(err)
	}
	clientBytes, err := json.Marshal(client)
	if err != nil {
		t.Fatal(err)
	}
	serverText := string(serverBytes)
	clientText := string(clientBytes)
	for _, wanted := range []string{`"server":"www.amazon.com"`, `"server_port":443`, `"listen":"127.0.0.1"`, `"listen_port":14443`} {
		if !strings.Contains(serverText, wanted) {
			t.Fatalf("temporary server config is missing %s: %s", wanted, serverText)
		}
	}
	for _, wanted := range []string{`"server_name":"www.amazon.com"`, `"server":"127.0.0.1"`, `"listen_port":17777`, `"fingerprint":"chrome"`} {
		if !strings.Contains(clientText, wanted) {
			t.Fatalf("temporary client config is missing %s: %s", wanted, clientText)
		}
	}
}

func TestLegoRenewArgumentsUseRunAndRenewFlags(t *testing.T) {
	config := acmeConfig{Server: defaultACMEServer}
	arguments := legoRenewArguments("hy2.example.com", "/var/lib/vpskit/lego", config, false)
	if len(arguments) == 0 || arguments[0] != "run" {
		t.Fatalf("lego v5 renewal must use the run command: %#v", arguments)
	}
	joined := strings.Join(arguments, " ")
	if !strings.Contains(joined, "--renew-days 30") {
		t.Fatalf("renewal window is missing: %#v", arguments)
	}
	if strings.Contains(joined, "--renew-force") {
		t.Fatalf("normal renewal must not force ACME issuance: %#v", arguments)
	}
	forced := strings.Join(legoRenewArguments("hy2.example.com", "/var/lib/vpskit/lego", config, true), " ")
	if !strings.Contains(forced, "--renew-force") {
		t.Fatalf("forced renewal flag is missing: %s", forced)
	}
}

func TestCertificateRenewalUnitsAreHardenedAndPersistent(t *testing.T) {
	service := certificateRenewServiceUnit()
	for _, wanted := range []string{
		"Type=oneshot",
		"ExecStart=/usr/local/bin/vpskit cert renew",
		"ProtectSystem=strict",
		"ReadWritePaths=/var/lib/vpskit /var/log/vpskit",
	} {
		if !strings.Contains(service, wanted) {
			t.Fatalf("certificate renewal service is missing %q: %s", wanted, service)
		}
	}
	timer := certificateRenewTimerUnit()
	for _, wanted := range []string{
		"OnCalendar=*-*-* 03:17:00",
		"RandomizedDelaySec=1h",
		"Persistent=true",
		"Unit=vpskit-certificate-renew.service",
	} {
		if !strings.Contains(timer, wanted) {
			t.Fatalf("certificate renewal timer is missing %q: %s", wanted, timer)
		}
	}
}

func TestAppendOwnershipValueIsIdempotent(t *testing.T) {
	document := map[string]any{"files": []any{installedBinary}}
	appendOwnershipValue(document, "files", installedBinary)
	appendOwnershipValue(document, "files", certificateRenewTimerUnitPath)
	values, ok := document["files"].([]any)
	if !ok || len(values) != 2 {
		t.Fatalf("unexpected ownership values: %#v", document["files"])
	}
	if values[1] != certificateRenewTimerUnitPath {
		t.Fatalf("timer path was not appended: %#v", values)
	}
}

func TestUpdateUsageRequiresBundleDirectory(t *testing.T) {
	if err := runUpdate([]string{"self"}, "public-key"); err == nil || !strings.Contains(err.Error(), "--bundle-dir is required") {
		t.Fatalf("runUpdate should require a bundle directory, got %v", err)
	}
	if err := runUpdate([]string{"core"}, "public-key"); err == nil || !strings.Contains(err.Error(), "--bundle-dir is required") {
		t.Fatalf("core update should require a bundle directory, got %v", err)
	}
}

func TestStateSchemaOneMigratesInMemory(t *testing.T) {
	input := []byte(`{
  "schema_version": 1,
  "domain": "node.example.com",
  "core": {"id":"sing-box","version":"1.13.14","path":"/usr/local/lib/vpskit/bin/sing-box","sha256":"abc"},
  "hysteria2": {"certificate_path":"/var/lib/vpskit/certificates/hysteria2.crt","key_path":"/var/lib/vpskit/certificates/hysteria2.key"}
}`)
	state, err := decodeInstalledState(input)
	if err != nil {
		t.Fatal(err)
	}
	if state.SchemaVersion != model.SchemaVersion || state.Core.Channel != "pinned" || !state.Reality.Enabled || !state.Hysteria2.Enabled {
		t.Fatalf("unexpected migrated state: %#v", state)
	}
}

func TestStateSchemaTwoAddsExplicitInboundState(t *testing.T) {
	state, err := decodeInstalledState([]byte(`{
  "schema_version": 2,
  "profile": "balanced",
  "reality": {"id": "reality-main"},
  "hysteria2": {"id": "hy2-backup"}
}`))
	if err != nil {
		t.Fatal(err)
	}
	if state.SchemaVersion != model.SchemaVersion || state.RealityCore.ID != "xray" || !state.Reality.Enabled || !state.Hysteria2.Enabled {
		t.Fatalf("unexpected migrated state: %#v", state)
	}
}

func TestUnknownFutureStateSchemaIsRejected(t *testing.T) {
	input := []byte(`{"schema_version":999,"domain":"node.example.com"}`)
	if _, err := decodeInstalledState(input); err == nil || !strings.Contains(err.Error(), "refusing to rewrite") {
		t.Fatalf("future schema should be rejected explicitly, got %v", err)
	}
}

func TestStableCoreAssetRequiresLockedOfficialMetadata(t *testing.T) {
	asset := release.Asset{
		ID:             "sing-box",
		Version:        "1.13.14",
		Channel:        "stable",
		SourceURL:      "https://github.com/SagerNet/sing-box/releases/tag/v1.13.14",
		SourceRepo:     "SagerNet/sing-box",
		SourceRef:      "v1.13.14",
		SourceCommit:   "25a600db24f7680ad9806ce5427bd0ab8afe1114",
		StateSchemaMin: 1,
	}
	if err := validateCoreAsset(asset, "stable", "1.13.14"); err != nil {
		t.Fatalf("locked stable asset should validate: %v", err)
	}
	asset.SourceRef = "v1.13.13"
	if err := validateCoreAsset(asset, "stable", "1.13.14"); err == nil {
		t.Fatal("tag/version mismatch should be rejected")
	}
}

func TestStableChannelRejectsPrereleaseAndDowngrade(t *testing.T) {
	asset := release.Asset{
		ID:             "sing-box",
		Version:        "1.14.0-alpha.47",
		Channel:        "stable",
		SourceURL:      "https://github.com/SagerNet/sing-box/releases/tag/v1.14.0-alpha.47",
		SourceRepo:     "SagerNet/sing-box",
		SourceRef:      "v1.14.0-alpha.47",
		SourceCommit:   "25a600db24f7680ad9806ce5427bd0ab8afe1114",
		StateSchemaMin: 1,
	}
	if err := validateCoreAsset(asset, "stable", "1.13.14"); err == nil || !strings.Contains(err.Error(), "prerelease") {
		t.Fatalf("stable prerelease should fail, got %v", err)
	}
	asset.Version = "1.13.13"
	asset.SourceRef = "v1.13.13"
	asset.SourceURL = "https://github.com/SagerNet/sing-box/releases/tag/v1.13.13"
	if err := validateCoreAsset(asset, "stable", "1.13.14"); err == nil || !strings.Contains(err.Error(), "downgrade") {
		t.Fatalf("downgrade should fail, got %v", err)
	}
}

func TestPrereleaseComparisonUsesSemanticNumericIdentifiers(t *testing.T) {
	alphaNine, err := parseCoreVersion("1.14.0-alpha.9")
	if err != nil {
		t.Fatal(err)
	}
	alphaTen, err := parseCoreVersion("1.14.0-alpha.10")
	if err != nil {
		t.Fatal(err)
	}
	if compareCoreVersion(alphaNine, alphaTen) >= 0 {
		t.Fatal("alpha.9 must sort before alpha.10")
	}
}

func TestVersionsLockSeparatesArchiveAndBinaryDigests(t *testing.T) {
	directory := t.TempDir()
	binaryDigest := strings.Repeat("a", 64)
	xrayDigest := strings.Repeat("e", 64)
	archiveDigest := strings.Repeat("b", 64)
	lock := `{
  "schema_version": 1,
  "generated_at": "2026-07-18T08:00:00Z",
  "template_revision": 1,
  "assets": [
    {
      "id":"sing-box","version":"1.13.14","channel":"stable","source_repo":"SagerNet/sing-box","source_ref":"v1.13.14","source_commit":"25a600db24f7680ad9806ce5427bd0ab8afe1114","source_url":"https://github.com/SagerNet/sing-box/releases/tag/v1.13.14",
      "source_archive":{"name":"sing-box-1.13.14-linux-amd64.tar.gz","size":23832905,"sha256":"` + archiveDigest + `"},
      "installed_file":"sing-box","binary_sha256":"` + binaryDigest + `","verified_at":"2026-07-18","state_schema_min":1
    },
    {
	  "id":"xray","version":"26.3.27","channel":"stable","source_repo":"XTLS/Xray-core","source_ref":"v26.3.27","source_commit":"d2758a023cd7f4174a5a5fa4ff66e487d4342ba0","source_url":"https://github.com/XTLS/Xray-core/releases/tag/v26.3.27",
	  "source_archive":{"name":"Xray-linux-64.zip","size":21136402,"sha256":"` + strings.Repeat("e", 64) + `"},
	  "installed_file":"xray","binary_sha256":"` + xrayDigest + `","verified_at":"2026-07-19","state_schema_min":4
	},
	{
      "id":"lego","version":"5.2.2","channel":"stable","source_repo":"go-acme/lego","source_ref":"v5.2.2","source_commit":"3d5a6695e027d625bd34334d516d77f578d43f11","source_url":"https://github.com/go-acme/lego/releases/tag/v5.2.2",
      "source_archive":{"name":"lego_v5.2.2_linux_amd64.tar.gz","size":21076129,"sha256":"` + strings.Repeat("c", 64) + `"},
      "installed_file":"lego","binary_sha256":"` + strings.Repeat("d", 64) + `","verified_at":"2026-07-18","state_schema_min":1
    }
  ]
}`
	if err := os.WriteFile(path.Join(directory, "versions.lock"), []byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}
	document, err := validateVersionsLock(
		directory,
		release.Asset{ID: "versions-lock", File: "versions.lock"},
		release.Asset{ID: "sing-box", Version: "1.13.14", SHA256: binaryDigest, SourceURL: "https://github.com/SagerNet/sing-box/releases/tag/v1.13.14", SourceRepo: "SagerNet/sing-box", SourceRef: "v1.13.14", SourceCommit: "25a600db24f7680ad9806ce5427bd0ab8afe1114"},
		release.Asset{ID: "xray", Version: "26.3.27", SHA256: xrayDigest, SourceURL: "https://github.com/XTLS/Xray-core/releases/tag/v26.3.27", SourceRepo: "XTLS/Xray-core", SourceRef: "v26.3.27", SourceCommit: "d2758a023cd7f4174a5a5fa4ff66e487d4342ba0"},
		release.Asset{ID: "lego", Version: "5.2.2", SHA256: strings.Repeat("d", 64), SourceURL: "https://github.com/go-acme/lego/releases/tag/v5.2.2", SourceRepo: "go-acme/lego", SourceRef: "v5.2.2", SourceCommit: "3d5a6695e027d625bd34334d516d77f578d43f11"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if document.Assets[0].SourceArchive.SHA256 == document.Assets[0].BinarySHA256 {
		t.Fatal("source archive and extracted binary digests must remain distinct")
	}
}

func TestOrphanScopeReportsOnlyUnexpectedDirectChildren(t *testing.T) {
	directory := t.TempDir()
	expected := filepath.Join(directory, "state.json")
	unexpected := filepath.Join(directory, "state.json.old")
	if err := os.WriteFile(expected, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unexpected, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	candidates, err := unexpectedDirectChildren(directory, pathSet(expected))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].Path != unexpected {
		t.Fatalf("unexpected orphan candidates: %#v", candidates)
	}
	if _, err := os.Stat(unexpected); err != nil {
		t.Fatalf("orphan scan must remain read-only: %v", err)
	}
}

func TestExternalCertificateReferenceIsDetected(t *testing.T) {
	directory := t.TempDir()
	unit := filepath.Join(directory, "consumer.service")
	content := "[Service]\nEnvironment=CERT=" + managedCertificate + "\n"
	if err := os.WriteFile(unit, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	references, err := externalCertificateReferences(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(references) != 1 || references[0] != unit {
		t.Fatalf("certificate consumer was not detected: %#v", references)
	}
	if filtered := withoutPaths([]string{managedCertificate, statePath}, managedCertificate); len(filtered) != 1 || filtered[0] != statePath {
		t.Fatalf("unexpected preservation filter result: %#v", filtered)
	}
}

func TestAutomaticRecoveryRunsOnlyBeforeMutatingCommands(t *testing.T) {
	for _, command := range []string{"update", "cert", "backup", "install", "instance", "uninstall"} {
		if !shouldAutoRecover(command) {
			t.Fatalf("%s should trigger incomplete mutation recovery", command)
		}
	}
	for _, command := range []string{"status", "doctor", "orphan", "bundle", "version", "export", "rollback", "restore"} {
		if shouldAutoRecover(command) {
			t.Fatalf("read-only or explicit recovery command %s must not trigger automatic recovery", command)
		}
	}
}

func TestBackupPruningNeverSelectsNewProtectedRecoveryPoint(t *testing.T) {
	ids := []string{"BK-old", "BK-current"}
	protected := map[string]bool{"BK-old": true, "BK-current": true}
	if index := oldestRemovableBackup(ids, protected); index != -1 {
		t.Fatalf("protected recovery point was selected for pruning: %d", index)
	}
	delete(protected, "BK-old")
	if index := oldestRemovableBackup(ids, protected); index != 0 {
		t.Fatalf("oldest unprotected backup should be selected, got %d", index)
	}
}

func TestRestoreRequiresExplicitConfirmation(t *testing.T) {
	if err := runRestore([]string{"BK-test"}); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("runRestore should require explicit confirmation, got %v", err)
	}
}

func TestBackupPathValidationRejectsTraversal(t *testing.T) {
	if validBackupRelative("lego/../state.json") {
		t.Fatal("backup path traversal was accepted")
	}
	if !validBackupRelative("lego/certificates/example.crt") {
		t.Fatal("valid backup path was rejected")
	}
}

func TestBackupValidationUsesStagedCertificatePaths(t *testing.T) {
	configuration := []byte(`{"inbounds":[{"type":"hysteria2","tls":{"certificate_path":"/var/lib/vpskit/certificates/hysteria2.crt","key_path":"/var/lib/vpskit/certificates/hysteria2.key"}}]}`)
	updated, err := configWithStagedCertificatePaths(configuration, "/tmp/staging/hysteria2.crt", "/tmp/staging/hysteria2.key")
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	if !strings.Contains(text, "/tmp/staging/hysteria2.crt") || !strings.Contains(text, "/tmp/staging/hysteria2.key") {
		t.Fatalf("staged certificate paths were not applied: %s", text)
	}
	if strings.Contains(text, "/var/lib/vpskit/certificates") {
		t.Fatalf("managed certificate paths remained in staging validation config: %s", text)
	}
}

func TestBackupValidationRequiresHysteria2Inbound(t *testing.T) {
	if _, err := configWithStagedCertificatePaths([]byte(`{"inbounds":[{"type":"vless"}]}`), "/tmp/cert", "/tmp/key"); err == nil {
		t.Fatal("missing Hysteria2 inbound should fail staged certificate rewrite")
	}
}

func TestManagedBackupDoesNotCopyCloudflareToken(t *testing.T) {
	for _, entry := range managedBackupSources() {
		switch entry.source {
		case cloudflareEnvPath:
			t.Fatal("managed backup must not duplicate the Cloudflare token")
		case acmeEnvPath:
			t.Fatal("managed backup must not duplicate the ACME EAB")
		}
	}
}

func TestRollbackRequiresExplicitConfirmation(t *testing.T) {
	if err := runRollback([]string{"TX-test"}); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("runRollback should require explicit confirmation, got %v", err)
	}
}

func TestInstanceRollbackRefusesCrossVersionBackup(t *testing.T) {
	if err := validateInstanceRollbackVersions("v0.1.0-lab.27", "v0.1.0-lab.26"); err == nil {
		t.Fatal("cross-version instance rollback should require explicit full restore")
	}
	if err := validateInstanceRollbackVersions("v0.1.0-lab.27", "v0.1.0-lab.27"); err != nil {
		t.Fatalf("same-version instance rollback should pass: %v", err)
	}
}

func TestUninstallRequiresExplicitConfirmation(t *testing.T) {
	if err := runUninstall(nil); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("runUninstall should require explicit confirmation, got %v", err)
	}
}

func TestInstanceStateChangesPreserveAtLeastOneEnabledInbound(t *testing.T) {
	state := model.State{
		Reality:   model.RealityState{Enabled: true, ID: "reality-main", ListenPort: 443},
		Hysteria2: model.Hysteria2State{Enabled: true, ID: "hy2-backup", ListenPort: 443},
	}
	secrets := model.Secrets{RealityUUID: "uuid", RealityPrivateKey: "private", Hysteria2Password: "password"}
	if err := applyInstanceStateChange(&state, &secrets, "disable", "hysteria2", 0); err != nil {
		t.Fatal(err)
	}
	if state.Hysteria2.Enabled || !state.Reality.Enabled {
		t.Fatalf("unexpected disabled state: %#v", state)
	}
	if err := applyInstanceStateChange(&state, &secrets, "disable", "reality", 0); err == nil {
		t.Fatal("expected disabling the last enabled instance to fail")
	}
	if err := applyInstanceStateChange(&state, &secrets, "enable", "hysteria2", 0); err != nil {
		t.Fatal(err)
	}
	if err := applyInstanceStateChange(&state, &secrets, "modify", "hysteria2", 8443); err != nil {
		t.Fatal(err)
	}
	if state.Hysteria2.ListenPort != 8443 {
		t.Fatalf("Hysteria2 port was not changed: %#v", state.Hysteria2)
	}
}

func TestInstanceDeleteClearsOnlySelectedSecret(t *testing.T) {
	state := model.State{
		Reality:   model.RealityState{Enabled: true, ID: "reality-main"},
		Hysteria2: model.Hysteria2State{Enabled: true, ID: "hy2-backup"},
	}
	secrets := model.Secrets{RealityUUID: "uuid", RealityPrivateKey: "private", Hysteria2Password: "password"}
	if err := applyInstanceStateChange(&state, &secrets, "delete", "hysteria2", 0); err != nil {
		t.Fatal(err)
	}
	if state.Hysteria2.ID != "" || secrets.Hysteria2Password != "" {
		t.Fatalf("Hysteria2 deletion was incomplete: state=%#v secrets=%#v", state.Hysteria2, secrets)
	}
	if state.Reality.ID == "" || secrets.RealityUUID == "" || secrets.RealityPrivateKey == "" {
		t.Fatal("Reality data was changed by Hysteria2 deletion")
	}
}

func TestInstanceDeleteRefusesLastEnabledInbound(t *testing.T) {
	state := model.State{
		Reality:   model.RealityState{Enabled: true, ID: "reality-main"},
		Hysteria2: model.Hysteria2State{Enabled: false, ID: "hy2-backup"},
	}
	secrets := model.Secrets{RealityUUID: "uuid", RealityPrivateKey: "private", Hysteria2Password: "password"}
	if err := applyInstanceStateChange(&state, &secrets, "delete", "reality", 0); err == nil {
		t.Fatal("expected deleting the last enabled inbound to fail")
	}
	if state.Reality.ID == "" || secrets.RealityUUID == "" || secrets.RealityPrivateKey == "" {
		t.Fatal("failed deletion changed Reality state or secrets")
	}
}

func TestRealityOnlyInstallOptionsDoNotRequireCertificateInputs(t *testing.T) {
	valid := InstallOptions{
		BundleDir:     "/tmp/vpskit-bundle",
		ConnectHost:   "203.0.113.10",
		RealityTarget: "www.amazon.com",
		TCPPort:       443,
	}
	if err := validateRealityOnlyInstallOptions(valid); err != nil {
		t.Fatalf("valid Reality-only options should pass without a domain or UDP port: %v", err)
	}
	for name, mutate := range map[string]func(*InstallOptions){
		"bundle":   func(value *InstallOptions) { value.BundleDir = "" },
		"host":     func(value *InstallOptions) { value.ConnectHost = "bad host" },
		"target":   func(value *InstallOptions) { value.RealityTarget = "127.0.0.1" },
		"tcp-port": func(value *InstallOptions) { value.TCPPort = 0 },
	} {
		value := valid
		mutate(&value)
		if err := validateRealityOnlyInstallOptions(value); err == nil {
			t.Fatalf("invalid Reality-only %s was accepted", name)
		}
	}
}

func TestManagedCertificateValidationIsSeparateFromGeneralState(t *testing.T) {
	realityOnly := model.State{Reality: model.RealityState{Enabled: true, ID: "reality-main"}}
	if err := validateManagedCertificateState(realityOnly); err == nil {
		t.Fatal("Reality-only state should not describe a managed Hysteria2 certificate")
	}
	balanced := model.State{
		Domain:  "node.example.com",
		Reality: model.RealityState{Enabled: true, ID: "reality-main"},
		Hysteria2: model.Hysteria2State{
			Enabled: true, ID: "hy2-backup", CertificatePath: managedCertificate, KeyPath: managedKey,
		},
	}
	if err := validateManagedCertificateState(balanced); err != nil {
		t.Fatalf("balanced managed-certificate state should validate: %v", err)
	}
}

func TestRealityOnlyArtifactsExcludeHysteria2Everywhere(t *testing.T) {
	state := model.State{
		Profile:           "reality-only",
		ConnectHost:       "203.0.113.10",
		RealityServerName: "www.amazon.com",
		Reality: model.RealityState{
			Enabled: true, ID: "reality-main", ListenPort: 443,
			PublicKey: "public-key", ShortID: "0123456789abcdef",
		},
	}
	secrets := model.Secrets{RealityUUID: "11111111-1111-4111-8111-111111111111", RealityPrivateKey: "private-key"}
	artifacts, err := renderProfileArtifacts(state, secrets)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := artifacts[filepath.Join(exportRoot, "sing-box-hysteria2.json")]; ok {
		t.Fatal("Reality-only artifacts unexpectedly contain a Hysteria2 client")
	}
	for path, content := range artifacts {
		if strings.Contains(strings.ToLower(string(content)), "hysteria2") {
			t.Fatalf("Reality-only artifact %s contains Hysteria2 data", path)
		}
	}
	exports := exportStateForProfile(state)
	if len(exports) != 3 {
		t.Fatalf("Reality-only export state should contain Mihomo, Reality and share links: %#v", exports)
	}
}

func TestTerminalQRCodeOutputIsDeterministicAndContainsNoANSI(t *testing.T) {
	links := "vless://example-reality\nhysteria2://example-hysteria2\n"
	first, err := renderTerminalQRCodes(links)
	if err != nil {
		t.Fatal(err)
	}
	second, err := renderTerminalQRCodes(links)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("QR rendering is not deterministic")
	}
	if !strings.Contains(first, "VPSKit REALITY QR") || !strings.Contains(first, "VPSKit HYSTERIA2 QR") {
		t.Fatalf("QR output is missing protocol labels: %q", first)
	}
	if strings.Contains(first, "\x1b[") {
		t.Fatal("QR output must not contain terminal control sequences")
	}
	if strings.Contains(first, "vless://") || strings.Contains(first, "hysteria2://") {
		t.Fatal("QR output must not echo raw share links")
	}
}

func TestTerminalQRCodeRejectsUnsupportedSchemes(t *testing.T) {
	if _, err := renderTerminalQRCodes("https://example.com\n"); err == nil {
		t.Fatal("expected unsupported share link scheme to fail")
	}
}

func TestTerminalQRCodeAcceptsRepresentativeProductionLengthLink(t *testing.T) {
	link := "vless://11111111-1111-4111-8111-111111111111@203.0.113.10:443?encryption=none&flow=xtls-rprx-vision&security=reality&sni=www.amazon.com&fp=chrome&pbk=abcdefghijklmnopqrstuvwxyz0123456789ABCDE&sid=0123456789abcdef&type=tcp#JP-Reality"
	output, err := renderTerminalQRCodes(link + "\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "VPSKit REALITY QR") || strings.Contains(output, link) {
		t.Fatal("representative Reality link was not safely rendered as a labeled QR code")
	}
}
