package queue

import (
	"context"
	"fmt"
	"sort"
)

// RegistryRepo is one registered repo's per-poll dispatch binding the
// registry-driven consumer acts on: the provider repo it polls
// (task_provider.repo) and the local_root it dispatches worktrees into. It is the
// minimal, first-class slice of a manifest RepoEntry the consumer needs (no
// co-location assumptions, no single CODEX_ORCHESTRATION_QUEUE_DRAIN_REPO env string).
// There is no per-repo numeric cap: concurrency is bounded by the count of idle pool
// worktrees (Task-0016).
type RegistryRepo struct {
	// ID is the repos[] id (for logging/proof).
	ID string
	// LocalRoot is the ARBITRARY absolute path the consumer dispatches into and
	// maps #N -> <LocalRoot>/Tracking/Task-N.
	LocalRoot string
	// ProviderRepo is task_provider.repo (owner/name) the consumer polls for
	// Queue==Ready. The provider abstraction is built from this, not an env string.
	ProviderRepo string
}

// RegistryRepos extracts the dispatch bindings from a loaded registry: every
// repos[] entry that carries a usable task_provider.repo and local_root. An entry
// missing either is skipped (it has nothing for the consumer to poll/dispatch).
// Bindings are sorted by id for deterministic iteration/proof.
func (m RepoManifest) RegistryRepos() []RegistryRepo {
	repos := make([]RegistryRepo, 0, len(m.Repos))
	for _, entry := range m.Repos {
		if entry.TaskProvider == nil || entry.TaskProvider.Repo == "" || entry.LocalRoot == "" {
			continue
		}
		repos = append(repos, RegistryRepo{
			ID:           entry.ID,
			LocalRoot:    entry.LocalRoot,
			ProviderRepo: entry.TaskProvider.Repo,
		})
	}
	sort.Slice(repos, func(i, j int) bool { return repos[i].ID < repos[j].ID })
	return repos
}

// RegistryLoader reads the central registry from the configured explicit path on
// each poll, so a registry edit (a new repo, a changed queue_workers) is picked up
// on the next cycle without restarting the consumer (global awareness).
type RegistryLoader func() (RepoManifest, error)

// RepoDispatch is the per-repo provider + dispatcher + pool sizer the registry-driven
// consumer drives for one RegistryRepo. Production builds these from the entry (gh
// provider polling ProviderRepo; a taskrun.Service bound to LocalRoot; a sizer reporting
// that repo's idle pool worktree count); tests inject fakes so the iteration and
// per-repo idle accounting are provable WITHOUT a real registry, GitHub, or git worktree.
type RepoDispatch struct {
	Provider   QueueProvider
	Dispatcher Dispatcher
	Sizer      PoolSizer
}

// RepoDispatchFactory builds the per-repo dispatch binding for one RegistryRepo.
// It is the seam that keeps the Service repo-parameterized: the consumer passes the
// registry entry's LocalRoot in, and the factory returns a Consumer wired to a
// per-LocalRoot Service (its idle pool worktree count is naturally per-repo).
type RepoDispatchFactory func(repo RegistryRepo) (RepoDispatch, error)

// RegistryConsumer is the registry-driven queue-drain consumer. On each poll it
// reads the central registry (explicit path) and, for EACH registered repo, polls
// that repo's task_provider.repo for Queue==Ready, maps #N ->
// <local_root>/Tracking/Task-N, checks slot availability for THAT repo (cap = the
// entry's queue_workers, used = active owned lanes of THAT repo), and dispatches
// into a worktree of THAT repo's local_root. Global awareness = iterating all
// registry repos each poll.
type RegistryConsumer struct {
	loadRegistry RegistryLoader
	dispatchFor  RepoDispatchFactory
	// autoCloseEnabled is propagated onto every per-repo Consumer this builds, so the
	// TEST-ONLY OBSIDIAN_AUTO_CLOSE_QUEUED auto-close is uniform across all registry
	// repos. Default false keeps every per-repo consumer read-only against GitHub.
	autoCloseEnabled bool
}

// NewRegistryConsumer builds the registry-driven consumer over a registry loader
// (explicit path) and a per-repo dispatch factory.
func NewRegistryConsumer(loadRegistry RegistryLoader, dispatchFor RepoDispatchFactory) *RegistryConsumer {
	return &RegistryConsumer{loadRegistry: loadRegistry, dispatchFor: dispatchFor}
}

// SetAutoCloseEnabled toggles the TEST-ONLY simulated-human auto-close, propagated to
// each per-repo Consumer this builds. It is set from the OBSIDIAN_AUTO_CLOSE_QUEUED
// config flag at wiring time.
func (c *RegistryConsumer) SetAutoCloseEnabled(enabled bool) {
	c.autoCloseEnabled = enabled
}

// DrainOnce performs one poll cycle across ALL registered repos. It loads the
// registry, then for each repo builds the per-repo dispatch binding and runs a
// per-repo Consumer.DrainOnce (which polls the entry's provider repo, maps #N to
// that repo's Tracking/Task-N, and dispatches into that repo's local_root capped at
// its queue_workers). A failure to load the registry aborts the cycle; a per-repo
// build/drain failure is collected and returned after all repos are attempted, so
// one bad repo does not wedge the others (the loop continues on the next tick).
func (c *RegistryConsumer) DrainOnce(ctx context.Context) (RegistryDrainResult, error) {
	out := RegistryDrainResult{ByRepo: map[string]DrainResult{}}
	if c.loadRegistry == nil || c.dispatchFor == nil {
		return out, nil
	}
	manifest, err := c.loadRegistry()
	if err != nil {
		return out, fmt.Errorf("load registry: %w", err)
	}

	var firstErr error
	for _, repo := range manifest.RegistryRepos() {
		dispatch, err := c.dispatchFor(repo)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("build dispatch for %s: %w", repo.ID, err)
			}
			continue
		}
		consumer := NewConsumer(repo.ProviderRepo, dispatch.Provider, dispatch.Dispatcher, dispatch.Sizer)
		consumer.SetAutoCloseEnabled(c.autoCloseEnabled)
		result, err := consumer.DrainOnce(ctx)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("drain %s (%s): %w", repo.ID, repo.ProviderRepo, err)
			}
			continue
		}
		out.ByRepo[repo.ID] = result
		out.Dispatched = append(out.Dispatched, result.Dispatched...)
		out.Parked = append(out.Parked, result.Parked...)
		out.Reclaimed = append(out.Reclaimed, result.Reclaimed...)
		out.Skipped = append(out.Skipped, result.Skipped...)
	}
	return out, firstErr
}

// RegistryDrainResult aggregates one RegistryConsumer.DrainOnce across all repos:
// the flattened action lists (for the workflow's "acted" log) plus the per-repo
// breakdown keyed by repo id (for proof).
type RegistryDrainResult struct {
	Dispatched []string
	Parked     []string
	Reclaimed  []string
	Skipped    []string
	ByRepo     map[string]DrainResult
}
