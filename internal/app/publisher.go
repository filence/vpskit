package app

import (
	"fmt"

	"vpskit.local/vpskit/internal/artifact"
	"vpskit.local/vpskit/internal/publisher"
)

func publishStaticClientArtifacts(set artifact.Set) error {
	set.PublicationID = fmt.Sprintf("local-r%04d", set.ClientRevision)
	staticPublisher := publisher.Static{Root: exportRoot, Mode: 0o600}
	if _, err := staticPublisher.Plan(set); err != nil {
		return fmt.Errorf("plan static client publication: %w", err)
	}
	if err := staticPublisher.Publish(set); err != nil {
		return fmt.Errorf("publish static client artifacts: %w", err)
	}
	return nil
}
