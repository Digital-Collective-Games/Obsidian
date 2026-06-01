package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/config"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/controlplane"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/taskrun"
)

// recordingDequeueProvider records the (repo, number) the dequeue endpoint asked the
// task provider to set not-ready, so the endpoint test can assert it went THROUGH the
// provider (never an inline gh call) and never closed the issue.
type recordingDequeueProvider struct {
	calls []struct {
		repo   string
		number int
	}
}

func (p *recordingDequeueProvider) DequeueIssue(repo string, number int) error {
	p.calls = append(p.calls, struct {
		repo   string
		number int
	}{repo, number})
	return nil
}

// poolMux builds a mux over a real-git worktree root with a fixture registry at the
// configured OBSIDIAN_REGISTRY_PATH, for the Task-0016 pool endpoints.
func poolMux(t *testing.T) (*http.ServeMux, string) {
	mux, _, _ := poolMuxWithService(t)
	return mux, ""
}

// poolMuxWithService is poolMux exposing the underlying task service so a test can inject
// a fake dequeue provider.
func poolMuxWithService(t *testing.T) (*http.ServeMux, *taskrun.Service, string) {
	t.Helper()
	worktreeRoot := writeTaskTrackingRoot(t)
	registry := filepath.Join(t.TempDir(), "REPO-MANIFEST.json")
	if err := os.WriteFile(registry, []byte(`{
  "repos": [
    {"id": "obsidian", "local_root": "`+filepath.ToSlash(worktreeRoot)+`", "queue_workers": 4,
     "task_provider": {"kind": "github_issues", "repo": "gregsemple2003/obsidian"}},
    {"id": "demo", "local_root": "C:\\Agent\\Demo", "queue_workers": 2,
     "task_provider": {"kind": "github_issues", "repo": "gregsemple2003/demo"}}
  ]
}`), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	taskService := taskrun.NewService(worktreeRoot, filepath.Join(worktreeRoot, ".runs"), newFakeTaskRuntime())
	mux := NewMux(config.Config{
		BindAddress:     "127.0.0.1:4318",
		JobsRoot:        t.TempDir(),
		WorktreeRoot:    worktreeRoot,
		TrackingRoot:    filepath.Join(worktreeRoot, "Tracking"),
		RegistryPath:    registry,
		Namespace:       "default",
		TaskQueue:       "codex-orchestration",
		TemporalAddress: "127.0.0.1:7233",
	}, controlplane.NewService(t.TempDir(), newFakeBackend()), taskService, nil)
	return mux, taskService, worktreeRoot
}

// Task-0016 PASS-0002 / AC2 + AC9: POST /api/v1/worktrees/create provisions an idle
// pool worktree, and GET /api/v1/worktrees lists it with status + worktree_id +
// worktree_path (the full-pool read shape).
func TestWorktreeCreateThenFullPoolRead(t *testing.T) {
	mux, _ := poolMux(t)

	createResp := httptest.NewRecorder()
	mux.ServeHTTP(createResp, httptest.NewRequest(http.MethodPost, "/api/v1/worktrees/create",
		strings.NewReader(`{"repo":"obsidian"}`)))
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", createResp.Code, createResp.Body.String())
	}
	var created taskrun.PoolWorktree
	if err := json.Unmarshal(createResp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.Status != "idle" || created.WorktreeID == "" || created.WorktreePath == "" {
		t.Fatalf("created worktree = %#v, want idle with id+path", created)
	}

	listResp := httptest.NewRecorder()
	mux.ServeHTTP(listResp, httptest.NewRequest(http.MethodGet, "/api/v1/worktrees", nil))
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", listResp.Code)
	}
	var payload struct {
		Worktrees []taskrun.PoolWorktree `json:"worktrees"`
	}
	if err := json.Unmarshal(listResp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(payload.Worktrees) != 1 {
		t.Fatalf("full pool = %d, want 1: %s", len(payload.Worktrees), listResp.Body.String())
	}
	wt := payload.Worktrees[0]
	if wt.Status != "idle" || wt.WorktreeID != created.WorktreeID || wt.WorktreePath == "" {
		t.Fatalf("listed worktree = %#v, want idle %q", wt, created.WorktreeID)
	}
	// AC9: the full-pool read carries status + worktree_id; the §8 shape.
	if !strings.Contains(listResp.Body.String(), `"status":"idle"`) ||
		!strings.Contains(listResp.Body.String(), `"worktree_id"`) {
		t.Fatalf("full-pool read missing status/worktree_id: %s", listResp.Body.String())
	}
}

// AC7: POST /api/v1/worktrees/destroy removes an idle worktree (200); the pool read no
// longer lists it.
func TestWorktreeDestroyIdle(t *testing.T) {
	mux, _ := poolMux(t)
	createResp := httptest.NewRecorder()
	mux.ServeHTTP(createResp, httptest.NewRequest(http.MethodPost, "/api/v1/worktrees/create",
		strings.NewReader(`{"repo":"obsidian"}`)))
	var created taskrun.PoolWorktree
	_ = json.Unmarshal(createResp.Body.Bytes(), &created)

	destroyResp := httptest.NewRecorder()
	mux.ServeHTTP(destroyResp, httptest.NewRequest(http.MethodPost, "/api/v1/worktrees/destroy",
		strings.NewReader(`{"worktree_id":"`+created.WorktreeID+`"}`)))
	if destroyResp.Code != http.StatusOK {
		t.Fatalf("destroy status = %d, want 200: %s", destroyResp.Code, destroyResp.Body.String())
	}

	listResp := httptest.NewRecorder()
	mux.ServeHTTP(listResp, httptest.NewRequest(http.MethodGet, "/api/v1/worktrees", nil))
	var payload struct {
		Worktrees []taskrun.PoolWorktree `json:"worktrees"`
	}
	_ = json.Unmarshal(listResp.Body.Bytes(), &payload)
	if len(payload.Worktrees) != 0 {
		t.Fatalf("full pool after destroy = %d, want 0", len(payload.Worktrees))
	}
}

// AC3/AC5 (endpoint): POST /api/v1/worktrees/assign binds Task-0008 onto a chosen idle
// worktree (202, reusing it — no fresh dir), which then reads as allocated in the pool;
// AC4: a second assign with no idle worktree left is rejected 409.
func TestWorktreeAssignBindsIdleThenRejectsWhenNoneIdle(t *testing.T) {
	mux, _ := poolMux(t)
	createResp := httptest.NewRecorder()
	mux.ServeHTTP(createResp, httptest.NewRequest(http.MethodPost, "/api/v1/worktrees/create",
		strings.NewReader(`{"repo":"obsidian"}`)))
	var created taskrun.PoolWorktree
	_ = json.Unmarshal(createResp.Body.Bytes(), &created)

	assignResp := httptest.NewRecorder()
	mux.ServeHTTP(assignResp, httptest.NewRequest(http.MethodPost, "/api/v1/worktrees/assign",
		strings.NewReader(`{"task_id":"Task-0008","repo":"obsidian","worktree_id":"`+created.WorktreeID+`"}`)))
	if assignResp.Code != http.StatusAccepted {
		t.Fatalf("assign status = %d, want 202: %s", assignResp.Code, assignResp.Body.String())
	}

	// The assigned worktree now reads allocated, bound to Task-0008, at the SAME path.
	listResp := httptest.NewRecorder()
	mux.ServeHTTP(listResp, httptest.NewRequest(http.MethodGet, "/api/v1/worktrees", nil))
	var payload struct {
		Worktrees []taskrun.PoolWorktree `json:"worktrees"`
	}
	_ = json.Unmarshal(listResp.Body.Bytes(), &payload)
	if len(payload.Worktrees) != 1 {
		t.Fatalf("pool = %d, want 1: %s", len(payload.Worktrees), listResp.Body.String())
	}
	wt := payload.Worktrees[0]
	if wt.Status != "allocated" || wt.WorktreeID != created.WorktreeID || wt.WorktreePath != created.WorktreePath {
		t.Fatalf("assigned worktree = %#v, want allocated %q at the same path", wt, created.WorktreeID)
	}

	// AC4: with no idle worktree left, a second assign is rejected 409.
	rejectResp := httptest.NewRecorder()
	mux.ServeHTTP(rejectResp, httptest.NewRequest(http.MethodPost, "/api/v1/worktrees/assign",
		strings.NewReader(`{"task_id":"Task-0008","repo":"obsidian"}`)))
	if rejectResp.Code != http.StatusConflict {
		t.Fatalf("assign with no idle worktree status = %d, want 409: %s", rejectResp.Code, rejectResp.Body.String())
	}
}

// AC10: GET /api/v1/repos returns id + local_root + task_provider_repo from the
// registry, with NO queue_workers in the response.
func TestReposEndpointReadsRegistryWithoutQueueWorkers(t *testing.T) {
	mux, _ := poolMux(t)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/repos", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("repos status = %d, want 200: %s", resp.Code, resp.Body.String())
	}
	if strings.Contains(resp.Body.String(), "queue_workers") {
		t.Fatalf("repos response must not carry queue_workers: %s", resp.Body.String())
	}
	var payload struct {
		Repos []taskrun.RepoView `json:"repos"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode repos: %v", err)
	}
	if len(payload.Repos) != 2 {
		t.Fatalf("repos = %d, want 2: %s", len(payload.Repos), resp.Body.String())
	}
	if payload.Repos[0].ID != "demo" || payload.Repos[1].ID != "obsidian" {
		t.Fatalf("repos not sorted by id: %#v", payload.Repos)
	}
}

// AC6/AC13 (endpoint): POST /api/v1/worktrees/eject on an allocated worktree returns it to
// idle in the pool view (folder kept) and dequeues the freed task through the provider.
func TestWorktreeEjectReturnsIdleAndDequeues(t *testing.T) {
	mux, taskService, _ := poolMuxWithService(t)
	fake := &recordingDequeueProvider{}
	taskService.SetDequeueProvider(fake)

	createResp := httptest.NewRecorder()
	mux.ServeHTTP(createResp, httptest.NewRequest(http.MethodPost, "/api/v1/worktrees/create",
		strings.NewReader(`{"repo":"obsidian"}`)))
	var created taskrun.PoolWorktree
	_ = json.Unmarshal(createResp.Body.Bytes(), &created)

	assignResp := httptest.NewRecorder()
	mux.ServeHTTP(assignResp, httptest.NewRequest(http.MethodPost, "/api/v1/worktrees/assign",
		strings.NewReader(`{"task_id":"Task-0008","repo":"obsidian","worktree_id":"`+created.WorktreeID+`"}`)))
	if assignResp.Code != http.StatusAccepted {
		t.Fatalf("assign status = %d, want 202: %s", assignResp.Code, assignResp.Body.String())
	}
	var assigned taskrun.TaskRunView
	_ = json.Unmarshal(assignResp.Body.Bytes(), &assigned)

	ejectResp := httptest.NewRecorder()
	mux.ServeHTTP(ejectResp, httptest.NewRequest(http.MethodPost, "/api/v1/worktrees/eject",
		strings.NewReader(`{"run_id":"`+assigned.RunID+`"}`)))
	if ejectResp.Code != http.StatusOK {
		t.Fatalf("eject status = %d, want 200: %s", ejectResp.Code, ejectResp.Body.String())
	}
	var ejected taskrun.PoolWorktree
	_ = json.Unmarshal(ejectResp.Body.Bytes(), &ejected)
	if ejected.Status != "idle" || ejected.WorktreeID != created.WorktreeID {
		t.Fatalf("ejected worktree = %#v, want idle %q", ejected, created.WorktreeID)
	}

	// The pool view shows it idle, and the freed task was dequeued (Task-0008 -> #8).
	listResp := httptest.NewRecorder()
	mux.ServeHTTP(listResp, httptest.NewRequest(http.MethodGet, "/api/v1/worktrees", nil))
	var payload struct {
		Worktrees []taskrun.PoolWorktree `json:"worktrees"`
	}
	_ = json.Unmarshal(listResp.Body.Bytes(), &payload)
	if len(payload.Worktrees) != 1 || payload.Worktrees[0].Status != "idle" {
		t.Fatalf("pool after eject = %#v, want one idle worktree", payload.Worktrees)
	}
	// BUG-0001: the in-app Eject routes the Queue->Never write to the EJECTED worktree's
	// own repo (record.Repo == "obsidian", set when the worktree was created), so the
	// multi-repo dashboard writes to the CORRECT repo's provider (not a hardcoded/empty one).
	if len(fake.calls) != 1 || fake.calls[0].number != 8 || fake.calls[0].repo != "obsidian" {
		t.Fatalf("eject dequeue calls = %#v, want one dequeue for repo obsidian issue #8 (Task-0008)", fake.calls)
	}
}

// AC15: POST /api/v1/worktrees/dequeue invokes the provider dequeue (Queue -> Never) for
// the named task WITHOUT ejecting and does NOT close the issue.
func TestWorktreeDequeueEndpointInvokesProviderWithoutClosing(t *testing.T) {
	mux, taskService, _ := poolMuxWithService(t)
	fake := &recordingDequeueProvider{}
	taskService.SetDequeueProvider(fake)

	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/v1/worktrees/dequeue",
		strings.NewReader(`{"repo":"gregsemple2003/obsidian","task_id":"Task-0007"}`)))
	if resp.Code != http.StatusOK {
		t.Fatalf("dequeue status = %d, want 200: %s", resp.Code, resp.Body.String())
	}
	if len(fake.calls) != 1 || fake.calls[0].repo != "gregsemple2003/obsidian" || fake.calls[0].number != 7 {
		t.Fatalf("dequeue provider calls = %#v, want one call for repo/issue #7", fake.calls)
	}
}

// AC11/AC15: the new routes are method/path-guarded — 405 on the wrong method, 404 on an
// unknown sub-path, matching the existing handler guards.
func TestWorktreePoolRouteGuards(t *testing.T) {
	mux, _ := poolMux(t)

	cases := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"create wrong method", http.MethodGet, "/api/v1/worktrees/create", http.StatusMethodNotAllowed},
		{"assign wrong method", http.MethodGet, "/api/v1/worktrees/assign", http.StatusMethodNotAllowed},
		{"destroy wrong method", http.MethodGet, "/api/v1/worktrees/destroy", http.StatusMethodNotAllowed},
		{"dequeue wrong method", http.MethodGet, "/api/v1/worktrees/dequeue", http.StatusMethodNotAllowed},
		{"eject wrong method", http.MethodGet, "/api/v1/worktrees/eject", http.StatusMethodNotAllowed},
		{"unknown subpath", http.MethodPost, "/api/v1/worktrees/bogus", http.StatusNotFound},
		{"repos wrong method", http.MethodPost, "/api/v1/repos", http.StatusMethodNotAllowed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			mux.ServeHTTP(resp, httptest.NewRequest(tc.method, tc.path, nil))
			if resp.Code != tc.want {
				t.Fatalf("%s %s status = %d, want %d", tc.method, tc.path, resp.Code, tc.want)
			}
		})
	}
}
