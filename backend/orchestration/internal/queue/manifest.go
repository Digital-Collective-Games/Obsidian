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

// RepoEntry is one repos[] entry — the first-class per-repo binding the
// registry-driven consumer iterates. LocalRoot is an ARBITRARY absolute path (no
// co-location assumption); TaskProvider is the first-class provider abstraction the
// consumer polls (the consumer constructs its provider from TaskProvider.Repo, not
// from a single env string). Unknown fields are ignored so the registry can carry
// config the consumer does not consult — including a legacy queue_workers key, which
// Task-0016 removed as an admission cap (concurrency is now bounded by the count of
// idle pool worktrees) and which is simply no longer decoded.
type RepoEntry struct {
	ID                    string                 `json:"id"`
	LocalRoot             string                 `json:"local_root"`
	SourceControlProvider *SourceControlProvider `json:"source_control_provider,omitempty"`
	TaskProvider          *TaskProvider          `json:"task_provider,omitempty"`
}

// SourceControlProvider is the repo's source-control binding (e.g. git remote).
// It is decoded as a first-class part of the registry entry; the consumer does not
// act on it directly today, but it is part of the single-source-of-truth binding.
type SourceControlProvider struct {
	Kind             string `json:"kind"`
	DefaultAgentUser string `json:"default_agent_user,omitempty"`
	Remote           string `json:"remote,omitempty"`
	Repo             string `json:"repo,omitempty"`
	URL              string `json:"url,omitempty"`
}

// TaskProvider is the repo's queue task source (e.g. github_issues). Repo is the
// owner/name the consumer polls for Queue==Ready; the consumer builds its
// QueueProvider from this entry, NOT from a single CODEX_ORCHESTRATION_QUEUE_DRAIN_REPO.
type TaskProvider struct {
	Kind           string `json:"kind"`
	Host           string `json:"host,omitempty"`
	Repo           string `json:"repo"`
	CanonicalQuery string `json:"canonical_query,omitempty"`
}

// LoadManifest reads REPO-MANIFEST.json from the given repo root. It is retained
// for the legacy single-task dispatch path (repoIdentity / co-located slot sizing);
// the registry-driven consumer uses LoadRegistry with an explicit path instead.
func LoadManifest(repoRoot string) (RepoManifest, error) {
	return LoadRegistry(filepath.Join(repoRoot, ManifestFileName))
}

// LoadRegistry reads the central repo registry from an EXPLICIT file path (the
// configured OBSIDIAN_REGISTRY_PATH), rather than assuming the file sits at a
// declared worktree root. It is the single source of truth the registry-driven
// consumer enumerates: every repos[] binding (id, local_root, task_provider) is
// returned for the consumer to iterate.
func LoadRegistry(registryPath string) (RepoManifest, error) {
	raw, err := os.ReadFile(registryPath)
	if err != nil {
		return RepoManifest{}, fmt.Errorf("read %s: %w", registryPath, err)
	}
	var manifest RepoManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return RepoManifest{}, fmt.Errorf("decode %s: %w", registryPath, err)
	}
	return manifest, nil
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
