package publisher

import (
	"path/filepath"
	"testing"

	"vpskit.local/vpskit/internal/artifact"
)

func TestStaticPublisherPlanPublishAndReadback(t *testing.T) {
	capability := artifact.Capability{Name: "test", RendererVersion: 1, CompatibilityProfile: "test/v1"}
	item, err := artifact.New("client.txt", "test", "text/plain", true, capability, []byte("secret\n"))
	if err != nil {
		t.Fatal(err)
	}
	set, err := artifact.NewSet("node-main", 1, 0, []artifact.Artifact{item})
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "exports")
	publisher := Static{Root: root}
	plan, err := publisher.Plan(set)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Changes[0].Action != "create" {
		t.Fatalf("unexpected initial plan: %#v", plan)
	}
	if err := publisher.Publish(set); err != nil {
		t.Fatal(err)
	}
	plan, err = publisher.Plan(set)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Changes[0].Action != "unchanged" {
		t.Fatalf("unexpected readback plan: %#v", plan)
	}
}
