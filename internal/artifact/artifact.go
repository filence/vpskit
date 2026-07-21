// Package artifact defines validated, deterministic client publication units.
package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

const SchemaVersion = 1

type Capability struct {
	Name                       string   `json:"name"`
	RendererVersion            int      `json:"renderer_version"`
	CompatibilityProfile       string   `json:"compatibility_profile"`
	SupportedProtocols         []string `json:"supported_protocols"`
	MinimumTestedClientVersion string   `json:"minimum_tested_client_version,omitempty"`
	SupportsFullConfig         bool     `json:"supports_full_config"`
	SupportsNodeSubscription   bool     `json:"supports_node_subscription"`
	SupportsRemoteRules        bool     `json:"supports_remote_rules"`
	SupportsIPv6               bool     `json:"supports_ipv6"`
	SupportsPortHopping        bool     `json:"supports_port_hopping"`
	SupportsWARPGroup          bool     `json:"supports_warp_group"`
	ValidationCommand          string   `json:"validation_command,omitempty"`
}

type Artifact struct {
	Name                 string `json:"name"`
	Target               string `json:"target"`
	MediaType            string `json:"media_type"`
	SHA256               string `json:"sha256"`
	Sensitive            bool   `json:"sensitive"`
	Renderer             string `json:"renderer"`
	RendererVersion      int    `json:"renderer_version"`
	CompatibilityProfile string `json:"compatibility_profile"`
	Content              []byte `json:"-"`
}

type Set struct {
	SchemaVersion   int        `json:"schema_version"`
	PublicationID   string     `json:"publication_id,omitempty"`
	NodeID          string     `json:"node_id"`
	ClientRevision  int        `json:"client_revision"`
	RulesetRevision int        `json:"ruleset_revision"`
	Artifacts       []Artifact `json:"artifacts"`
}

func New(name, target, mediaType string, sensitive bool, capability Capability, content []byte) (Artifact, error) {
	name = strings.TrimSpace(name)
	target = strings.TrimSpace(target)
	mediaType = strings.TrimSpace(mediaType)
	if name == "" || filepath.Base(name) != name || name == "." {
		return Artifact{}, fmt.Errorf("artifact name must be a base filename: %q", name)
	}
	if target == "" || mediaType == "" {
		return Artifact{}, errors.New("artifact target and media type are required")
	}
	if strings.TrimSpace(capability.Name) == "" || capability.RendererVersion < 1 {
		return Artifact{}, errors.New("renderer capability name and positive version are required")
	}
	if len(content) == 0 {
		return Artifact{}, fmt.Errorf("artifact %s has empty content", name)
	}
	digest := sha256.Sum256(content)
	return Artifact{
		Name:                 name,
		Target:               target,
		MediaType:            mediaType,
		SHA256:               hex.EncodeToString(digest[:]),
		Sensitive:            sensitive,
		Renderer:             capability.Name,
		RendererVersion:      capability.RendererVersion,
		CompatibilityProfile: capability.CompatibilityProfile,
		Content:              append([]byte(nil), content...),
	}, nil
}

func NewSet(nodeID string, clientRevision, rulesetRevision int, artifacts []Artifact) (Set, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return Set{}, errors.New("artifact set node_id is required")
	}
	if clientRevision < 1 || rulesetRevision < 0 {
		return Set{}, errors.New("artifact set revisions are invalid")
	}
	if len(artifacts) == 0 {
		return Set{}, errors.New("artifact set must contain at least one artifact")
	}
	seen := make(map[string]struct{}, len(artifacts))
	copyArtifacts := append([]Artifact(nil), artifacts...)
	for _, item := range copyArtifacts {
		if _, exists := seen[item.Name]; exists {
			return Set{}, fmt.Errorf("duplicate artifact name %q", item.Name)
		}
		seen[item.Name] = struct{}{}
	}
	sort.SliceStable(copyArtifacts, func(i, j int) bool { return copyArtifacts[i].Name < copyArtifacts[j].Name })
	return Set{
		SchemaVersion:   SchemaVersion,
		NodeID:          nodeID,
		ClientRevision:  clientRevision,
		RulesetRevision: rulesetRevision,
		Artifacts:       copyArtifacts,
	}, nil
}
