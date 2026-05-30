package queue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ManifestFileName is the repo registry file at the repo root (renamed from
// CODEX-REPO-MANIFEST.json in O1).
const ManifestFileName = "REPO-MANIFEST.json"

// RepoManifest is the minimal view of REPO-MANIFEST.json needed to size per-repo
// slots. Only the fields O2 consults are decoded; unknown fields are ignored so
// the manifest can carry provider config the slot logic does not care about.
type RepoManifest struct {
	Repos []RepoEntry `json:"repos"`
}

// RepoEntry is one repos[] entry. LocalRoot binds the entry to a worktree root on
// disk; QueueWorkers is the per-repo max concurrent owned lanes.
type RepoEntry struct {
	ID           string `json:"id"`
	LocalRoot    string `json:"local_root"`
	QueueWorkers int    `json:"queue_workers"`
}

// LoadManifest reads REPO-MANIFEST.json from the given repo root.
func LoadManifest(repoRoot string) (RepoManifest, error) {
	path := filepath.Join(repoRoot, ManifestFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		return RepoManifest{}, fmt.Errorf("read %s: %w", path, err)
	}
	var manifest RepoManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return RepoManifest{}, fmt.Errorf("decode %s: %w", path, err)
	}
	return manifest, nil
}

// QueueWorkersForRoot returns the queue_workers configured for the repos[] entry
// whose local_root matches worktreeRoot. It returns DefaultQueueWorkers (and ok
// false) when no entry matches or the entry omits a positive queue_workers, so a
// missing manifest never blocks dispatch — it just falls back to the default cap.
func (m RepoManifest) QueueWorkersForRoot(worktreeRoot string) (workers int, ok bool) {
	target := normalizeRoot(worktreeRoot)
	for _, entry := range m.Repos {
		if normalizeRoot(entry.LocalRoot) != target {
			continue
		}
		if entry.QueueWorkers > 0 {
			return entry.QueueWorkers, true
		}
		return DefaultQueueWorkers, false
	}
	return DefaultQueueWorkers, false
}

// RepoIDForRoot returns the configured id for the repos[] entry whose local_root
// matches worktreeRoot, or "" when no entry matches. O6 uses it to name the repo
// in the worktree<->session binding.
func (m RepoManifest) RepoIDForRoot(worktreeRoot string) string {
	target := normalizeRoot(worktreeRoot)
	for _, entry := range m.Repos {
		if normalizeRoot(entry.LocalRoot) == target {
			return entry.ID
		}
	}
	return ""
}

// normalizeRoot makes two paths comparable across separator and trailing-slash
// differences without resolving symlinks (the manifest stores literal roots).
func normalizeRoot(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	cleaned = filepath.ToSlash(cleaned)
	cleaned = strings.TrimRight(cleaned, "/")
	return strings.ToLower(cleaned)
}
