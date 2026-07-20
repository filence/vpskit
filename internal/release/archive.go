package release

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var linuxExecutableAssets = map[string]bool{
	"vpskit":   true,
	"sing-box": true,
	"xray":     true,
	"lego":     true,
}

// CreateLinuxArchive packages a verified release directory with explicit Linux
// ownership and modes. This avoids host-dependent executable bits when a
// release is assembled on Windows.
func CreateLinuxArchive(sourceDirectory, outputPath string) (returnErr error) {
	source, err := filepath.Abs(sourceDirectory)
	if err != nil {
		return err
	}
	output, err := filepath.Abs(outputPath)
	if err != nil {
		return err
	}
	info, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("inspect release directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("release source must be a non-symlink directory")
	}
	if pathWithin(output, source) {
		return errors.New("archive output must be outside the release source directory")
	}
	if _, err := os.Lstat(output); err == nil {
		return fmt.Errorf("refusing to overwrite archive: %s", output)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	outputDirectory := filepath.Dir(output)
	if parent, err := os.Stat(outputDirectory); err != nil || !parent.IsDir() {
		if err != nil {
			return fmt.Errorf("inspect archive output directory: %w", err)
		}
		return errors.New("archive output parent is not a directory")
	}

	temporary, err := os.CreateTemp(outputDirectory, ".vpskit-release-*.tar.gz")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		if returnErr != nil {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}

	gzipWriter := gzip.NewWriter(temporary)
	gzipWriter.Header.ModTime = time.Unix(0, 0).UTC()
	gzipWriter.Header.OS = 3
	tarWriter := tar.NewWriter(gzipWriter)
	rootName := filepath.Base(source)
	if rootName == "." || rootName == string(filepath.Separator) || rootName == "" {
		return errors.New("release source directory has an invalid base name")
	}
	if err := tarWriter.WriteHeader(linuxTarHeader(rootName+"/", 0, tar.TypeDir)); err != nil {
		return err
	}

	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == source {
			return nil
		}
		entryInfo, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if entryInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("release source contains a symbolic link: %s", path)
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		archiveName := rootName + "/" + filepath.ToSlash(relative)
		if entryInfo.IsDir() {
			return tarWriter.WriteHeader(linuxTarHeader(archiveName+"/", 0, tar.TypeDir))
		}
		if !entryInfo.Mode().IsRegular() {
			return fmt.Errorf("release source contains an unsupported file type: %s", path)
		}
		mode := int64(0o644)
		if linuxExecutableAssets[filepath.ToSlash(relative)] {
			mode = 0o755
		}
		header := linuxTarHeader(archiveName, entryInfo.Size(), tar.TypeReg)
		header.Mode = mode
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.CopyN(tarWriter, file, entryInfo.Size())
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		return err
	}
	if err := tarWriter.Close(); err != nil {
		return err
	}
	if err := gzipWriter.Close(); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, output); err != nil {
		return err
	}
	if err := os.Chmod(output, 0o644); err != nil {
		return err
	}
	return nil
}

func linuxTarHeader(name string, size int64, typeFlag byte) *tar.Header {
	return &tar.Header{
		Name:       name,
		Mode:       0o755,
		Uid:        0,
		Gid:        0,
		Size:       size,
		ModTime:    time.Unix(0, 0).UTC(),
		AccessTime: time.Time{},
		ChangeTime: time.Time{},
		Typeflag:   typeFlag,
		Uname:      "root",
		Gname:      "root",
	}
}

func pathWithin(candidate, root string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative))
}
