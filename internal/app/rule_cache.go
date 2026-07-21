package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/artifact"
	"vpskit.local/vpskit/internal/fsutil"
	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/render"
)

const (
	rulesCacheSchemaVersion = 1
	rulesCacheMaxSourceSize = 1 << 20
)

var rulesCacheHTTPClient = &http.Client{Timeout: 45 * time.Second}

type rulesCacheSource struct {
	Name      string `json:"name"`
	Target    string `json:"target"`
	URL       string `json:"url"`
	MediaType string `json:"media_type"`
	SHA256    string `json:"sha256"`
	Bytes     int    `json:"bytes"`
	Path      string `json:"path"`
}

type rulesCacheManifest struct {
	SchemaVersion int                `json:"schema_version"`
	Profile       string             `json:"profile"`
	Revision      int                `json:"revision"`
	CreatedAt     time.Time          `json:"created_at"`
	Sources       []rulesCacheSource `json:"sources"`
}

func rulesCacheRevisionRoot(revision int) string {
	return filepath.Join(rulesCacheRoot, fmt.Sprintf("r%04d", revision))
}

func rulesCacheManifestPath(revision int) string {
	return filepath.Join(rulesCacheRevisionRoot(revision), "manifest.json")
}

func managedRuleProviderBaseURL() (string, error) {
	configuration, secrets, err := readSubscriptionDocuments()
	if err != nil {
		return "", fmt.Errorf("managed rules require a configured subscription: %w", err)
	}
	return strings.TrimRight(configuration.Endpoint, "/") + "/s/" + secrets.ReadToken + "/rules", nil
}

func refreshManagedRuleCache(profile string, revision int) (rulesCacheManifest, error) {
	if revision < 1 {
		return rulesCacheManifest{}, errors.New("managed rules revision must be positive")
	}
	sources, err := render.MihomoRuleSources(profile)
	if err != nil {
		return rulesCacheManifest{}, err
	}
	if len(sources) == 0 {
		return rulesCacheManifest{}, fmt.Errorf("rules profile %q has no remote providers to cache", profile)
	}
	root := rulesCacheRevisionRoot(revision)
	if err := os.MkdirAll(root, 0o750); err != nil {
		return rulesCacheManifest{}, err
	}
	manifest := rulesCacheManifest{SchemaVersion: rulesCacheSchemaVersion, Profile: profile, Revision: revision, CreatedAt: time.Now().UTC(), Sources: make([]rulesCacheSource, 0, len(sources))}
	for _, source := range sources {
		content, err := downloadRuleSource(source)
		if err != nil {
			return rulesCacheManifest{}, err
		}
		filename := source.Target + ".rules"
		path := filepath.Join(root, filename)
		if err := fsutil.WriteFileAtomic(path, content, 0o640); err != nil {
			return rulesCacheManifest{}, err
		}
		digest := sha256.Sum256(content)
		manifest.Sources = append(manifest.Sources, rulesCacheSource{Name: source.Name, Target: source.Target, URL: source.URL, MediaType: source.MediaType, SHA256: hex.EncodeToString(digest[:]), Bytes: len(content), Path: filename})
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return rulesCacheManifest{}, err
	}
	if err := fsutil.WriteFileAtomic(rulesCacheManifestPath(revision), append(manifestBytes, '\n'), 0o640); err != nil {
		return rulesCacheManifest{}, err
	}
	return manifest, nil
}

func downloadRuleSource(source render.MihomoRuleSource) ([]byte, error) {
	requestContext, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, source.URL, nil)
	if err != nil {
		return nil, err
	}
	response, err := rulesCacheHTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download rule source %s: %w", source.Name, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download rule source %s returned HTTP %d", source.Name, response.StatusCode)
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, rulesCacheMaxSourceSize+1))
	if err != nil {
		return nil, fmt.Errorf("read rule source %s: %w", source.Name, err)
	}
	if len(content) == 0 || len(content) > rulesCacheMaxSourceSize {
		return nil, fmt.Errorf("rule source %s has invalid size %d bytes", source.Name, len(content))
	}
	return content, nil
}

func appendManagedRuleCacheArtifacts(state model.State, set artifact.Set) (artifact.Set, error) {
	if state.Rules.SourceMode != model.RulesSourceManaged {
		return set, nil
	}
	manifest, err := loadManagedRuleCache(state.Rules)
	if err != nil {
		return artifact.Set{}, err
	}
	items := append([]artifact.Artifact(nil), set.Artifacts...)
	capability := artifact.Capability{Name: "managed-rule-cache", RendererVersion: 1, CompatibilityProfile: "mihomo-managed-rules-v1", SupportsRemoteRules: true}
	for _, source := range manifest.Sources {
		content, err := os.ReadFile(filepath.Join(rulesCacheRevisionRoot(manifest.Revision), source.Path))
		if err != nil {
			return artifact.Set{}, fmt.Errorf("read cached rule %s: %w", source.Target, err)
		}
		digest := sha256.Sum256(content)
		if len(content) != source.Bytes || hex.EncodeToString(digest[:]) != source.SHA256 {
			return artifact.Set{}, fmt.Errorf("cached rule %s digest does not match its manifest", source.Target)
		}
		item, err := artifact.New("rule-"+source.Target+".rules", "rule/"+source.Target, source.MediaType, false, capability, content)
		if err != nil {
			return artifact.Set{}, err
		}
		items = append(items, item)
	}
	updated, err := artifact.NewSet(set.NodeID, set.ClientRevision, set.RulesetRevision, items)
	if err != nil {
		return artifact.Set{}, err
	}
	updated.PublicationID = set.PublicationID
	return updated, nil
}

func loadManagedRuleCache(rules model.RulesState) (rulesCacheManifest, error) {
	if rules.SourceMode != model.RulesSourceManaged || rules.Revision < 1 {
		return rulesCacheManifest{}, errors.New("managed rule cache is not active")
	}
	bytes, err := os.ReadFile(rulesCacheManifestPath(rules.Revision))
	if err != nil {
		return rulesCacheManifest{}, fmt.Errorf("read managed rule cache manifest: %w", err)
	}
	var manifest rulesCacheManifest
	decoder := json.NewDecoder(strings.NewReader(string(bytes)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return rulesCacheManifest{}, fmt.Errorf("parse managed rule cache manifest: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return rulesCacheManifest{}, fmt.Errorf("parse managed rule cache manifest: %w", err)
	}
	if manifest.SchemaVersion != rulesCacheSchemaVersion || manifest.Profile != rules.Profile || manifest.Revision != rules.Revision || len(manifest.Sources) == 0 {
		return rulesCacheManifest{}, errors.New("managed rule cache manifest does not match the active rules state")
	}
	expected, err := render.MihomoRuleSources(rules.Profile)
	if err != nil {
		return rulesCacheManifest{}, err
	}
	if len(expected) != len(manifest.Sources) {
		return rulesCacheManifest{}, errors.New("managed rule cache source count does not match the active profile")
	}
	for index, source := range expected {
		cached := manifest.Sources[index]
		if cached.Name != source.Name || cached.Target != source.Target || cached.URL != source.URL || cached.MediaType != source.MediaType || cached.Path != source.Target+".rules" || cached.SHA256 == "" || cached.Bytes < 1 || cached.Bytes > rulesCacheMaxSourceSize {
			return rulesCacheManifest{}, fmt.Errorf("managed rule cache source %s is invalid", source.Target)
		}
	}
	return manifest, nil
}
