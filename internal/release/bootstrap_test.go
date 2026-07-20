package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateBootstrapReplacesPinnedReleaseValues(t *testing.T) {
	directory := t.TempDir()
	template := filepath.Join(directory, "install.sh.in")
	archive := filepath.Join(directory, "v1.2.3-linux-amd64.tar.gz")
	output := filepath.Join(directory, "install.sh")
	templateContent := "version=@VPSKIT_VERSION@\nrepo=@VPSKIT_REPOSITORY@\nhash=@VPSKIT_ARCHIVE_SHA256@\r\n"
	if err := os.WriteFile(template, []byte(templateContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, []byte("archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash, err := GenerateBootstrap(template, archive, output, "v1.2.3", "owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, expected := range []string{"version=v1.2.3", "repo=owner/repo", "hash=" + hash} {
		if !strings.Contains(text, expected) {
			t.Fatalf("generated bootstrap is missing %q: %s", expected, text)
		}
	}
	if strings.Contains(text, "@VPSKIT_") || strings.Contains(text, "\r") {
		t.Fatalf("generated bootstrap contains unresolved or non-LF content: %q", text)
	}
}

func TestGenerateBootstrapRefusesOverwrite(t *testing.T) {
	directory := t.TempDir()
	template := filepath.Join(directory, "install.sh.in")
	archive := filepath.Join(directory, "archive.tar.gz")
	output := filepath.Join(directory, "install.sh")
	content := "@VPSKIT_VERSION@ @VPSKIT_REPOSITORY@ @VPSKIT_ARCHIVE_SHA256@"
	if err := os.WriteFile(template, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, []byte("archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := GenerateBootstrap(template, archive, output, "v1.2.3", "owner/repo"); err == nil {
		t.Fatal("expected overwrite refusal")
	}
	kept, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != "keep" {
		t.Fatalf("existing output changed: %q", kept)
	}
}

func TestGenerateBootstrapRequiresExactPlaceholders(t *testing.T) {
	directory := t.TempDir()
	template := filepath.Join(directory, "install.sh.in")
	archive := filepath.Join(directory, "archive.tar.gz")
	if err := os.WriteFile(template, []byte("@VPSKIT_VERSION@"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, []byte("archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := GenerateBootstrap(template, archive, filepath.Join(directory, "install.sh"), "v1.2.3", "owner/repo"); err == nil {
		t.Fatal("expected placeholder validation failure")
	}
}
