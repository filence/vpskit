package release

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateLinuxArchiveUsesExplicitExecutableModes(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "v1.2.3")
	if err := os.MkdirAll(filepath.Join(source, "licenses"), 0o755); err != nil {
		t.Fatal(err)
	}
	for relative, content := range map[string]string{
		"vpskit":             "binary",
		"sing-box":           "binary",
		"xray":               "binary",
		"lego":               "binary",
		"versions.lock":      "{}",
		"licenses/a.LICENSE": "license",
	} {
		if err := os.WriteFile(filepath.Join(source, filepath.FromSlash(relative)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	archivePath := filepath.Join(parent, "release.tar.gz")
	if err := CreateLinuxArchive(source, archivePath); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	modes := make(map[string]int64)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		modes[header.Name] = header.Mode
	}
	for _, name := range []string{"vpskit", "sing-box", "xray", "lego"} {
		if got := modes["v1.2.3/"+name]; got != 0o755 {
			t.Fatalf("mode for %s = %#o, want 0755", name, got)
		}
	}
	if got := modes["v1.2.3/versions.lock"]; got != 0o644 {
		t.Fatalf("mode for versions.lock = %#o, want 0644", got)
	}
}

func TestCreateLinuxArchiveRefusesOverwrite(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "v1")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(parent, "release.tar.gz")
	if err := os.WriteFile(output, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CreateLinuxArchive(source, output); err == nil {
		t.Fatal("expected overwrite refusal")
	}
	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "keep" {
		t.Fatalf("existing output changed: %q", content)
	}
}

func TestCreateLinuxArchiveRefusesOutputInsideSource(t *testing.T) {
	source := t.TempDir()
	if err := CreateLinuxArchive(source, filepath.Join(source, "release.tar.gz")); err == nil {
		t.Fatal("expected inside-source output refusal")
	}
}

func TestCreateLinuxArchiveRefusesSymlink(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "v1")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(parent, "target")
	if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(source, "link")); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}
	if err := CreateLinuxArchive(source, filepath.Join(parent, "release.tar.gz")); err == nil {
		t.Fatal("expected symlink refusal")
	}
}
