package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/artifact"
	"vpskit.local/vpskit/internal/fsutil"
	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/platform"
	"vpskit.local/vpskit/internal/publisher"
	"vpskit.local/vpskit/internal/render"
)

const subscriptionSchemaVersion = 1

type subscriptionConfig struct {
	SchemaVersion int      `json:"schema_version"`
	Backend       string   `json:"backend"`
	Endpoint      string   `json:"endpoint"`
	NodeID        string   `json:"node_id"`
	Targets       []string `json:"targets"`
	AutoPublish   bool     `json:"auto_publish"`
}

type subscriptionSecrets struct {
	SchemaVersion     int    `json:"schema_version"`
	NodePublishSecret string `json:"node_publish_secret"`
	ReadToken         string `json:"read_token"`
}

type subscriptionCredentialInput struct {
	SchemaVersion     int    `json:"schema_version"`
	Endpoint          string `json:"endpoint"`
	NodeID            string `json:"node_id"`
	NodePublishSecret string `json:"node_publish_secret"`
	ReadToken         string `json:"read_token"`
}

type publicationState struct {
	SchemaVersion   int       `json:"schema_version"`
	Status          string    `json:"status"`
	PublicationID   string    `json:"publication_id,omitempty"`
	ClientRevision  int       `json:"client_revision,omitempty"`
	RulesetRevision int       `json:"ruleset_revision,omitempty"`
	PublishedAt     time.Time `json:"published_at,omitempty"`
	CheckedAt       time.Time `json:"checked_at,omitempty"`
	LastError       string    `json:"last_error,omitempty"`
}

type subscriptionPostCommit struct {
	Status        string `json:"status"`
	PublicationID string `json:"publication_id,omitempty"`
	LastError     string `json:"last_error,omitempty"`
}

type subscriptionClientFile struct {
	SchemaVersion int               `json:"schema_version"`
	Endpoint      string            `json:"endpoint"`
	NodeID        string            `json:"node_id"`
	ReadToken     string            `json:"read_token"`
	URLs          map[string]string `json:"urls"`
}

type optionalFileSnapshot struct {
	path   string
	bytes  []byte
	mode   os.FileMode
	exists bool
}

func runSubscription(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: vpskit subscription <plan|configure|publish|status|rotate-read-token|revoke-read-token|rollback|remove> [options]")
	}
	switch arguments[0] {
	case "plan":
		return runSubscriptionPlan(arguments[1:])
	case "configure":
		return runSubscriptionConfigure(arguments[1:])
	case "publish":
		return runSubscriptionPublish(arguments[1:])
	case "status":
		return runSubscriptionStatus(arguments[1:])
	case "rotate-read-token":
		return runSubscriptionRotateReadToken(arguments[1:])
	case "revoke-read-token":
		return runSubscriptionRevokeReadToken(arguments[1:])
	case "rollback":
		return runSubscriptionRollback(arguments[1:])
	case "remove":
		return runSubscriptionRemove(arguments[1:])
	default:
		return fmt.Errorf("unsupported subscription operation %q", arguments[0])
	}
}

func runSubscriptionPlan(arguments []string) error {
	flags := flag.NewFlagSet("subscription plan", flag.ContinueOnError)
	credentialsFile := flags.String("credentials-file", "", "0600 JSON credentials created by the trusted Cloudflare deployment environment")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*credentialsFile) == "" {
		return errors.New("usage: vpskit subscription plan --credentials-file <path>")
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	input, err := readSubscriptionCredentialInput(*credentialsFile)
	if err != nil {
		return err
	}
	configuration, _, err := subscriptionDocuments(state.Node.ID, input)
	if err != nil {
		return err
	}
	return printJSON(commandResult{Command: "subscription plan", Status: "PASS", Detail: map[string]any{
		"mode":                "READ_ONLY",
		"backend":             configuration.Backend,
		"endpoint":            configuration.Endpoint,
		"node_id":             configuration.NodeID,
		"targets":             configuration.Targets,
		"writes":              []string{subscriptionConfigPath, subscriptionSecretPath, publicationStatePath, subscriptionOwnershipPath},
		"secret_values_shown": false,
	}})
}

func runSubscriptionConfigure(arguments []string) error {
	flags := flag.NewFlagSet("subscription configure", flag.ContinueOnError)
	credentialsFile := flags.String("credentials-file", "", "0600 JSON credentials created by the trusted Cloudflare deployment environment")
	autoPublish := flags.Bool("auto-publish", true, "publish after future client-facing changes")
	yes := flags.Bool("yes", false, "confirm subscription credential installation")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*credentialsFile) == "" {
		return errors.New("usage: vpskit subscription configure --credentials-file <path> [--auto-publish=true] --yes")
	}
	if !*yes {
		return errors.New("subscription configure requires explicit --yes confirmation")
	}
	if !platform.IsRoot() {
		return errors.New("subscription configure requires root privileges")
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	input, err := readSubscriptionCredentialInput(*credentialsFile)
	if err != nil {
		return err
	}
	configuration, secrets, err := subscriptionDocuments(state.Node.ID, input)
	if err != nil {
		return err
	}
	configuration.AutoPublish = *autoPublish
	if err := configureSubscriptionFiles(configuration, secrets); err != nil {
		return err
	}
	return printJSON(commandResult{Command: "subscription configure", Status: "PASS", Detail: map[string]any{
		"backend":               configuration.Backend,
		"endpoint":              configuration.Endpoint,
		"node_id":               configuration.NodeID,
		"targets":               configuration.Targets,
		"auto_publish":          configuration.AutoPublish,
		"credentials_installed": true,
		"publish_required":      true,
	}})
}

func runSubscriptionPublish(arguments []string) error {
	flags := flag.NewFlagSet("subscription publish", flag.ContinueOnError)
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: vpskit subscription publish")
	}
	if !platform.IsRoot() {
		return errors.New("subscription publish requires root privileges")
	}
	result, err := publishCurrentSubscription(context.Background())
	if err != nil {
		_ = recordPublicationFailure(err)
		return err
	}
	return printJSON(commandResult{Command: "subscription publish", Status: "PASS", Detail: map[string]any{
		"publication_id":  result.PublicationID,
		"client_revision": result.NodeRevision,
		"published_at":    result.PublishedAt,
		"targets":         result.Targets,
		"readback":        "PASS",
	}})
}

func runSubscriptionStatus(arguments []string) error {
	if len(arguments) != 0 {
		return errors.New("usage: vpskit subscription status")
	}
	if !platform.IsRoot() {
		return errors.New("subscription status requires root privileges")
	}
	configuration, secrets, err := readSubscriptionDocuments()
	if err != nil {
		return err
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	localPublication, _ := readPublicationState()
	clientSet, err := currentClientArtifactSet(state)
	if err != nil {
		return err
	}
	remote, err := newWorkersPublisher(configuration, secrets)
	if err != nil {
		return err
	}
	health := "PASS"
	readback := "NOT_PUBLISHED"
	if err := remote.Healthcheck(); err != nil {
		health = "FAIL"
	} else if localPublication.PublicationID != "" {
		if err := remote.Readback(clientSet); err != nil {
			readback = "DEGRADED"
		} else {
			readback = "PASS"
		}
	}
	status := "PASS"
	if health != "PASS" || readback == "DEGRADED" {
		status = "DEGRADED"
	}
	if localPublication.ClientRevision > 0 && localPublication.ClientRevision != state.ConfigRevision {
		readback = "STALE"
		status = "DEGRADED"
	}
	return printJSON(commandResult{Command: "subscription status", Status: status, Detail: map[string]any{
		"backend":             configuration.Backend,
		"endpoint":            configuration.Endpoint,
		"node_id":             configuration.NodeID,
		"auto_publish":        configuration.AutoPublish,
		"healthcheck":         health,
		"readback":            readback,
		"local_revision":      state.ConfigRevision,
		"published_revision":  localPublication.ClientRevision,
		"publication_id":      localPublication.PublicationID,
		"secret_values_shown": false,
	}})
}

func runSubscriptionRotateReadToken(arguments []string) error {
	flags := flag.NewFlagSet("subscription rotate-read-token", flag.ContinueOnError)
	overlapText := flags.String("overlap", "24h", "old-token overlap window, maximum 168h")
	output := flags.String("output", "", "new 0600 client subscription credential file")
	yes := flags.Bool("yes", false, "confirm remote read-token rotation")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*output) == "" {
		return errors.New("usage: vpskit subscription rotate-read-token --output <new-file> [--overlap 24h] --yes")
	}
	if !*yes {
		return errors.New("read-token rotation requires explicit --yes confirmation")
	}
	if !platform.IsRoot() {
		return errors.New("read-token rotation requires root privileges")
	}
	overlap, err := time.ParseDuration(strings.TrimSpace(*overlapText))
	if err != nil {
		return fmt.Errorf("parse read-token overlap: %w", err)
	}
	configuration, secrets, err := readSubscriptionDocuments()
	if err != nil {
		return err
	}
	newToken, err := randomSubscriptionToken()
	if err != nil {
		return err
	}
	outputPath, err := filepath.Abs(*output)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(outputPath); err == nil {
		return fmt.Errorf("refusing to overwrite existing token output: %s", outputPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := writeSubscriptionClientFile(outputPath, configuration, newToken); err != nil {
		return err
	}
	keepOutput := false
	defer func() {
		if !keepOutput {
			_ = os.Remove(outputPath)
		}
	}()
	remote, err := newWorkersPublisher(configuration, secrets)
	if err != nil {
		return err
	}
	result, err := remote.RotateReadTokenTo(context.Background(), newToken, overlap)
	if err != nil {
		return err
	}
	// Once the remote accepts the new token, this file is the recovery path if
	// persisting the local copy fails. It must not be deleted by the defer.
	keepOutput = true
	secrets.ReadToken = newToken
	if err := writeSubscriptionSecrets(secrets); err != nil {
		return fmt.Errorf("remote token rotated but local secret update failed; recover from %s: %w", outputPath, err)
	}
	return printJSON(commandResult{Command: "subscription rotate-read-token", Status: "PASS", Detail: map[string]any{
		"client_file":         outputPath,
		"overlap_until":       result.OverlapUntil,
		"old_token_revoked":   overlap == 0,
		"secret_values_shown": false,
	}})
}

func runSubscriptionRevokeReadToken(arguments []string) error {
	flags := flag.NewFlagSet("subscription revoke-read-token", flag.ContinueOnError)
	tokenFile := flags.String("token-file", "", "client credential JSON containing the token to revoke")
	yes := flags.Bool("yes", false, "confirm remote read-token revocation")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*tokenFile) == "" || !*yes {
		return errors.New("usage: vpskit subscription revoke-read-token --token-file <path> --yes")
	}
	if !platform.IsRoot() {
		return errors.New("read-token revocation requires root privileges")
	}
	configuration, secrets, err := readSubscriptionDocuments()
	if err != nil {
		return err
	}
	token, err := readTokenFromClientFile(*tokenFile)
	if err != nil {
		return err
	}
	remote, err := newWorkersPublisher(configuration, secrets)
	if err != nil {
		return err
	}
	if err := remote.RevokeReadTokenValue(context.Background(), token); err != nil {
		return err
	}
	return printJSON(commandResult{Command: "subscription revoke-read-token", Status: "PASS", Detail: map[string]any{"revoked": true, "secret_values_shown": false}})
}

func runSubscriptionRollback(arguments []string) error {
	flags := flag.NewFlagSet("subscription rollback", flag.ContinueOnError)
	publicationID := flags.String("publication-id", "", "previous immutable publication ID")
	yes := flags.Bool("yes", false, "confirm remote publication rollback")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*publicationID) == "" || !*yes {
		return errors.New("usage: vpskit subscription rollback --publication-id <id> --yes")
	}
	if !platform.IsRoot() {
		return errors.New("subscription rollback requires root privileges")
	}
	configuration, secrets, err := readSubscriptionDocuments()
	if err != nil {
		return err
	}
	remote, err := newWorkersPublisher(configuration, secrets)
	if err != nil {
		return err
	}
	remoteStatus, err := remote.RollbackWithReadback(context.Background(), *publicationID)
	if err != nil {
		_ = recordPublicationFailure(err)
		return err
	}
	if err := writePublicationState(publicationState{
		SchemaVersion: subscriptionSchemaVersion, Status: "ROLLED_BACK", PublicationID: remoteStatus.PublicationID,
		ClientRevision: remoteStatus.NodeRevision, RulesetRevision: remoteStatus.RulesetRevision,
		PublishedAt: remoteStatus.PublishedAt, CheckedAt: time.Now().UTC(),
	}); err != nil {
		return err
	}
	return printJSON(commandResult{Command: "subscription rollback", Status: "PASS", Detail: map[string]any{
		"publication_id": remoteStatus.PublicationID, "client_revision": remoteStatus.NodeRevision, "readback": "PASS",
	}})
}

func runSubscriptionRemove(arguments []string) error {
	flags := flag.NewFlagSet("subscription remove", flag.ContinueOnError)
	yes := flags.Bool("yes", false, "confirm local subscription configuration removal")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || !*yes {
		return errors.New("usage: vpskit subscription remove --yes")
	}
	if !platform.IsRoot() {
		return errors.New("subscription remove requires root privileges")
	}
	if err := removeSubscriptionFiles(); err != nil {
		return err
	}
	return printJSON(commandResult{Command: "subscription remove", Status: "PASS", Detail: map[string]any{"remote_publication_preserved": true, "remote_tokens_preserved": true}})
}

func removeSubscriptionFiles() error {
	lock, err := platform.AcquireProcessLock(subscriptionLockPath)
	if err != nil {
		return err
	}
	defer lock.Close()
	paths := []string{subscriptionSecretPath, publicationStatePath, subscriptionConfigPath, subscriptionOwnershipPath}
	snapshots := make([]optionalFileSnapshot, 0, len(paths))
	for _, path := range paths {
		snapshot, err := snapshotOptionalRegularFile(path)
		if err != nil {
			return err
		}
		snapshots = append(snapshots, snapshot)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		for index := len(snapshots) - 1; index >= 0; index-- {
			_ = restoreOptionalFile(snapshots[index])
		}
	}()
	if err := removeSubscriptionOwnership(); err != nil {
		return err
	}
	for _, path := range []string{subscriptionSecretPath, publicationStatePath, subscriptionConfigPath} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	committed = true
	return nil
}

func publishCurrentSubscription(ctx context.Context) (publisher.PublicationResult, error) {
	configuration, secrets, err := readSubscriptionDocuments()
	if err != nil {
		return publisher.PublicationResult{}, err
	}
	state, err := readInstalledState()
	if err != nil {
		return publisher.PublicationResult{}, err
	}
	clientSet, err := currentClientArtifactSet(state)
	if err != nil {
		return publisher.PublicationResult{}, err
	}
	clientSet.PublicationID = fmt.Sprintf("%s-r%04d", state.Node.ID, state.ConfigRevision)
	remote, err := newWorkersPublisher(configuration, secrets)
	if err != nil {
		return publisher.PublicationResult{}, err
	}
	if err := remote.Healthcheck(); err != nil {
		return publisher.PublicationResult{}, err
	}
	if _, err := remote.Plan(clientSet); err != nil {
		return publisher.PublicationResult{}, err
	}
	result, err := remote.PublishWithResult(ctx, clientSet)
	if err != nil {
		return publisher.PublicationResult{}, err
	}
	if err := writePublicationState(publicationState{
		SchemaVersion: subscriptionSchemaVersion, Status: "COMMITTED", PublicationID: result.PublicationID,
		ClientRevision: result.NodeRevision, RulesetRevision: clientSet.RulesetRevision, PublishedAt: result.PublishedAt, CheckedAt: time.Now().UTC(),
	}); err != nil {
		return result, err
	}
	return result, nil
}

func attemptAutoPublishSubscription(ctx context.Context) subscriptionPostCommit {
	configuration, err := readSubscriptionConfiguration()
	if errors.Is(err, os.ErrNotExist) {
		return subscriptionPostCommit{Status: "NOT_CONFIGURED"}
	}
	if err != nil {
		_ = recordPublicationFailure(err)
		return subscriptionPostCommit{Status: "DEGRADED", LastError: sanitizeError(err, "")}
	}
	if !configuration.AutoPublish {
		return subscriptionPostCommit{Status: "DISABLED"}
	}
	result, err := publishCurrentSubscription(ctx)
	if err != nil {
		_ = recordPublicationFailure(err)
		return subscriptionPostCommit{Status: "DEGRADED", LastError: sanitizeError(err, "")}
	}
	return subscriptionPostCommit{Status: "PASS", PublicationID: result.PublicationID}
}

func readSubscriptionConfiguration() (subscriptionConfig, error) {
	bytes, err := os.ReadFile(subscriptionConfigPath)
	if err != nil {
		return subscriptionConfig{}, err
	}
	var configuration subscriptionConfig
	decoder := json.NewDecoder(strings.NewReader(string(bytes)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&configuration); err != nil {
		return subscriptionConfig{}, fmt.Errorf("parse subscription configuration: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return subscriptionConfig{}, fmt.Errorf("parse subscription configuration: %w", err)
	}
	if configuration.SchemaVersion != subscriptionSchemaVersion || configuration.Backend != "workers" {
		return subscriptionConfig{}, errors.New("subscription configuration schema is unsupported")
	}
	return configuration, nil
}

func recordPublicationFailure(publicationErr error) error {
	state, err := readPublicationState()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	state.SchemaVersion = subscriptionSchemaVersion
	state.Status = "DEGRADED"
	state.CheckedAt = time.Now().UTC()
	state.LastError = sanitizeError(publicationErr, "")
	return writePublicationState(state)
}

func currentClientArtifactSet(state model.State) (artifact.Set, error) {
	secrets, err := readInstalledSecrets()
	if err != nil {
		return artifact.Set{}, err
	}
	values, err := runtimeValuesForRender(state, secrets)
	if err != nil {
		return artifact.Set{}, err
	}
	set, err := render.ClientArtifactSet(values)
	if err != nil {
		return artifact.Set{}, err
	}
	return appendManagedRuleCacheArtifacts(state, set)
}

func subscriptionDocuments(installedNodeID string, input subscriptionCredentialInput) (subscriptionConfig, subscriptionSecrets, error) {
	configuration := subscriptionConfig{
		SchemaVersion: subscriptionSchemaVersion, Backend: "workers", Endpoint: strings.TrimRight(strings.TrimSpace(input.Endpoint), "/"),
		NodeID: strings.TrimSpace(input.NodeID), Targets: []string{"mihomo", "v2rayn", "manifest"}, AutoPublish: true,
	}
	secrets := subscriptionSecrets{SchemaVersion: subscriptionSchemaVersion, NodePublishSecret: strings.TrimSpace(input.NodePublishSecret), ReadToken: strings.TrimSpace(input.ReadToken)}
	if input.SchemaVersion != subscriptionSchemaVersion {
		return subscriptionConfig{}, subscriptionSecrets{}, errors.New("unsupported subscription credential schema")
	}
	if configuration.NodeID != installedNodeID {
		return subscriptionConfig{}, subscriptionSecrets{}, errors.New("subscription credential node ID does not match the installed node")
	}
	if !validSubscriptionToken(secrets.ReadToken) {
		return subscriptionConfig{}, subscriptionSecrets{}, errors.New("subscription read token must contain at least 256 bits of entropy")
	}
	if !validSubscriptionToken(secrets.NodePublishSecret) {
		return subscriptionConfig{}, subscriptionSecrets{}, errors.New("node publish secret must contain at least 256 bits of entropy")
	}
	if _, err := newWorkersPublisher(configuration, secrets); err != nil {
		return subscriptionConfig{}, subscriptionSecrets{}, err
	}
	return configuration, secrets, nil
}

func newWorkersPublisher(configuration subscriptionConfig, secrets subscriptionSecrets) (*publisher.Workers, error) {
	if configuration.SchemaVersion != subscriptionSchemaVersion || configuration.Backend != "workers" || secrets.SchemaVersion != subscriptionSchemaVersion {
		return nil, errors.New("subscription configuration schema is unsupported")
	}
	return publisher.NewWorkers(publisher.WorkersConfig{
		Endpoint: configuration.Endpoint, NodeID: configuration.NodeID,
		PublishSecret: secrets.NodePublishSecret, ReadToken: secrets.ReadToken,
	})
}

func readSubscriptionCredentialInput(path string) (subscriptionCredentialInput, error) {
	bytes, err := readSensitiveRegularFile(path)
	if err != nil {
		return subscriptionCredentialInput{}, err
	}
	var input subscriptionCredentialInput
	decoder := json.NewDecoder(strings.NewReader(string(bytes)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return subscriptionCredentialInput{}, fmt.Errorf("parse subscription credential file: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return subscriptionCredentialInput{}, fmt.Errorf("parse subscription credential file: %w", err)
	}
	return input, nil
}

func readSubscriptionDocuments() (subscriptionConfig, subscriptionSecrets, error) {
	configuration, err := readSubscriptionConfiguration()
	if err != nil {
		return subscriptionConfig{}, subscriptionSecrets{}, fmt.Errorf("read subscription configuration: %w", err)
	}
	secretBytes, err := readSensitiveRegularFile(subscriptionSecretPath)
	if err != nil {
		return subscriptionConfig{}, subscriptionSecrets{}, err
	}
	var secrets subscriptionSecrets
	decoder := json.NewDecoder(strings.NewReader(string(secretBytes)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&secrets); err != nil {
		return subscriptionConfig{}, subscriptionSecrets{}, fmt.Errorf("parse subscription secrets: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return subscriptionConfig{}, subscriptionSecrets{}, fmt.Errorf("parse subscription secrets: %w", err)
	}
	if _, err := newWorkersPublisher(configuration, secrets); err != nil {
		return subscriptionConfig{}, subscriptionSecrets{}, err
	}
	return configuration, secrets, nil
}

func configureSubscriptionFiles(configuration subscriptionConfig, secrets subscriptionSecrets) (returnErr error) {
	lock, err := platform.AcquireProcessLock(subscriptionLockPath)
	if err != nil {
		return err
	}
	defer lock.Close()
	paths := []string{subscriptionConfigPath, subscriptionSecretPath, publicationStatePath, subscriptionOwnershipPath}
	snapshots := make([]optionalFileSnapshot, 0, len(paths))
	for _, path := range paths {
		snapshot, err := snapshotOptionalRegularFile(path)
		if err != nil {
			return err
		}
		snapshots = append(snapshots, snapshot)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		for index := len(snapshots) - 1; index >= 0; index-- {
			_ = restoreOptionalFile(snapshots[index])
		}
	}()
	configBytes, _ := json.MarshalIndent(configuration, "", "  ")
	secretBytes, _ := json.MarshalIndent(secrets, "", "  ")
	stateBytes, _ := json.MarshalIndent(publicationState{SchemaVersion: subscriptionSchemaVersion, Status: "CONFIGURED"}, "", "  ")
	if err := fsutil.WriteFileAtomic(subscriptionSecretPath, append(secretBytes, '\n'), 0o600); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(subscriptionConfigPath, append(configBytes, '\n'), 0o644); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(publicationStatePath, append(stateBytes, '\n'), 0o600); err != nil {
		return err
	}
	if err := addSubscriptionOwnership(); err != nil {
		return err
	}
	committed = true
	return nil
}

func addSubscriptionOwnership() error {
	document, err := readSubscriptionOwnership()
	if err != nil {
		return err
	}
	for _, path := range []string{subscriptionConfigPath, subscriptionSecretPath, publicationStatePath} {
		appendOwnershipValue(document, "files", path)
	}
	bytes, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(subscriptionOwnershipPath, append(bytes, '\n'), 0o600)
}

func removeSubscriptionOwnership() error {
	document, err := readSubscriptionOwnership()
	if err != nil {
		return err
	}
	blocked := map[string]bool{subscriptionConfigPath: true, subscriptionSecretPath: true, publicationStatePath: true}
	raw, ok := document["files"].([]any)
	if !ok {
		return errors.New("ownership files field is invalid")
	}
	filtered := make([]any, 0, len(raw))
	for _, item := range raw {
		value, ok := item.(string)
		if !ok {
			return errors.New("ownership files field contains a non-string value")
		}
		if !blocked[value] {
			filtered = append(filtered, value)
		}
	}
	document["files"] = filtered
	bytes, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(subscriptionOwnershipPath, append(bytes, '\n'), 0o600)
}

func readSubscriptionOwnership() (map[string]any, error) {
	bytes, err := os.ReadFile(subscriptionOwnershipPath)
	if err != nil {
		return nil, fmt.Errorf("read ownership: %w", err)
	}
	var ownership map[string]any
	if err := json.Unmarshal(bytes, &ownership); err != nil {
		return nil, fmt.Errorf("parse ownership: %w", err)
	}
	if ownership == nil {
		return nil, errors.New("ownership document is empty")
	}
	return ownership, nil
}

func writePublicationState(state publicationState) error {
	bytes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(publicationStatePath, append(bytes, '\n'), 0o600)
}

func readPublicationState() (publicationState, error) {
	bytes, err := os.ReadFile(publicationStatePath)
	if err != nil {
		return publicationState{}, err
	}
	var state publicationState
	if err := json.Unmarshal(bytes, &state); err != nil {
		return publicationState{}, err
	}
	return state, nil
}

func writeSubscriptionSecrets(secrets subscriptionSecrets) error {
	bytes, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(subscriptionSecretPath, append(bytes, '\n'), 0o600)
}

func writeSubscriptionClientFile(path string, configuration subscriptionConfig, token string) error {
	document := subscriptionClientFile{
		SchemaVersion: subscriptionSchemaVersion, Endpoint: configuration.Endpoint, NodeID: configuration.NodeID, ReadToken: token,
		URLs: subscriptionURLs(configuration.Endpoint, token),
	}
	bytes, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, append(bytes, '\n'), 0o600)
}

func readTokenFromClientFile(path string) (string, error) {
	bytes, err := readSensitiveRegularFile(path)
	if err != nil {
		return "", err
	}
	var document subscriptionClientFile
	if err := json.Unmarshal(bytes, &document); err != nil {
		return "", err
	}
	if !validSubscriptionToken(document.ReadToken) {
		return "", errors.New("client subscription file contains an invalid read token")
	}
	return strings.TrimSpace(document.ReadToken), nil
}

func validSubscriptionToken(value string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	return err == nil && len(decoded) >= 32
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func subscriptionURLs(endpoint, token string) map[string]string {
	root := strings.TrimRight(endpoint, "/") + "/s/" + token + "/"
	return map[string]string{"mihomo": root + "mihomo", "v2rayn": root + "v2rayn", "manifest": root + "manifest", "rules": root + "rules/"}
}

func randomSubscriptionToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func readSensitiveRegularFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing non-regular sensitive file: %s", path)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("sensitive file must not be readable by group or others: %s", path)
	}
	return os.ReadFile(path)
}

func snapshotOptionalRegularFile(path string) (optionalFileSnapshot, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return optionalFileSnapshot{path: path}, nil
	}
	if err != nil {
		return optionalFileSnapshot{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return optionalFileSnapshot{}, fmt.Errorf("refusing non-regular managed path: %s", path)
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return optionalFileSnapshot{}, err
	}
	return optionalFileSnapshot{path: path, bytes: bytes, mode: info.Mode().Perm(), exists: true}, nil
}

func restoreOptionalFile(snapshot optionalFileSnapshot) error {
	if !snapshot.exists {
		if err := os.Remove(snapshot.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return fsutil.WriteFileAtomic(snapshot.path, snapshot.bytes, snapshot.mode)
}
