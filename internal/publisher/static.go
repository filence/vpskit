// Package publisher activates validated client artifact sets through pluggable backends.
package publisher

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"vpskit.local/vpskit/internal/artifact"
	"vpskit.local/vpskit/internal/fsutil"
)

type Change struct {
	Name   string `json:"name"`
	Action string `json:"action"`
	SHA256 string `json:"sha256"`
}

type Plan struct {
	Publisher     string   `json:"publisher"`
	PublicationID string   `json:"publication_id"`
	Changes       []Change `json:"changes"`
}

type Publisher interface {
	Preflight(artifact.Set) error
	Plan(artifact.Set) (Plan, error)
	Publish(artifact.Set) error
	Readback(artifact.Set) error
	Activate(string) error
	Rollback(string) error
	RotateReadToken() error
	RevokeReadToken() error
	Healthcheck() error
}

type Static struct {
	Root string
	Mode os.FileMode
}

func (publisher Static) Preflight(set artifact.Set) error {
	if set.SchemaVersion != artifact.SchemaVersion || set.NodeID == "" || set.ClientRevision < 1 || len(set.Artifacts) == 0 {
		return errors.New("invalid artifact set for static publisher")
	}
	if !filepath.IsAbs(publisher.Root) {
		return errors.New("static publisher root must be absolute")
	}
	for _, item := range set.Artifacts {
		if filepath.Base(item.Name) != item.Name || item.Name == "." || len(item.Content) == 0 {
			return fmt.Errorf("invalid static artifact %q", item.Name)
		}
		digest := sha256.Sum256(item.Content)
		if hex.EncodeToString(digest[:]) != item.SHA256 {
			return fmt.Errorf("artifact %s digest does not match content", item.Name)
		}
	}
	return nil
}

func (publisher Static) Plan(set artifact.Set) (Plan, error) {
	if err := publisher.Preflight(set); err != nil {
		return Plan{}, err
	}
	changes := make([]Change, 0, len(set.Artifacts))
	for _, item := range set.Artifacts {
		action := "create"
		content, err := os.ReadFile(filepath.Join(publisher.Root, item.Name))
		if err == nil {
			digest := sha256.Sum256(content)
			if hex.EncodeToString(digest[:]) == item.SHA256 {
				action = "unchanged"
			} else {
				action = "update"
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return Plan{}, fmt.Errorf("inspect static artifact %s: %w", item.Name, err)
		}
		changes = append(changes, Change{Name: item.Name, Action: action, SHA256: item.SHA256})
	}
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].Name < changes[j].Name })
	return Plan{Publisher: "static", PublicationID: set.PublicationID, Changes: changes}, nil
}

func (publisher Static) Publish(set artifact.Set) error {
	if err := publisher.Preflight(set); err != nil {
		return err
	}
	mode := publisher.Mode
	if mode == 0 {
		mode = 0o600
	}
	if err := os.MkdirAll(publisher.Root, 0o700); err != nil {
		return fmt.Errorf("create static publisher root: %w", err)
	}
	for _, item := range set.Artifacts {
		if err := fsutil.WriteFileAtomic(filepath.Join(publisher.Root, item.Name), item.Content, mode); err != nil {
			return fmt.Errorf("publish static artifact %s: %w", item.Name, err)
		}
	}
	return publisher.Readback(set)
}

func (publisher Static) Readback(set artifact.Set) error {
	if err := publisher.Preflight(set); err != nil {
		return err
	}
	for _, item := range set.Artifacts {
		path := filepath.Join(publisher.Root, item.Name)
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("read back static artifact %s: %w", item.Name, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("static artifact %s is not a regular file", item.Name)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(content)
		if hex.EncodeToString(digest[:]) != item.SHA256 {
			return fmt.Errorf("static artifact %s failed digest readback", item.Name)
		}
	}
	return nil
}

func (publisher Static) Activate(string) error { return publisher.Healthcheck() }

func (publisher Static) Rollback(string) error {
	return errors.New("static publisher rollback is owned by the enclosing VPSKit transaction")
}

func (publisher Static) RotateReadToken() error {
	return errors.New("static publisher does not use read tokens")
}

func (publisher Static) RevokeReadToken() error {
	return errors.New("static publisher does not use read tokens")
}

func (publisher Static) Healthcheck() error {
	if !filepath.IsAbs(publisher.Root) {
		return errors.New("static publisher root must be absolute")
	}
	info, err := os.Stat(publisher.Root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("static publisher root is not a directory")
	}
	return nil
}
