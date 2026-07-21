package artifact

import "testing"

func TestNewSetCalculatesDigestAndSortsArtifacts(t *testing.T) {
	capability := Capability{Name: "test", RendererVersion: 1, CompatibilityProfile: "test/v1"}
	second, err := New("z.txt", "test", "text/plain", true, capability, []byte("z\n"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := New("a.txt", "test", "text/plain", false, capability, []byte("a\n"))
	if err != nil {
		t.Fatal(err)
	}
	set, err := NewSet("node-main", 7, 0, []Artifact{second, first})
	if err != nil {
		t.Fatal(err)
	}
	if set.Artifacts[0].Name != "a.txt" || set.Artifacts[0].SHA256 == "" {
		t.Fatalf("unexpected artifact set: %#v", set)
	}
}

func TestNewSetRejectsDuplicateNames(t *testing.T) {
	capability := Capability{Name: "test", RendererVersion: 1}
	item, err := New("same.txt", "test", "text/plain", true, capability, []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewSet("node-main", 1, 0, []Artifact{item, item}); err == nil {
		t.Fatal("duplicate artifact names must be rejected")
	}
}
