package release

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var (
	releaseVersionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$`)
	repositoryPattern     = regexp.MustCompile(`^[0-9A-Za-z_.-]+/[0-9A-Za-z_.-]+$`)
)

func GenerateBootstrap(templatePath, archivePath, outputPath, version, repository string) (returnHash string, returnErr error) {
	if !releaseVersionPattern.MatchString(version) {
		return "", errors.New("release version format is invalid")
	}
	if !repositoryPattern.MatchString(repository) {
		return "", errors.New("GitHub repository must use owner/name format")
	}
	template, err := regularAbsolutePath(templatePath, "bootstrap template")
	if err != nil {
		return "", err
	}
	archive, err := regularAbsolutePath(archivePath, "release archive")
	if err != nil {
		return "", err
	}
	output, err := filepath.Abs(outputPath)
	if err != nil {
		return "", err
	}
	if samePath(output, template) || samePath(output, archive) {
		return "", errors.New("bootstrap output would overwrite an input file")
	}
	if _, err := os.Lstat(output); err == nil {
		return "", fmt.Errorf("refusing to overwrite bootstrap: %s", output)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	outputDirectory := filepath.Dir(output)
	if info, err := os.Stat(outputDirectory); err != nil || !info.IsDir() {
		if err != nil {
			return "", fmt.Errorf("inspect bootstrap output directory: %w", err)
		}
		return "", errors.New("bootstrap output parent is not a directory")
	}

	templateBytes, err := os.ReadFile(template)
	if err != nil {
		return "", err
	}
	content := string(templateBytes)
	if strings.HasPrefix(content, "\ufeff") {
		return "", errors.New("bootstrap template must not contain a UTF-8 BOM")
	}
	for _, placeholder := range []string{"@VPSKIT_VERSION@", "@VPSKIT_REPOSITORY@", "@VPSKIT_ARCHIVE_SHA256@"} {
		if strings.Count(content, placeholder) != 1 {
			return "", fmt.Errorf("bootstrap template must contain exactly one %s placeholder", placeholder)
		}
	}
	hash, err := FileSHA256(archive)
	if err != nil {
		return "", err
	}
	content = strings.ReplaceAll(content, "@VPSKIT_VERSION@", version)
	content = strings.ReplaceAll(content, "@VPSKIT_REPOSITORY@", repository)
	content = strings.ReplaceAll(content, "@VPSKIT_ARCHIVE_SHA256@", hash)
	if strings.Contains(content, "@VPSKIT_") {
		return "", errors.New("bootstrap template contains an unresolved VPSKit placeholder")
	}
	content = strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n")

	temporary, err := os.CreateTemp(outputDirectory, ".vpskit-bootstrap-*.tmp")
	if err != nil {
		return "", err
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		if returnErr != nil {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o644); err != nil {
		return "", err
	}
	if _, err := temporary.WriteString(content); err != nil {
		return "", err
	}
	if err := temporary.Sync(); err != nil {
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(temporaryPath, output); err != nil {
		return "", err
	}
	return hash, nil
}

func regularAbsolutePath(path, description string) (string, error) {
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(resolved)
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", description, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s must be a regular non-symlink file", description)
	}
	return resolved, nil
}

func samePath(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}
