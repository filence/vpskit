package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"vpskit.local/vpskit/internal/model"
)

type stateEnvelope struct {
	SchemaVersion int `json:"schema_version"`
}

func decodeInstalledState(stateBytes []byte) (model.State, error) {
	envelope, err := decodeStateEnvelope(stateBytes)
	if err != nil {
		return model.State{}, err
	}
	if envelope.SchemaVersion < 1 {
		return model.State{}, errors.New("state schema_version must be at least 1")
	}
	if envelope.SchemaVersion > model.SchemaVersion {
		return model.State{}, fmt.Errorf("state schema %d is newer than supported schema %d; refusing to rewrite it", envelope.SchemaVersion, model.SchemaVersion)
	}
	var state model.State
	if err := json.Unmarshal(stateBytes, &state); err != nil {
		return model.State{}, fmt.Errorf("parse state: %w", err)
	}
	return migrateState(state)
}

func decodeStateEnvelope(stateBytes []byte) (stateEnvelope, error) {
	var envelope stateEnvelope
	if err := json.Unmarshal(stateBytes, &envelope); err != nil {
		return stateEnvelope{}, fmt.Errorf("parse state schema: %w", err)
	}
	return envelope, nil
}

func installedStateSchemaVersion() (int, error) {
	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		return 0, fmt.Errorf("read state schema: %w", err)
	}
	envelope, err := decodeStateEnvelope(stateBytes)
	if err != nil {
		return 0, err
	}
	return envelope.SchemaVersion, nil
}

func migrateState(state model.State) (model.State, error) {
	for state.SchemaVersion < model.SchemaVersion {
		switch state.SchemaVersion {
		case 1:
			state.Core.Channel = "pinned"
			state.SchemaVersion = 2
		case 2:
			legacyBalanced := state.Profile == "" || state.Profile == "balanced"
			state.Reality.Enabled = state.Reality.ID != "" || legacyBalanced
			state.Hysteria2.Enabled = state.Hysteria2.ID != "" || (legacyBalanced && (state.Hysteria2.CertificatePath != "" || state.Domain != ""))
			if state.Profile == "" {
				state.Profile = profileFromInboundState(state.Reality.Enabled, state.Hysteria2.Enabled)
			}
			state.SchemaVersion = 3
		case 3:
			// Schema 4 splits REALITY onto a separately pinned Xray core. The
			// self-update transaction fills version, digest and config hash from
			// the signed bundle before committing the migrated state.
			state.RealityCore = model.CoreState{ID: "xray", Channel: "pinned", Path: installedXray}
			state.SchemaVersion = 4
		case 4:
			// Schema 5 records the revision of client-facing connection
			// parameters. Existing installations start at revision 1; only
			// mutations that regenerate client exports increment it.
			state.ConfigRevision = 1
			state.SchemaVersion = 5
		case 5:
			// Schema 6 adds stable node identity and client-facing labels without
			// changing legacy export names. Existing installations keep the JP
			// display prefix until the owner explicitly changes it.
			state.Node = model.NodeMetadata{
				ID:                    "node-main",
				DisplayName:           "JP",
				Priority:              100,
				EnabledInSubscription: true,
			}
			state.SchemaVersion = 6
		default:
			return model.State{}, fmt.Errorf("no migration from state schema %d", state.SchemaVersion)
		}
	}
	if state.ConfigRevision < 1 {
		state.ConfigRevision = 1
	}
	if err := validateNodeMetadata(state.Node); err != nil {
		return model.State{}, fmt.Errorf("invalid schema %d node metadata: %w", state.SchemaVersion, err)
	}
	return state, nil
}

func profileFromInboundState(realityEnabled, hysteria2Enabled bool) string {
	switch {
	case realityEnabled && hysteria2Enabled:
		return "balanced"
	case realityEnabled:
		return "reality-only"
	case hysteria2Enabled:
		return "hysteria2-only"
	default:
		return "empty"
	}
}
