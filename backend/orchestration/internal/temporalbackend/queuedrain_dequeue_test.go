package temporalbackend

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/queue"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/taskrun"
)

// recordingProvider records the (repo, number) dequeue write calls and FATALS on any
// CloseIssue, so a test asserts the control-plane dequeue routed the Queue->Never write
// through the resolved provider with the correct repo + issue number and NEVER closed the
// issue (human-only closure preserved).
type recordingProvider struct {
	t     *testing.T
	calls []struct {
		repo   string
		number int
	}
}

func (p *recordingProvider) ListReadyIssues(string) ([]queue.IssueRef, error) { return nil, nil }

func (p *recordingProvider) CloseIssue(string, int) error {
	p.t.Fatalf("dequeue must NEVER close the issue (human-only closure)")
	return nil
}

func (p *recordingProvider) DequeueIssue(repo string, number int) error {
	p.calls = append(p.calls, struct {
		repo   string
		number int
	}{repo, number})
	return nil
}

// writeTwoRepoRegistry writes a fixture registry with two github_issues repos and one
// non-github_issues repo (no resolvable provider), at a temp path.
func writeTwoRepoRegistry(t *testing.T) string {
	t.Helper()
	registry := filepath.Join(t.TempDir(), "REPO-MANIFEST.json")
	if err := os.WriteFile(registry, []byte(`{
  "repos": [
    {"id": "obsidian", "local_root": "C:\\Agent\\CodexDashboard",
     "task_provider": {"kind": "github_issues", "repo": "gregsemple2003/obsidian"}},
    {"id": "demo", "local_root": "C:\\Agent\\Demo",
     "task_provider": {"kind": "github_issues", "repo": "gregsemple2003/demo"}},
    {"id": "noprovider", "local_root": "C:\\Agent\\NoProvider"}
  ]
}`), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	return registry
}

// newRecordingDequeueProvider builds the registry-backed control-plane dequeue provider
// with its inner gh build replaced by a recording provider, so the resolver's repo->slug
// resolution and the per-slug write routing are provable WITHOUT real GitHub.
func newRecordingDequeueProvider(t *testing.T, registryPath string) (*registryDequeueProvider, *recordingProvider) {
	t.Helper()
	rec := &recordingProvider{t: t}
	p := newControlPlaneDequeueProvider(registryPath)
	p.build = func(string) (queue.QueueProvider, error) { return rec, nil }
	return p, rec
}

// Task-0016 BUG-0001 fix: the dashboard control-plane dequeue provider resolves a worktree's
// repo to that repo's task_provider slug and routes the Queue->Never write to the CORRECT
// repo, by BOTH a registry id ("obsidian") and an already-resolved slug
// ("gregsemple2003/demo"), through the provider — never closing the issue.
func TestControlPlaneDequeueResolvesRepoToCorrectProvider(t *testing.T) {
	registry := writeTwoRepoRegistry(t)
	p, rec := newRecordingDequeueProvider(t, registry)

	// Eject of an obsidian worktree passes record.Repo == "obsidian" (a registry id); it
	// must resolve to gregsemple2003/obsidian and write issue #16.
	if err := p.DequeueIssue("obsidian", 16); err != nil {
		t.Fatalf("dequeue by registry id: %v", err)
	}
	// The standalone dequeue endpoint passes an already-resolved slug; it must route there
	// unchanged (to the OTHER repo) with its own issue number.
	if err := p.DequeueIssue("gregsemple2003/demo", 7); err != nil {
		t.Fatalf("dequeue by slug: %v", err)
	}

	if len(rec.calls) != 2 {
		t.Fatalf("dequeue calls = %#v, want 2", rec.calls)
	}
	if rec.calls[0].repo != "gregsemple2003/obsidian" || rec.calls[0].number != 16 {
		t.Fatalf("call[0] = %#v, want gregsemple2003/obsidian #16", rec.calls[0])
	}
	if rec.calls[1].repo != "gregsemple2003/demo" || rec.calls[1].number != 7 {
		t.Fatalf("call[1] = %#v, want gregsemple2003/demo #7", rec.calls[1])
	}
}

// A repo with no resolvable GitHub provider is a SAFE no-op (no write, no error, no
// provider build): an unknown repo, a non-github_issues entry, and an empty registry path
// all dequeue to nothing rather than crash the dashboard.
func TestControlPlaneDequeueSafeNoOpWhenNoProvider(t *testing.T) {
	registry := writeTwoRepoRegistry(t)

	t.Run("unknown repo", func(t *testing.T) {
		p, rec := newRecordingDequeueProvider(t, registry)
		if err := p.DequeueIssue("does-not-exist", 16); err != nil {
			t.Fatalf("unknown repo dequeue must be a safe no-op, got: %v", err)
		}
		if len(rec.calls) != 0 {
			t.Fatalf("unknown repo dequeue calls = %#v, want 0", rec.calls)
		}
	})

	t.Run("entry without a task provider", func(t *testing.T) {
		p, rec := newRecordingDequeueProvider(t, registry)
		if err := p.DequeueIssue("noprovider", 16); err != nil {
			t.Fatalf("no-provider repo dequeue must be a safe no-op, got: %v", err)
		}
		if len(rec.calls) != 0 {
			t.Fatalf("no-provider repo dequeue calls = %#v, want 0", rec.calls)
		}
	})

	t.Run("empty registry path", func(t *testing.T) {
		p, rec := newRecordingDequeueProvider(t, "")
		if err := p.DequeueIssue("obsidian", 16); err != nil {
			t.Fatalf("empty-registry dequeue must be a safe no-op, got: %v", err)
		}
		if len(rec.calls) != 0 {
			t.Fatalf("empty-registry dequeue calls = %#v, want 0", rec.calls)
		}
	})
}

// TestControlPlaneWiringRoutesServiceDequeueToResolvedRepo proves the EXACT production
// composition main.go performs (BUG-0001): a multi-repo dashboard taskrun.Service injected
// with NewControlPlaneDequeueProvider routes the Service-level Queue->Never write — the same
// call the in-app Eject and the standalone dequeue endpoint make — to the worktree's
// resolved repo provider with the correct issue number, never closing the issue. The inner
// gh build is recorded so no real GitHub is touched.
func TestControlPlaneWiringRoutesServiceDequeueToResolvedRepo(t *testing.T) {
	registry := writeTwoRepoRegistry(t)
	provider, rec := newRecordingDequeueProvider(t, registry)

	// Build the dashboard Service exactly as cmd/controlplane/main.go does (a single
	// multi-repo NewService) and inject the registry-backed dequeue provider.
	root := t.TempDir()
	service := taskrun.NewService(root, filepath.Join(root, ".runs"), nil)
	service.SetDequeueProvider(provider)

	// Eject of an "obsidian" worktree, and the standalone dequeue endpoint for a "demo"
	// task, both call Service.DequeueTask(repo, taskID); each must route to its own repo.
	if err := service.DequeueTask("obsidian", "Task-0016"); err != nil {
		t.Fatalf("dequeue obsidian: %v", err)
	}
	if err := service.DequeueTask("gregsemple2003/demo", "Task-0007"); err != nil {
		t.Fatalf("dequeue demo: %v", err)
	}

	if len(rec.calls) != 2 {
		t.Fatalf("service dequeue calls = %#v, want 2", rec.calls)
	}
	if rec.calls[0].repo != "gregsemple2003/obsidian" || rec.calls[0].number != 16 {
		t.Fatalf("call[0] = %#v, want gregsemple2003/obsidian #16", rec.calls[0])
	}
	if rec.calls[1].repo != "gregsemple2003/demo" || rec.calls[1].number != 7 {
		t.Fatalf("call[1] = %#v, want gregsemple2003/demo #7", rec.calls[1])
	}
}
