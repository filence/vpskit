package fsutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func WriteFileAtomic(path string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".vpskit-staging-*")
	if err != nil {
		return fmt.Errorf("create staging file: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return fmt.Errorf("set staging mode: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write staging file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync staging file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close staging file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("activate staging file: %w", err)
	}
	committed = true
	if err := syncDirectory(directory); err != nil {
		return err
	}
	return nil
}

func CopyFileAtomic(source, destination string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer input.Close()
	directory := filepath.Dir(destination)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".vpskit-copy-*")
	if err != nil {
		return fmt.Errorf("create destination staging file: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return fmt.Errorf("set destination staging mode: %w", err)
	}
	if _, err := io.Copy(temporary, input); err != nil {
		return fmt.Errorf("copy file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync destination staging file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close destination staging file: %w", err)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("activate copied file: %w", err)
	}
	committed = true
	return syncDirectory(directory)
}

func RemoveManagedTree(path string, allowedRoots ...string) error {
	cleanPath := filepath.Clean(path)
	allowed := false
	for _, root := range allowedRoots {
		cleanRoot := filepath.Clean(root)
		if cleanPath == cleanRoot || strings.HasPrefix(cleanPath, cleanRoot+string(os.PathSeparator)) {
			allowed = true
			break
		}
	}
	if !allowed || cleanPath == string(os.PathSeparator) || cleanPath == "." {
		return fmt.Errorf("refusing to remove unmanaged path: %s", path)
	}
	if err := os.RemoveAll(cleanPath); err != nil {
		return fmt.Errorf("remove managed path %s: %w", cleanPath, err)
	}
	return nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open directory for sync: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		if runtime.GOOS == "windows" {
			return nil
		}
		return fmt.Errorf("sync directory: %w", err)
	}
	return nil
}
