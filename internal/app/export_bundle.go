package app

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/platform"
)

type clientBundleManifest struct {
	SchemaVersion  int                 `json:"schema_version"`
	GeneratedAt    time.Time           `json:"generated_at"`
	Profile        string              `json:"profile"`
	ConfigRevision int                 `json:"config_revision"`
	Files          []clientBundleEntry `json:"files"`
}

type clientBundleEntry struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

func runClientBundleExport(outputDirectory string) error {
	if !platform.IsRoot() {
		return errors.New("client bundle export requires root privileges")
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	sources := clientExportPaths(state)
	destination, ownerUID, ownerGID, err := resolveClientBundleDestination(outputDirectory)
	if err != nil {
		return err
	}
	generatedAt := time.Now().UTC()
	filename := fmt.Sprintf("vpskit-client-r%04d-%s.zip", state.ConfigRevision, generatedAt.Format("20060102-150405"))
	bundlePath := filepath.Join(destination, filename)
	manifest, err := writeClientBundle(bundlePath, sources, state, generatedAt)
	if err != nil {
		return err
	}
	if ownerUID >= 0 && ownerGID >= 0 {
		if err := os.Chown(bundlePath, ownerUID, ownerGID); err != nil {
			_ = os.Remove(bundlePath)
			return fmt.Errorf("assign client bundle to invoking SSH user: %w", err)
		}
	}
	return printJSON(commandResult{Command: "export bundle", Status: "PASS", Detail: map[string]any{
		"bundle":                 bundlePath,
		"mode":                   "0600",
		"profile":                state.Profile,
		"config_revision":        state.ConfigRevision,
		"files":                  len(manifest.Files),
		"contains_credentials":   true,
		"recommended_transfer":   "scp-or-sftp",
		"delete_after_download":  true,
		"client_update_required": false,
	}})
}

func clientExportPaths(state model.State) []string {
	reality, _ := instanceForAdapter(state, "reality")
	hysteria2, _ := instanceForAdapter(state, "hysteria2")
	paths := []string{filepath.Join(exportRoot, "mihomo.yaml")}
	if reality.Enabled {
		paths = append(paths, filepath.Join(exportRoot, "sing-box-reality.json"))
	}
	if hysteria2.Enabled {
		paths = append(paths, filepath.Join(exportRoot, "sing-box-hysteria2.json"))
	}
	return append(paths, filepath.Join(exportRoot, "share-links.txt"))
}

func resolveClientBundleDestination(requested string) (string, int, int, error) {
	destination := strings.TrimSpace(requested)
	ownerUID, ownerGID := -1, -1
	if destination == "" {
		current, err := exportOwner()
		if err != nil {
			return "", -1, -1, err
		}
		destination = current.HomeDir
		ownerUID, ownerGID = current.UID, current.GID
	}
	if !filepath.IsAbs(destination) {
		return "", -1, -1, errors.New("client bundle output directory must be absolute")
	}
	cleaned := filepath.Clean(destination)
	info, err := os.Lstat(cleaned)
	if err != nil {
		return "", -1, -1, fmt.Errorf("inspect client bundle output directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", -1, -1, errors.New("client bundle output directory must be an existing non-symlink directory")
	}
	return cleaned, ownerUID, ownerGID, nil
}

type exportOwnerIdentity struct {
	HomeDir string
	UID     int
	GID     int
}

func exportOwner() (exportOwnerIdentity, error) {
	name := strings.TrimSpace(os.Getenv("SUDO_USER"))
	var account *user.User
	var err error
	if name != "" && name != "root" {
		account, err = user.Lookup(name)
	} else {
		account, err = user.Current()
	}
	if err != nil {
		return exportOwnerIdentity{}, fmt.Errorf("resolve invoking SSH user: %w", err)
	}
	uid, err := strconv.Atoi(account.Uid)
	if err != nil || uid < 0 {
		return exportOwnerIdentity{}, errors.New("invoking SSH user has an invalid UID")
	}
	gid, err := strconv.Atoi(account.Gid)
	if err != nil || gid < 0 {
		return exportOwnerIdentity{}, errors.New("invoking SSH user has an invalid GID")
	}
	if account.HomeDir == "" {
		return exportOwnerIdentity{}, errors.New("invoking SSH user has no home directory")
	}
	return exportOwnerIdentity{HomeDir: account.HomeDir, UID: uid, GID: gid}, nil
}

func writeClientBundle(bundlePath string, sources []string, state model.State, generatedAt time.Time) (manifest clientBundleManifest, returnErr error) {
	file, err := os.OpenFile(bundlePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return clientBundleManifest{}, fmt.Errorf("create client bundle without overwrite: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); returnErr == nil && closeErr != nil {
			returnErr = closeErr
		}
		if returnErr != nil {
			_ = os.Remove(bundlePath)
		}
	}()
	archive := zip.NewWriter(file)
	archiveClosed := false
	defer func() {
		if archiveClosed {
			return
		}
		if closeErr := archive.Close(); returnErr == nil && closeErr != nil {
			returnErr = closeErr
		}
	}()

	manifest = clientBundleManifest{
		SchemaVersion:  1,
		GeneratedAt:    generatedAt,
		Profile:        state.Profile,
		ConfigRevision: state.ConfigRevision,
		Files:          make([]clientBundleEntry, 0, len(sources)),
	}
	for _, source := range sources {
		entry, content, err := readClientExport(source)
		if err != nil {
			return clientBundleManifest{}, err
		}
		if err := writeZipEntry(archive, entry.Name, content, generatedAt); err != nil {
			return clientBundleManifest{}, err
		}
		manifest.Files = append(manifest.Files, entry)
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return clientBundleManifest{}, err
	}
	if err := writeZipEntry(archive, "manifest.json", append(manifestBytes, '\n'), generatedAt); err != nil {
		return clientBundleManifest{}, err
	}
	if err := writeZipEntry(archive, "README.txt", clientBundleReadme(state), generatedAt); err != nil {
		return clientBundleManifest{}, err
	}
	if err := archive.Close(); err != nil {
		return clientBundleManifest{}, fmt.Errorf("finalize client bundle: %w", err)
	}
	archiveClosed = true
	if err := file.Sync(); err != nil {
		return clientBundleManifest{}, fmt.Errorf("sync client bundle: %w", err)
	}
	if err := os.Chmod(bundlePath, 0o600); err != nil {
		return clientBundleManifest{}, fmt.Errorf("secure client bundle permissions: %w", err)
	}
	return manifest, nil
}

func readClientExport(path string) (clientBundleEntry, []byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return clientBundleEntry{}, nil, fmt.Errorf("inspect client export %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return clientBundleEntry{}, nil, fmt.Errorf("refusing non-regular client export: %s", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return clientBundleEntry{}, nil, fmt.Errorf("read client export %s: %w", path, err)
	}
	digest := sha256.Sum256(content)
	return clientBundleEntry{Name: filepath.Base(path), Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:])}, content, nil
}

func writeZipEntry(archive *zip.Writer, name string, content []byte, modified time.Time) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o600)
	header.Modified = modified
	entry, err := archive.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create client bundle entry %s: %w", name, err)
	}
	if _, err := io.Copy(entry, bytes.NewReader(content)); err != nil {
		return fmt.Errorf("write client bundle entry %s: %w", name, err)
	}
	return nil
}

func clientBundleReadme(state model.State) []byte {
	text := fmt.Sprintf(`VPSKit 客户端配置包

配置修订：r%04d
部署方案：%s

导入说明：
- Clash Verge Rev：导入 mihomo.yaml。
- Hiddify：从 share-links.txt 复制对应链接，或使用 VPSKit 终端二维码。
- sing-box：按协议导入 sing-box-reality.json 或 sing-box-hysteria2.json。

安全提示：
- 本压缩包包含连接凭据，请勿上传到 GitHub、公开网盘或群聊。
- 导入完成并确认可用后，请删除 VPS 和电脑上不再需要的临时压缩包。
- 配置修订变化后，静态文件和分享链接不会自动更新，需要重新导出并导入。
`, state.ConfigRevision, state.Profile)
	return []byte(text)
}
