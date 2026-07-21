package publisher

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/artifact"
)

const (
	workersPayloadSchema = 1
	workersMaxBodyBytes  = 1 << 20
)

type WorkersConfig struct {
	Endpoint      string
	NodeID        string
	PublishSecret string
	ReadToken     string
	HTTPClient    *http.Client
	Now           func() time.Time
	ReadbackWait  time.Duration
	ReadbackPoll  time.Duration
}

type Workers struct {
	config WorkersConfig
}

type RemoteArtifact struct {
	Target               string `json:"target"`
	MediaType            string `json:"media_type"`
	SHA256               string `json:"sha256"`
	ContentBase64        string `json:"content_base64"`
	Renderer             string `json:"renderer"`
	RendererVersion      int    `json:"renderer_version"`
	CompatibilityProfile string `json:"compatibility_profile"`
}

type PublicationPayload struct {
	SchemaVersion   int              `json:"schema_version"`
	NodeID          string           `json:"node_id"`
	NodeRevision    int              `json:"node_revision"`
	RulesetRevision int              `json:"ruleset_revision"`
	PublicationID   string           `json:"publication_id"`
	Artifacts       []RemoteArtifact `json:"artifacts"`
}

type PublicationResult struct {
	Status        string    `json:"status"`
	PublicationID string    `json:"publication_id"`
	NodeRevision  int       `json:"node_revision"`
	PublishedAt   time.Time `json:"published_at"`
	Targets       []string  `json:"targets"`
}

type RemoteManifest struct {
	SchemaVersion   int    `json:"schema_version"`
	NodeID          string `json:"node_id"`
	NodeRevision    int    `json:"node_revision"`
	RulesetRevision int    `json:"ruleset_revision"`
	PublicationID   string `json:"publication_id"`
	Artifacts       []struct {
		Target               string `json:"target"`
		MediaType            string `json:"media_type"`
		SHA256               string `json:"sha256"`
		Renderer             string `json:"renderer"`
		RendererVersion      int    `json:"renderer_version"`
		CompatibilityProfile string `json:"compatibility_profile"`
	} `json:"artifacts"`
}

type TokenRotationRequest struct {
	NewTokenHash string    `json:"new_token_hash"`
	OverlapUntil time.Time `json:"overlap_until"`
}

type TokenRotationResult struct {
	Status       string    `json:"status"`
	OverlapUntil time.Time `json:"overlap_until"`
}

type RemotePublicationStatus struct {
	SchemaVersion   int       `json:"schema_version"`
	NodeID          string    `json:"node_id"`
	NodeRevision    int       `json:"node_revision"`
	RulesetRevision int       `json:"ruleset_revision"`
	PublicationID   string    `json:"publication_id"`
	PublishedAt     time.Time `json:"published_at"`
	Targets         []string  `json:"targets"`
}

func NewWorkers(config WorkersConfig) (*Workers, error) {
	config.Endpoint = strings.TrimRight(strings.TrimSpace(config.Endpoint), "/")
	config.NodeID = strings.TrimSpace(config.NodeID)
	config.PublishSecret = strings.TrimSpace(config.PublishSecret)
	config.ReadToken = strings.TrimSpace(config.ReadToken)
	endpoint, err := url.Parse(config.Endpoint)
	localTestEndpoint := endpoint != nil && endpoint.Scheme == "http" && (endpoint.Hostname() == "127.0.0.1" || endpoint.Hostname() == "localhost")
	if err != nil || (endpoint.Scheme != "https" && !localTestEndpoint) || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, errors.New("Workers endpoint must be an HTTPS origin without credentials, query, or fragment")
	}
	if config.NodeID == "" || strings.ContainsAny(config.NodeID, "/?#") {
		return nil, errors.New("Workers publisher node ID is invalid")
	}
	if !validOpaqueSecret(config.PublishSecret) {
		return nil, errors.New("Workers publisher secret must contain at least 256 bits of entropy")
	}
	if !validOpaqueSecret(config.ReadToken) {
		return nil, errors.New("Workers read token must contain at least 256 bits of entropy")
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 20 * time.Second}
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.ReadbackWait == 0 {
		config.ReadbackWait = 75 * time.Second
	}
	if config.ReadbackPoll == 0 {
		config.ReadbackPoll = 2 * time.Second
	}
	if config.ReadbackWait < 0 || config.ReadbackWait > 5*time.Minute || config.ReadbackPoll <= 0 || config.ReadbackPoll > 30*time.Second {
		return nil, errors.New("Workers readback timing configuration is invalid")
	}
	return &Workers{config: config}, nil
}

func (publisher *Workers) Preflight(set artifact.Set) error {
	if publisher == nil {
		return errors.New("Workers publisher is nil")
	}
	if set.SchemaVersion != artifact.SchemaVersion || set.NodeID != publisher.config.NodeID || set.ClientRevision < 1 {
		return errors.New("artifact set does not match Workers publisher identity")
	}
	_, err := publicationPayload(set)
	return err
}

func (publisher *Workers) Plan(set artifact.Set) (Plan, error) {
	if err := publisher.Preflight(set); err != nil {
		return Plan{}, err
	}
	changes := []Change{}
	for _, item := range set.Artifacts {
		target, publish := remoteTarget(item)
		if publish {
			changes = append(changes, Change{Name: target, Action: "publish", SHA256: item.SHA256})
		}
	}
	manifest, err := publicationManifest(set)
	if err != nil {
		return Plan{}, err
	}
	digest := sha256.Sum256(manifest)
	changes = append(changes, Change{Name: "manifest", Action: "publish", SHA256: hex.EncodeToString(digest[:])})
	return Plan{Publisher: "workers", PublicationID: publicationID(set), Changes: changes}, nil
}

func (publisher *Workers) Publish(set artifact.Set) error {
	_, err := publisher.PublishWithResult(context.Background(), set)
	return err
}

func (publisher *Workers) PublishWithResult(ctx context.Context, set artifact.Set) (PublicationResult, error) {
	if err := publisher.Preflight(set); err != nil {
		return PublicationResult{}, err
	}
	payload, err := publicationPayload(set)
	if err != nil {
		return PublicationResult{}, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return PublicationResult{}, err
	}
	path := "/api/v1/nodes/" + url.PathEscape(publisher.config.NodeID) + "/publish"
	response, err := publisher.signedRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return PublicationResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return PublicationResult{}, sanitizedHTTPError("publish client artifacts", response)
	}
	var result PublicationResult
	if err := decodeLimitedJSON(response.Body, &result); err != nil {
		return PublicationResult{}, fmt.Errorf("decode Workers publication response: %w", err)
	}
	if result.Status != "COMMITTED" || result.PublicationID != payload.PublicationID || result.NodeRevision != payload.NodeRevision {
		return PublicationResult{}, errors.New("Workers publication response does not match the submitted revision")
	}
	if err := publisher.ReadbackContext(ctx, set); err != nil {
		return result, fmt.Errorf("Workers publication committed but readback is degraded: %w", err)
	}
	return result, nil
}

func (publisher *Workers) Readback(set artifact.Set) error {
	return publisher.ReadbackContext(context.Background(), set)
}

func (publisher *Workers) ReadbackContext(ctx context.Context, set artifact.Set) error {
	deadline := time.Now().Add(publisher.config.ReadbackWait)
	var lastErr error
	for {
		lastErr = publisher.readbackOnce(ctx, set)
		if lastErr == nil {
			return nil
		}
		if publisher.config.ReadbackWait == 0 || !time.Now().Before(deadline) {
			return lastErr
		}
		timer := time.NewTimer(publisher.config.ReadbackPoll)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("Workers readback canceled: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func (publisher *Workers) readbackOnce(ctx context.Context, set artifact.Set) error {
	if strings.TrimSpace(publisher.config.ReadToken) == "" {
		return errors.New("Workers read token is not configured")
	}
	expected := map[string]string{}
	for _, item := range set.Artifacts {
		if target, publish := remoteTarget(item); publish {
			expected[target] = item.SHA256
		}
	}
	manifest, err := publicationManifest(set)
	if err != nil {
		return err
	}
	manifestDigest := sha256.Sum256(manifest)
	expected["manifest"] = hex.EncodeToString(manifestDigest[:])
	for target, digest := range expected {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, publisher.subscriptionURL(target), nil)
		if err != nil {
			return err
		}
		response, err := publisher.config.HTTPClient.Do(request)
		if err != nil {
			return fmt.Errorf("read back Workers target %s: %w", target, err)
		}
		content, readErr := io.ReadAll(io.LimitReader(response.Body, workersMaxBodyBytes+1))
		response.Body.Close()
		if readErr != nil {
			return readErr
		}
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("read back Workers target %s returned HTTP %d", target, response.StatusCode)
		}
		if len(content) > workersMaxBodyBytes {
			return fmt.Errorf("Workers target %s exceeds the response size limit", target)
		}
		actual := sha256.Sum256(content)
		if hex.EncodeToString(actual[:]) != digest {
			return fmt.Errorf("Workers target %s digest does not match the submitted revision", target)
		}
	}
	return nil
}

func (publisher *Workers) Activate(publicationID string) error {
	return publisher.revisionAction(context.Background(), "activate", map[string]string{"publication_id": strings.TrimSpace(publicationID)})
}

func (publisher *Workers) Rollback(publicationID string) error {
	return publisher.revisionAction(context.Background(), "rollback", map[string]string{"publication_id": strings.TrimSpace(publicationID)})
}

func (publisher *Workers) RollbackWithReadback(ctx context.Context, publicationID string) (RemotePublicationStatus, error) {
	publicationID = strings.TrimSpace(publicationID)
	if err := publisher.revisionAction(ctx, "rollback", map[string]string{"publication_id": publicationID}); err != nil {
		return RemotePublicationStatus{}, err
	}
	status, err := publisher.CurrentPublication(ctx)
	if err != nil {
		return RemotePublicationStatus{}, fmt.Errorf("read back rolled-back publication status: %w", err)
	}
	if status.PublicationID != publicationID {
		return RemotePublicationStatus{}, errors.New("rolled-back publication status does not match the requested revision")
	}
	manifest, err := publisher.CurrentManifest(ctx)
	if err != nil {
		return RemotePublicationStatus{}, fmt.Errorf("read back rolled-back publication manifest: %w", err)
	}
	if manifest.PublicationID != publicationID || manifest.NodeRevision != status.NodeRevision || manifest.RulesetRevision != status.RulesetRevision {
		return RemotePublicationStatus{}, errors.New("rolled-back publication manifest does not match remote status")
	}
	return status, nil
}

func (publisher *Workers) CurrentPublication(ctx context.Context) (RemotePublicationStatus, error) {
	path := "/api/v1/nodes/" + url.PathEscape(publisher.config.NodeID) + "/status"
	response, err := publisher.signedRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return RemotePublicationStatus{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return RemotePublicationStatus{}, sanitizedHTTPError("read Workers publication status", response)
	}
	var status RemotePublicationStatus
	if err := decodeLimitedJSON(response.Body, &status); err != nil {
		return RemotePublicationStatus{}, err
	}
	if status.SchemaVersion != workersPayloadSchema || status.NodeID != publisher.config.NodeID || status.NodeRevision < 1 || status.PublicationID == "" {
		return RemotePublicationStatus{}, errors.New("Workers publication status is invalid")
	}
	return status, nil
}

func (publisher *Workers) CurrentManifest(ctx context.Context) (RemoteManifest, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, publisher.subscriptionURL("manifest"), nil)
	if err != nil {
		return RemoteManifest{}, err
	}
	response, err := publisher.config.HTTPClient.Do(request)
	if err != nil {
		return RemoteManifest{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return RemoteManifest{}, fmt.Errorf("read Workers publication manifest returned HTTP %d", response.StatusCode)
	}
	var manifest RemoteManifest
	if err := decodeLimitedJSON(response.Body, &manifest); err != nil {
		return RemoteManifest{}, err
	}
	if manifest.SchemaVersion != workersPayloadSchema || manifest.NodeID != publisher.config.NodeID || manifest.NodeRevision < 1 || manifest.PublicationID == "" {
		return RemoteManifest{}, errors.New("Workers publication manifest is invalid")
	}
	return manifest, nil
}

func (publisher *Workers) RotateReadToken() error {
	return errors.New("use RotateReadTokenTo with an explicitly stored replacement token")
}

func (publisher *Workers) RotateReadTokenTo(ctx context.Context, newToken string, overlap time.Duration) (TokenRotationResult, error) {
	newToken = strings.TrimSpace(newToken)
	if len(newToken) < 43 {
		return TokenRotationResult{}, errors.New("replacement read token must contain at least 256 bits of entropy")
	}
	if overlap < 0 || overlap > 7*24*time.Hour {
		return TokenRotationResult{}, errors.New("read token overlap must be between zero and seven days")
	}
	digest := sha256.Sum256([]byte(newToken))
	payload := TokenRotationRequest{NewTokenHash: hex.EncodeToString(digest[:]), OverlapUntil: publisher.config.Now().UTC().Add(overlap)}
	body, err := json.Marshal(payload)
	if err != nil {
		return TokenRotationResult{}, err
	}
	path := "/api/v1/nodes/" + url.PathEscape(publisher.config.NodeID) + "/read-token/rotate"
	response, err := publisher.signedRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return TokenRotationResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return TokenRotationResult{}, sanitizedHTTPError("rotate subscription read token", response)
	}
	var result TokenRotationResult
	if err := decodeLimitedJSON(response.Body, &result); err != nil {
		return TokenRotationResult{}, err
	}
	if result.Status != "ROTATED" {
		return TokenRotationResult{}, errors.New("Workers read token rotation was not committed")
	}
	return result, nil
}

func (publisher *Workers) RevokeReadToken() error {
	return errors.New("use RevokeReadTokenValue with an explicitly identified token")
}

func (publisher *Workers) RevokeReadTokenValue(ctx context.Context, token string) error {
	digest := sha256.Sum256([]byte(strings.TrimSpace(token)))
	body, err := json.Marshal(map[string]string{"token_hash": hex.EncodeToString(digest[:])})
	if err != nil {
		return err
	}
	path := "/api/v1/nodes/" + url.PathEscape(publisher.config.NodeID) + "/read-token/revoke"
	response, err := publisher.signedRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return sanitizedHTTPError("revoke subscription read token", response)
	}
	return nil
}

func (publisher *Workers) Healthcheck() error {
	request, err := http.NewRequest(http.MethodGet, publisher.config.Endpoint+"/healthz", nil)
	if err != nil {
		return err
	}
	response, err := publisher.config.HTTPClient.Do(request)
	if err != nil {
		return fmt.Errorf("Workers healthcheck failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Workers healthcheck returned HTTP %d", response.StatusCode)
	}
	return nil
}

func (publisher *Workers) subscriptionURL(target string) string {
	return publisher.config.Endpoint + "/s/" + url.PathEscape(publisher.config.ReadToken) + "/" + url.PathEscape(target)
}

func (publisher *Workers) signedRequest(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	timestamp := strconv.FormatInt(publisher.config.Now().UTC().Unix(), 10)
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, err
	}
	nonce := hex.EncodeToString(nonceBytes)
	bodyDigest := sha256.Sum256(body)
	canonical := strings.Join([]string{timestamp, nonce, method, path, hex.EncodeToString(bodyDigest[:])}, "\n")
	mac := hmac.New(sha256.New, []byte(publisher.config.PublishSecret))
	_, _ = mac.Write([]byte(canonical))
	signature := hex.EncodeToString(mac.Sum(nil))
	request, err := http.NewRequestWithContext(ctx, method, publisher.config.Endpoint+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-VPSKit-Timestamp", timestamp)
	request.Header.Set("X-VPSKit-Nonce", nonce)
	request.Header.Set("X-VPSKit-Signature", signature)
	return publisher.config.HTTPClient.Do(request)
}

func (publisher *Workers) revisionAction(ctx context.Context, action string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	path := "/api/v1/nodes/" + url.PathEscape(publisher.config.NodeID) + "/" + action
	response, err := publisher.signedRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return sanitizedHTTPError(action+" Workers publication", response)
	}
	return nil
}

func publicationPayload(set artifact.Set) (PublicationPayload, error) {
	remote := make([]RemoteArtifact, 0, 3)
	for _, item := range set.Artifacts {
		target, publish := remoteTarget(item)
		if !publish {
			continue
		}
		remote = append(remote, RemoteArtifact{
			Target: target, MediaType: item.MediaType, SHA256: item.SHA256,
			ContentBase64: base64.StdEncoding.EncodeToString(item.Content), Renderer: item.Renderer,
			RendererVersion: item.RendererVersion, CompatibilityProfile: item.CompatibilityProfile,
		})
	}
	if len(remote) != 2 {
		return PublicationPayload{}, errors.New("Workers publication requires exactly Mihomo and v2rayN artifacts")
	}
	manifest, err := publicationManifest(set)
	if err != nil {
		return PublicationPayload{}, err
	}
	digest := sha256.Sum256(manifest)
	remote = append(remote, RemoteArtifact{
		Target: "manifest", MediaType: "application/json", SHA256: hex.EncodeToString(digest[:]),
		ContentBase64: base64.StdEncoding.EncodeToString(manifest), Renderer: "manifest", RendererVersion: 1,
		CompatibilityProfile: "vpskit-publication-manifest-v1",
	})
	payload := PublicationPayload{
		SchemaVersion: workersPayloadSchema, NodeID: set.NodeID, NodeRevision: set.ClientRevision,
		RulesetRevision: set.RulesetRevision, PublicationID: publicationID(set), Artifacts: remote,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return PublicationPayload{}, err
	}
	if len(body) > workersMaxBodyBytes {
		return PublicationPayload{}, errors.New("Workers publication exceeds the one MiB request limit")
	}
	return payload, nil
}

func publicationManifest(set artifact.Set) ([]byte, error) {
	manifest := RemoteManifest{
		SchemaVersion: workersPayloadSchema, NodeID: set.NodeID, NodeRevision: set.ClientRevision,
		RulesetRevision: set.RulesetRevision, PublicationID: publicationID(set),
	}
	for _, item := range set.Artifacts {
		target, publish := remoteTarget(item)
		if !publish {
			continue
		}
		manifest.Artifacts = append(manifest.Artifacts, struct {
			Target               string `json:"target"`
			MediaType            string `json:"media_type"`
			SHA256               string `json:"sha256"`
			Renderer             string `json:"renderer"`
			RendererVersion      int    `json:"renderer_version"`
			CompatibilityProfile string `json:"compatibility_profile"`
		}{target, item.MediaType, item.SHA256, item.Renderer, item.RendererVersion, item.CompatibilityProfile})
	}
	return json.MarshalIndent(manifest, "", "  ")
}

func publicationID(set artifact.Set) string {
	if strings.TrimSpace(set.PublicationID) != "" {
		return strings.TrimSpace(set.PublicationID)
	}
	return fmt.Sprintf("%s-r%04d", set.NodeID, set.ClientRevision)
}

func validOpaqueSecret(value string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	return err == nil && len(decoded) >= 32
}

func remoteTarget(item artifact.Artifact) (string, bool) {
	switch item.Target {
	case "mihomo":
		return "mihomo", true
	case "share-links", "v2rayn":
		return "v2rayn", true
	default:
		return "", false
	}
}

func sanitizedHTTPError(operation string, response *http.Response) error {
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	return fmt.Errorf("%s returned HTTP %d", operation, response.StatusCode)
}

func decodeLimitedJSON(reader io.Reader, destination any) error {
	decoder := json.NewDecoder(io.LimitReader(reader, 64<<10))
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}
