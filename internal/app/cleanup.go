package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/fsutil"
	"vpskit.local/vpskit/internal/platform"
)

type cleanupCandidate struct {
	Category string    `json:"category"`
	ID       string    `json:"id"`
	Path     string    `json:"path"`
	Bytes    int64     `json:"bytes"`
	Modified time.Time `json:"modified_at"`
}

type cleanupPlan struct {
	GeneratedAt      time.Time          `json:"generated_at"`
	KeepBackups      int                `json:"keep_backups"`
	KeepTransactions int                `json:"keep_transactions"`
	OlderThan        string             `json:"older_than"`
	Candidates       []cleanupCandidate `json:"candidates"`
	ReclaimableBytes int64              `json:"reclaimable_bytes"`
}

func runCleanup(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: vpskit cleanup <plan|apply> [--keep-backups 5] [--keep-transactions 20] [--older-than 168h]")
	}
	if !platform.IsRoot() {
		return errors.New("cleanup requires root privileges")
	}
	operation := arguments[0]
	flags := flag.NewFlagSet("cleanup "+operation, flag.ContinueOnError)
	keepBackups := flags.Int("keep-backups", 5, "number of newest unprotected backups to retain")
	keepTransactions := flags.Int("keep-transactions", 20, "number of newest terminal transactions to retain")
	olderThan := flags.Duration("older-than", 168*time.Hour, "minimum age before removal")
	yes := flags.Bool("yes", false, "confirm cleanup apply")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *keepBackups < 1 || *keepTransactions < 1 || *olderThan < time.Hour {
		return errors.New("cleanup retention values are invalid")
	}
	if operation == "plan" {
		if *yes {
			return errors.New("cleanup plan does not accept --yes")
		}
		plan, err := buildCleanupPlan(backupRoot, transactionRoot, time.Now().UTC(), *keepBackups, *keepTransactions, *olderThan, referencedBackupIDs())
		if err != nil {
			return err
		}
		return printJSON(commandResult{Command: "cleanup plan", Status: "PASS", Detail: plan})
	}
	if operation != "apply" {
		return fmt.Errorf("unsupported cleanup operation %q", operation)
	}
	if !*yes {
		return errors.New("cleanup apply requires explicit --yes confirmation")
	}
	lock, err := platform.AcquireProcessLock(filepath.Join(stateRoot, "locks", "vpskit.lock"))
	if err != nil {
		return err
	}
	defer lock.Close()
	plan, err := buildCleanupPlan(backupRoot, transactionRoot, time.Now().UTC(), *keepBackups, *keepTransactions, *olderThan, referencedBackupIDs())
	if err != nil {
		return err
	}
	removed := make([]string, 0, len(plan.Candidates))
	for _, candidate := range plan.Candidates {
		root := backupRoot
		if candidate.Category == "transaction" {
			root = transactionRoot
		}
		if err := fsutil.RemoveManagedTree(candidate.Path, root); err != nil {
			return err
		}
		removed = append(removed, candidate.ID)
	}
	_ = appendAudit(map[string]any{
		"time":            time.Now().UTC(),
		"command":         "cleanup apply",
		"status":          "COMMITTED",
		"removed":         removed,
		"reclaimed_bytes": plan.ReclaimableBytes,
	})
	return printJSON(commandResult{Command: "cleanup apply", Status: "PASS", Detail: map[string]any{
		"removed":          removed,
		"removed_count":    len(removed),
		"reclaimed_bytes":  plan.ReclaimableBytes,
		"retention_policy": plan,
	}})
}

func buildCleanupPlan(backups, transactions string, now time.Time, keepBackups, keepTransactions int, olderThan time.Duration, protected map[string]bool) (cleanupPlan, error) {
	plan := cleanupPlan{GeneratedAt: now, KeepBackups: keepBackups, KeepTransactions: keepTransactions, OlderThan: olderThan.String(), Candidates: []cleanupCandidate{}}
	backupCandidates, err := cleanupDirectoryCandidates(backups, "BK-", "backup", now, keepBackups, olderThan, func(id, _ string) bool { return !protected[id] })
	if err != nil {
		return cleanupPlan{}, err
	}
	transactionCandidates, err := cleanupDirectoryCandidates(transactions, "TX-", "transaction", now, keepTransactions, olderThan, terminalTransaction)
	if err != nil {
		return cleanupPlan{}, err
	}
	plan.Candidates = append(plan.Candidates, backupCandidates...)
	plan.Candidates = append(plan.Candidates, transactionCandidates...)
	sort.SliceStable(plan.Candidates, func(i, j int) bool {
		if plan.Candidates[i].Category == plan.Candidates[j].Category {
			return plan.Candidates[i].ID < plan.Candidates[j].ID
		}
		return plan.Candidates[i].Category < plan.Candidates[j].Category
	})
	for _, candidate := range plan.Candidates {
		plan.ReclaimableBytes += candidate.Bytes
	}
	return plan, nil
}

func cleanupDirectoryCandidates(root, prefix, category string, now time.Time, keep int, olderThan time.Duration, removable func(string, string) bool) ([]cleanupCandidate, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	type retainedEntry struct {
		id       string
		path     string
		modified time.Time
	}
	eligible := make([]retainedEntry, 0)
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		path := filepath.Join(root, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if removable(entry.Name(), path) {
			eligible = append(eligible, retainedEntry{id: entry.Name(), path: path, modified: info.ModTime().UTC()})
		}
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		if eligible[i].modified.Equal(eligible[j].modified) {
			return eligible[i].id > eligible[j].id
		}
		return eligible[i].modified.After(eligible[j].modified)
	})
	candidates := make([]cleanupCandidate, 0)
	for index, entry := range eligible {
		if index < keep || now.Sub(entry.modified) < olderThan {
			continue
		}
		size, err := directoryBytes(entry.path)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, cleanupCandidate{Category: category, ID: entry.id, Path: entry.path, Bytes: size, Modified: entry.modified})
	}
	return candidates, nil
}

func terminalTransaction(_ string, path string) bool {
	content, err := os.ReadFile(filepath.Join(path, "transaction.json"))
	if err != nil {
		return false
	}
	var record struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(content, &record) != nil {
		return false
	}
	return record.Status == "COMMITTED" || record.Status == "ROLLED_BACK"
}

func directoryBytes(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}
