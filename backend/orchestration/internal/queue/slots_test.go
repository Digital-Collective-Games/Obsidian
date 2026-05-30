package queue

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEvaluateSlotAdmitsWhileFreeSlotRemains(t *testing.T) {
	cases := []struct {
		name      string
		limit     int
		used      int
		wantAdmit bool
		wantAvail int
	}{
		{name: "empty repo admits", limit: 4, used: 0, wantAdmit: true, wantAvail: 4},
		{name: "one used still admits", limit: 4, used: 1, wantAdmit: true, wantAvail: 3},
		{name: "second sibling admits while three free", limit: 4, used: 1, wantAdmit: true, wantAvail: 3},
		{name: "last slot admits", limit: 4, used: 3, wantAdmit: true, wantAvail: 1},
		{name: "fifth dispatch refused when full", limit: 4, used: 4, wantAdmit: false, wantAvail: 0},
		{name: "over-full refused", limit: 4, used: 6, wantAdmit: false, wantAvail: 0},
		{name: "limit one is single concurrency", limit: 1, used: 1, wantAdmit: false, wantAvail: 0},
		{name: "non-positive limit falls back to default", limit: 0, used: 4, wantAdmit: false, wantAvail: 0},
		{name: "non-positive limit admits below default", limit: 0, used: 2, wantAdmit: true, wantAvail: 2},
		{name: "negative used normalized to zero", limit: 4, used: -3, wantAdmit: true, wantAvail: 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision := EvaluateSlot(tc.limit, tc.used)
			if decision.Admit != tc.wantAdmit {
				t.Fatalf("admit = %v, want %v (decision = %#v)", decision.Admit, tc.wantAdmit, decision)
			}
			if decision.Available() != tc.wantAvail {
				t.Fatalf("available = %d, want %d (decision = %#v)", decision.Available(), tc.wantAvail, decision)
			}
			if tc.wantAdmit && decision.Reason != "" {
				t.Fatalf("admitted decision should have no reason, got %q", decision.Reason)
			}
			if !tc.wantAdmit && decision.Reason == "" {
				t.Fatal("refused decision should carry a reason")
			}
		})
	}
}

func TestEvaluateSlotNormalizesLimitToDefault(t *testing.T) {
	decision := EvaluateSlot(-1, 0)
	if decision.Limit != DefaultQueueWorkers {
		t.Fatalf("limit = %d, want default %d", decision.Limit, DefaultQueueWorkers)
	}
	if !decision.Admit {
		t.Fatal("an empty repo with the default limit should admit")
	}
}

func TestLoadManifestResolvesQueueWorkersByLocalRoot(t *testing.T) {
	repoRoot := t.TempDir()
	manifest := `{
  "schema_version": 1,
  "manifest_type": "codex_repo_registry",
  "repos": [
    {
      "id": "CodexDashboard",
      "queue_workers": 4,
      "local_root": "` + jsonPath(repoRoot) + `",
      "task_provider": {"kind": "github_issues"}
    }
  ]
}`
	writeFile(t, filepath.Join(repoRoot, ManifestFileName), manifest)

	loaded, err := LoadManifest(repoRoot)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	workers, ok := loaded.QueueWorkersForRoot(repoRoot)
	if !ok {
		t.Fatal("expected the matching repo entry to supply queue_workers")
	}
	if workers != 4 {
		t.Fatalf("queue_workers = %d, want 4", workers)
	}
}

func TestQueueWorkersForRootMatchesAcrossSeparatorAndTrailingSlash(t *testing.T) {
	manifest := RepoManifest{Repos: []RepoEntry{{
		ID:           "CodexDashboard",
		LocalRoot:    "C:\\Agent\\CodexDashboard",
		QueueWorkers: 4,
	}}}

	for _, probe := range []string{
		"C:\\Agent\\CodexDashboard",
		"C:/Agent/CodexDashboard",
		"C:/Agent/CodexDashboard/",
		"c:\\agent\\codexdashboard",
	} {
		workers, ok := manifest.QueueWorkersForRoot(probe)
		if !ok || workers != 4 {
			t.Fatalf("probe %q => workers %d ok %v, want 4 true", probe, workers, ok)
		}
	}
}

func TestQueueWorkersForRootFallsBackToDefaultWhenUnmatched(t *testing.T) {
	manifest := RepoManifest{Repos: []RepoEntry{{
		ID:           "CodexDashboard",
		LocalRoot:    "C:\\Agent\\CodexDashboard",
		QueueWorkers: 4,
	}}}
	workers, ok := manifest.QueueWorkersForRoot("C:\\Agent\\SomeOtherRepo")
	if ok {
		t.Fatal("an unmatched root should not report a configured value")
	}
	if workers != DefaultQueueWorkers {
		t.Fatalf("fallback workers = %d, want default %d", workers, DefaultQueueWorkers)
	}
}

func TestQueueWorkersForRootFallsBackWhenEntryOmitsField(t *testing.T) {
	manifest := RepoManifest{Repos: []RepoEntry{{
		ID:        "CodexDashboard",
		LocalRoot: "C:\\Agent\\CodexDashboard",
	}}}
	workers, ok := manifest.QueueWorkersForRoot("C:\\Agent\\CodexDashboard")
	if ok {
		t.Fatal("an entry without queue_workers should not report a configured value")
	}
	if workers != DefaultQueueWorkers {
		t.Fatalf("fallback workers = %d, want default %d", workers, DefaultQueueWorkers)
	}
}

func jsonPath(p string) string {
	// JSON-escape backslashes so a Windows TempDir embeds cleanly in the literal.
	out := make([]rune, 0, len(p))
	for _, r := range p {
		if r == '\\' {
			out = append(out, '\\', '\\')
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
