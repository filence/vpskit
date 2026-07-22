package app

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/platform"
	"vpskit.local/vpskit/internal/render"
)

type supportBundleEntry struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type supportBundleManifest struct {
	SchemaVersion int                  `json:"schema_version"`
	GeneratedAt   time.Time            `json:"generated_at"`
	Allowlist     []string             `json:"allowlist"`
	Files         []supportBundleEntry `json:"files"`
}

func runSupport(arguments []string) error {
	if len(arguments) == 0 || arguments[0] != "bundle" {
		return errors.New("usage: vpskit support bundle [--output-dir <absolute-directory>]")
	}
	flags := flag.NewFlagSet("support bundle", flag.ContinueOnError)
	outputDirectory := flags.String("output-dir", "", "existing absolute output directory")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if !platform.IsRoot() {
		return errors.New("support bundle requires root privileges")
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	destination, ownerUID, ownerGID, err := resolveClientBundleDestination(*outputDirectory)
	if err != nil {
		return err
	}
	generatedAt := time.Now().UTC()
	path := filepath.Join(destination, fmt.Sprintf("vpskit-support-%s.zip", generatedAt.Format("20060102-150405")))
	manifest, err := writeSupportBundle(path, state, generatedAt)
	if err != nil {
		return err
	}
	if ownerUID >= 0 && ownerGID >= 0 {
		if err := os.Chown(path, ownerUID, ownerGID); err != nil {
			_ = os.Remove(path)
			return fmt.Errorf("assign support bundle to invoking SSH user: %w", err)
		}
	}
	return printJSON(commandResult{Command: "support bundle", Status: "PASS", Detail: map[string]any{
		"bundle":                 path,
		"mode":                   "0600",
		"files":                  len(manifest.Files),
		"allowlist_only":         true,
		"contains_protocol_keys": false,
		"contains_api_tokens":    false,
	}})
}

func writeSupportBundle(path string, state model.State, generatedAt time.Time) (supportBundleManifest, error) {
	systemReport, err := collectSystemInspect()
	if err != nil {
		return supportBundleManifest{}, err
	}
	files := map[string][]byte{}
	for name, value := range map[string]any{
		"state-redacted.json": redactedSupportState(state),
		"system.json":         systemReport,
		"renderers.json":      render.RendererCapabilities(),
		"transactions.json":   supportTransactionSummaries(transactionRoot, 20),
	} {
		content, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return supportBundleManifest{}, err
		}
		files[name] = append(content, '\n')
	}
	files["README.txt"] = []byte("VPSKit 脱敏诊断包\n\n本包仅包含明确允许的状态摘要、系统摘要、Renderer 能力和事务状态；不包含协议私钥、UUID、密码、Cloudflare Token、域名、连接地址或日志正文。\n")

	manifest := supportBundleManifest{SchemaVersion: 1, GeneratedAt: generatedAt, Allowlist: make([]string, 0, len(files)), Files: make([]supportBundleEntry, 0, len(files))}
	for name, content := range files {
		digest := sha256.Sum256(content)
		manifest.Allowlist = append(manifest.Allowlist, name)
		manifest.Files = append(manifest.Files, supportBundleEntry{Name: name, Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:])})
	}
	sort.Strings(manifest.Allowlist)
	sort.SliceStable(manifest.Files, func(i, j int) bool { return manifest.Files[i].Name < manifest.Files[j].Name })
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return supportBundleManifest{}, err
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return supportBundleManifest{}, fmt.Errorf("create support bundle without overwrite: %w", err)
	}
	archive := zip.NewWriter(file)
	committed := false
	defer func() {
		_ = archive.Close()
		_ = file.Close()
		if !committed {
			_ = os.Remove(path)
		}
	}()
	for _, name := range manifest.Allowlist {
		if err := writeZipEntry(archive, name, files[name], generatedAt); err != nil {
			return supportBundleManifest{}, err
		}
	}
	if err := writeZipEntry(archive, "manifest.json", append(manifestBytes, '\n'), generatedAt); err != nil {
		return supportBundleManifest{}, err
	}
	if err := archive.Close(); err != nil {
		return supportBundleManifest{}, err
	}
	if err := file.Sync(); err != nil {
		return supportBundleManifest{}, err
	}
	if err := file.Close(); err != nil {
		return supportBundleManifest{}, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return supportBundleManifest{}, err
	}
	committed = true
	return manifest, nil
}

func redactedSupportState(state model.State) map[string]any {
	node := map[string]any{
		"node_id":                 state.Node.ID,
		"display_name":            redactIfSet(state.Node.DisplayName),
		"provider":                redactIfSet(state.Node.Provider),
		"country":                 state.Node.Country,
		"city":                    redactIfSet(state.Node.City),
		"priority":                state.Node.Priority,
		"enabled_in_subscription": state.Node.EnabledInSubscription,
		"tags_count":              len(state.Node.Tags),
	}
	return map[string]any{
		"schema_version":      state.SchemaVersion,
		"vpskit_version":      state.VPSKitVersion,
		"transaction_id":      state.TransactionID,
		"installed_at":        state.InstalledAt,
		"profile":             state.Profile,
		"node":                node,
		"connect_host":        "[REDACTED]",
		"domain":              redactIfSet(state.Domain),
		"reality_server_name": redactIfSet(state.RealityServerName),
		"core":                state.Core,
		"reality_core":        state.RealityCore,
		"reality": map[string]any{
			"enabled": state.Reality.Enabled, "id": state.Reality.ID, "listen_port": state.Reality.ListenPort,
			"public_key": "[REDACTED]", "short_id": "[REDACTED]",
		},
		"hysteria2": map[string]any{
			"enabled": state.Hysteria2.Enabled, "id": state.Hysteria2.ID, "listen_port": state.Hysteria2.ListenPort,
			"certificate_authority": state.Hysteria2.CertificateAuthority,
		},
		"firewall":              state.Firewall,
		"config_sha256":         state.ConfigSHA256,
		"reality_config_sha256": state.RealityConfigSHA256,
		"config_revision":       state.ConfigRevision,
		"exports_count":         len(state.Exports),
	}
}

func redactIfSet(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "[REDACTED]"
}

func supportTransactionSummaries(root string, limit int) []map[string]any {
	entries, err := os.ReadDir(root)
	if err != nil {
		return []map[string]any{}
	}
	ids := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "TX-") {
			ids = append(ids, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	if len(ids) > limit {
		ids = ids[:limit]
	}
	result := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		content, err := os.ReadFile(filepath.Join(root, id, "transaction.json"))
		if err != nil {
			continue
		}
		var record map[string]any
		if json.Unmarshal(content, &record) != nil {
			continue
		}
		summary := map[string]any{"transaction_id": id}
		for _, key := range []string{"command", "status", "started_at", "committed_at", "failed_at"} {
			if value, ok := record[key]; ok {
				summary[key] = value
			}
		}
		result = append(result, summary)
	}
	return result
}
