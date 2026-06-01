package queue

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// TestLoadRegistryDecodesProviders proves the registry loader decodes the first-class
// binding fields the registry-driven consumer needs: task_provider (kind, host, repo,
// canonical_query), source_control_provider, and local_root — read from an EXPLICIT path
// (not a co-located worktree-root lookup). A legacy queue_workers key is simply ignored
// (Task-0016 removed it as an admission cap).
func TestLoadRegistryDecodesProviders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "REPO-MANIFEST.json")
	const body = `{
  "schema_version": 1,
  "repos": [
    {
      "id": "TestbedA",
      "queue_workers": 3,
      "local_root": "C:\\Agent\\TestbedA",
      "source_control_provider": { "kind": "git", "default_agent_user": "gregsemple2003", "repo": "Org/Mirror" },
      "task_provider": { "kind": "github_issues", "host": "github.com", "repo": "Org/TestbedA", "canonical_query": "is:issue is:open" }
    }
  ]
}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture registry: %v", err)
	}

	manifest, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(manifest.Repos) != 1 {
		t.Fatalf("repos = %d, want 1", len(manifest.Repos))
	}
	entry := manifest.Repos[0]
	if entry.ID != "TestbedA" || entry.LocalRoot != `C:\Agent\TestbedA` {
		t.Fatalf("entry id/local_root = %q/%q, want TestbedA/C:\\Agent\\TestbedA", entry.ID, entry.LocalRoot)
	}
	if entry.SourceControlProvider == nil || entry.SourceControlProvider.Kind != "git" || entry.SourceControlProvider.Repo != "Org/Mirror" {
		t.Fatalf("source_control_provider = %#v, want git/Org/Mirror decoded", entry.SourceControlProvider)
	}
	if entry.TaskProvider == nil {
		t.Fatal("task_provider was not decoded")
	}
	if entry.TaskProvider.Kind != "github_issues" || entry.TaskProvider.Host != "github.com" ||
		entry.TaskProvider.Repo != "Org/TestbedA" || entry.TaskProvider.CanonicalQuery != "is:issue is:open" {
		t.Fatalf("task_provider = %#v, want github_issues/github.com/Org/TestbedA/is:issue is:open", entry.TaskProvider)
	}
}

// TestRegistryReposSkipsEntriesWithoutProviderOrRoot proves an entry missing a
// task_provider.repo or a local_root is not enumerated (nothing to poll/dispatch).
func TestRegistryReposSkipsEntriesWithoutProviderOrRoot(t *testing.T) {
	manifest := RepoManifest{Repos: []RepoEntry{
		{ID: "NoProvider", LocalRoot: `C:\Agent\NoProvider`},
		{ID: "NoRoot", TaskProvider: &TaskProvider{Repo: "Org/NoRoot"}},
		{ID: "Good", LocalRoot: `C:\Agent\Good`, TaskProvider: &TaskProvider{Repo: "Org/Good"}},
	}}
	repos := manifest.RegistryRepos()
	if len(repos) != 1 || repos[0].ID != "Good" {
		t.Fatalf("RegistryRepos = %#v, want only the Good entry", repos)
	}
}

// recordingDispatchFactory builds a per-repo fake dispatcher for each RegistryRepo and
// records which RegistryRepo it was asked to build, so a test can prove the consumer
// iterates EVERY registry repo, polls the entry's task_provider.repo, and dispatches into
// the entry's local_root drawing from THAT repo's idle pool. idleByRepo models each repo's
// idle pool worktree count (the per-repo admission budget).
type recordingDispatchFactory struct {
	providers   map[string]*fakeProvider // keyed by repo id
	dispatchers map[string]*fakeDispatcher
	idleByRepo  map[string]int
	builtRepos  []RegistryRepo
}

func (f *recordingDispatchFactory) build(repo RegistryRepo) (RepoDispatch, error) {
	f.builtRepos = append(f.builtRepos, repo)
	provider := f.providers[repo.ID]
	dispatcher := f.dispatchers[repo.ID]
	// The per-repo admission budget is THAT repo's idle pool worktree count (Task-0016),
	// modeled here by sizing the fake idle sizer per repo id.
	return RepoDispatch{Provider: provider, Dispatcher: dispatcher, Sizer: fixedIdleSizer(f.idleByRepo[repo.ID])}, nil
}

// TestRegistryConsumerIteratesAllReposPerRepoPool is the core registry-driven proof:
// a fake MULTI-ENTRY registry with two repos at DIFFERENT local_roots and DIFFERENT
// task_provider.repos. The consumer must, per repo: poll THAT entry's task_provider.repo,
// map #N -> Task-N for dispatch into THAT repo's local_root, and draw from THAT repo's
// own IDLE pool (one repo's idle worktrees never lend capacity to the other).
func TestRegistryConsumerIteratesAllReposPerRepoPool(t *testing.T) {
	const (
		rootA = `C:\Agent\RepoA`
		rootB = `C:\Agent\RepoB`
		provA = "Org/RepoA-issues"
		provB = "Org/RepoB-issues"
	)
	manifest := RepoManifest{Repos: []RepoEntry{
		// RepoB declared first to prove iteration is deterministic (sorted by id) and
		// independent of declaration order.
		{ID: "RepoB", LocalRoot: rootB, TaskProvider: &TaskProvider{Kind: "github_issues", Repo: provB}},
		{ID: "RepoA", LocalRoot: rootA, TaskProvider: &TaskProvider{Kind: "github_issues", Repo: provA}},
	}}

	// RepoA: two Ready issues, 2 idle worktrees -> both dispatch.
	// RepoB: two Ready issues but 0 idle worktrees (its only worktree is allocated to
	// Task-8000) -> neither dispatches, proving per-repo (not global) pool accounting.
	providers := map[string]*fakeProvider{
		"RepoA": {issues: []IssueRef{
			{Number: 101, State: IssueState{Queue: QueueReady}},
			{Number: 102, State: IssueState{Queue: QueueReady}},
		}},
		"RepoB": {issues: []IssueRef{
			{Number: 201, State: IssueState{Queue: QueueReady}},
			{Number: 202, State: IssueState{Queue: QueueReady}},
		}},
	}
	dispatchers := map[string]*fakeDispatcher{
		"RepoA": newFakeDispatcher(),
		"RepoB": newFakeDispatcher("Task-8000"), // RepoB's worktree is already allocated
	}
	factory := &recordingDispatchFactory{
		providers:   providers,
		dispatchers: dispatchers,
		idleByRepo:  map[string]int{"RepoA": 2, "RepoB": 0},
	}

	consumer := NewRegistryConsumer(func() (RepoManifest, error) { return manifest, nil }, factory.build)
	result, err := consumer.DrainOnce(context.Background())
	if err != nil {
		t.Fatalf("DrainOnce: %v", err)
	}

	// Iterated EVERY registry repo (global awareness), deterministically by id.
	if len(factory.builtRepos) != 2 || factory.builtRepos[0].ID != "RepoA" || factory.builtRepos[1].ID != "RepoB" {
		t.Fatalf("built repos = %#v, want [RepoA, RepoB] in id order", factory.builtRepos)
	}
	// Each repo was built with ITS OWN local_root + task_provider.repo.
	if factory.builtRepos[0].LocalRoot != rootA || factory.builtRepos[0].ProviderRepo != provA {
		t.Fatalf("RepoA binding = %#v, want local_root=%s provider=%s", factory.builtRepos[0], rootA, provA)
	}
	if factory.builtRepos[1].LocalRoot != rootB || factory.builtRepos[1].ProviderRepo != provB {
		t.Fatalf("RepoB binding = %#v, want local_root=%s provider=%s", factory.builtRepos[1], rootB, provB)
	}

	// Each provider was polled for ITS entry's task_provider.repo exactly once.
	if providers["RepoA"].calls != 1 || providers["RepoB"].calls != 1 {
		t.Fatalf("provider polls A=%d B=%d, want 1 each", providers["RepoA"].calls, providers["RepoB"].calls)
	}

	// RepoA dispatched both Ready issues into RepoA (2 idle worktrees). #N -> Task-N exact.
	if want := []string{"Task-0101", "Task-0102"}; !reflect.DeepEqual(dispatchers["RepoA"].dispatched, want) {
		t.Fatalf("RepoA dispatched = %v, want %v (both into RepoA's lane, 2 idle)", dispatchers["RepoA"].dispatched, want)
	}
	// RepoB dispatched NONE: its only worktree was already allocated (0 idle). Per-repo
	// accounting — RepoA's idle worktrees did NOT lend capacity to RepoB.
	if len(dispatchers["RepoB"].dispatched) != 0 {
		t.Fatalf("RepoB dispatched = %v, want none (0 idle worktrees per-repo)", dispatchers["RepoB"].dispatched)
	}

	// Aggregate result carries RepoA's dispatches and a per-repo breakdown.
	gotDispatched := append([]string(nil), result.Dispatched...)
	sort.Strings(gotDispatched)
	if want := []string{"Task-0101", "Task-0102"}; !reflect.DeepEqual(gotDispatched, want) {
		t.Fatalf("aggregate dispatched = %v, want %v", gotDispatched, want)
	}
	if result.ByRepo["RepoA"].Dispatched == nil || len(result.ByRepo["RepoB"].Dispatched) != 0 {
		t.Fatalf("per-repo breakdown = %#v, want RepoA dispatched + RepoB none", result.ByRepo)
	}
}

// TestRegistryConsumerProviderSourceIsRegistryNotEnv proves the consumer's provider
// repo comes from the registry entry's task_provider.repo, NOT from the legacy
// CODEX_ORCHESTRATION_QUEUE_DRAIN_REPO env. Setting that env to a decoy and leaving
// it out of the registry must not change which repo is polled: the consumer polls
// ONLY the registry's task_provider.repo, never the env value.
func TestRegistryConsumerProviderSourceIsRegistryNotEnv(t *testing.T) {
	const decoy = "Org/DECOY-from-env"
	const registryProvider = "Org/registry-provider"
	t.Setenv("CODEX_ORCHESTRATION_QUEUE_DRAIN_REPO", decoy)

	manifest := RepoManifest{Repos: []RepoEntry{
		{ID: "OnlyRepo", LocalRoot: `C:\Agent\OnlyRepo`,
			TaskProvider: &TaskProvider{Kind: "github_issues", Repo: registryProvider}},
	}}

	var polledRepos []string
	dispatcher := newFakeDispatcher()
	dispatchFor := func(repo RegistryRepo) (RepoDispatch, error) {
		// Capture the providerRepo the consumer threads from the registry entry, and a
		// provider that records the repo it is asked to list.
		provider := &capturingProvider{record: &polledRepos}
		return RepoDispatch{Provider: provider, Dispatcher: dispatcher, Sizer: fixedIdleSizer(4)}, nil
	}
	consumer := NewRegistryConsumer(func() (RepoManifest, error) { return manifest, nil }, dispatchFor)
	if _, err := consumer.DrainOnce(context.Background()); err != nil {
		t.Fatalf("DrainOnce: %v", err)
	}

	if len(polledRepos) != 1 || polledRepos[0] != registryProvider {
		t.Fatalf("polled repos = %v, want only the registry task_provider.repo %q (NOT the env %q)", polledRepos, registryProvider, decoy)
	}
	for _, r := range polledRepos {
		if r == decoy {
			t.Fatalf("consumer polled the env QUEUE_DRAIN_REPO %q; the provider repo must come from the registry", decoy)
		}
	}
}

// capturingProvider records the repo string it is asked to list, so a test can prove
// which provider repo the consumer threads into ListReadyIssues.
type capturingProvider struct {
	record *[]string
}

func (p *capturingProvider) ListReadyIssues(repo string) ([]IssueRef, error) {
	*p.record = append(*p.record, repo)
	return nil, nil
}

func (p *capturingProvider) CloseIssue(string, int) error { return nil }

func (p *capturingProvider) DequeueIssue(string, int) error { return nil }
