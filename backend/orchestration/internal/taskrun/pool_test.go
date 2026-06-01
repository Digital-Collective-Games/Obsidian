package taskrun

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Task-0016 PASS-0000: the durable pool record carries the four mandatory fields
// (worktree_id, stable worktree_path, repo, run_id-or-empty); an IDLE member
// (run_id == "") persists and is enumerated rather than dropped; and the stable
// worktree id is byte-identical across two reads.

func newPoolTestService(t *testing.T, repoNamespace string) *Service {
	t.Helper()
	root := t.TempDir()
	service := NewService(root, filepath.Join(root, ".runs"), newFakeRuntime())
	service.SetRepoNamespace(repoNamespace)
	// Pin the owned-lane root under the temp dir so the pool layout is isolated and
	// inspectable (defaultOwnedLaneRoot points at the shared OS temp dir on Windows).
	service.ownedLaneRoot = filepath.Join(root, "owned-lanes")
	return service
}

// writeIdlePoolMember materializes one idle pool member folder + its `w` checkout +
// a durable pool record with run_id == "", as Create will, and returns the record.
func writeIdlePoolMember(t *testing.T, s *Service, seq int) poolRecord {
	t.Helper()
	checkout := s.poolCheckoutPath(seq)
	if err := os.MkdirAll(checkout, 0o755); err != nil {
		t.Fatalf("mkdir pool checkout: %v", err)
	}
	record := poolRecord{
		WorktreeID:   s.poolWorktreeID(seq),
		WorktreePath: checkout,
		Repo:         s.poolRepoSegment(),
		RunID:        "",
	}
	if err := s.writePoolRecord(seq, record); err != nil {
		t.Fatalf("write pool record: %v", err)
	}
	return record
}

func TestPoolRecordPersistsIdleMemberAndStableID(t *testing.T) {
	service := newPoolTestService(t, "obsidian")
	want := writeIdlePoolMember(t, service, 1)

	if want.RunID != "" {
		t.Fatalf("idle member should persist with empty run_id, got %q", want.RunID)
	}
	if want.WorktreeID != "obsidian/wt-0001" {
		t.Fatalf("worktree id = %q, want obsidian/wt-0001", want.WorktreeID)
	}

	// Read the record back twice and assert the id (and the rest of the record) is
	// byte-stable across reads — a record that cannot represent run_id == "" or an
	// unstable id fails this pass.
	memberDir := service.poolMemberDir(1)
	first, ok, err := readPoolRecord(memberDir)
	if err != nil || !ok {
		t.Fatalf("read pool record (1): ok=%v err=%v", ok, err)
	}
	second, ok, err := readPoolRecord(memberDir)
	if err != nil || !ok {
		t.Fatalf("read pool record (2): ok=%v err=%v", ok, err)
	}
	if first != second {
		t.Fatalf("pool record not stable across reads: %#v vs %#v", first, second)
	}
	if first.WorktreeID != want.WorktreeID {
		t.Fatalf("read worktree id = %q, want %q", first.WorktreeID, want.WorktreeID)
	}
	if first.RunID != "" {
		t.Fatalf("read run_id = %q, want empty (idle)", first.RunID)
	}
	if first.WorktreePath != want.WorktreePath {
		t.Fatalf("read worktree path = %q, want %q", first.WorktreePath, want.WorktreePath)
	}
}

func TestEnumeratePoolRecordsSurfacesIdleMembers(t *testing.T) {
	service := newPoolTestService(t, "obsidian")
	idle := writeIdlePoolMember(t, service, 1)

	// A second member marked allocated (non-empty run_id) to prove both are surfaced.
	allocatedSeq := 2
	checkout := service.poolCheckoutPath(allocatedSeq)
	if err := os.MkdirAll(checkout, 0o755); err != nil {
		t.Fatalf("mkdir allocated checkout: %v", err)
	}
	allocated := poolRecord{
		WorktreeID:   service.poolWorktreeID(allocatedSeq),
		WorktreePath: checkout,
		Repo:         service.poolRepoSegment(),
		RunID:        "taskrun--obsidian--Task-0007--active",
	}
	if err := service.writePoolRecord(allocatedSeq, allocated); err != nil {
		t.Fatalf("write allocated pool record: %v", err)
	}

	records, err := service.enumeratePoolRecords()
	if err != nil {
		t.Fatalf("enumerate pool records: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("enumerated %d records, want 2 (idle must not be dropped): %#v", len(records), records)
	}
	gotIdle, ok := records[idle.WorktreeID]
	if !ok {
		t.Fatalf("idle member %q not enumerated", idle.WorktreeID)
	}
	if gotIdle.RunID != "" {
		t.Fatalf("idle member run_id = %q, want empty", gotIdle.RunID)
	}
	gotAllocated, ok := records[allocated.WorktreeID]
	if !ok {
		t.Fatalf("allocated member %q not enumerated", allocated.WorktreeID)
	}
	if gotAllocated.RunID != allocated.RunID {
		t.Fatalf("allocated member run_id = %q, want %q", gotAllocated.RunID, allocated.RunID)
	}
}

// poolDiscoverRuntime is a minimal Runtime that reports a live run for exactly one
// run id (the allocated member's), and ErrRunNotFound for everything else. It models a
// backend RESTART: the in-memory service is fresh, but the per-run workflow store still
// holds the live run for the allocated pool member.
type poolDiscoverRuntime struct {
	liveRunID  string
	liveTaskID string
}

func (r poolDiscoverRuntime) StartTaskRun(context.Context, StartTaskRunRequest) (TaskRunView, error) {
	return TaskRunView{}, ErrRunNotFound
}
func (r poolDiscoverRuntime) GetTaskRun(_ context.Context, runID string) (TaskRunView, error) {
	return r.GetActiveTaskRun(context.Background(), runID)
}
func (r poolDiscoverRuntime) GetActiveTaskRun(_ context.Context, runID string) (TaskRunView, error) {
	if runID != r.liveRunID {
		return TaskRunView{}, ErrRunNotFound
	}
	return TaskRunView{
		RunID:  r.liveRunID,
		TaskID: r.liveTaskID,
		Status: "active",
		RepoLane: RepoLane{Binding: &RepoBinding{
			TaskID:       r.liveTaskID,
			RunGateState: RunGateStateRunning,
		}},
	}, nil
}
func (r poolDiscoverRuntime) SetRunGateState(context.Context, string, string) (TaskRunView, error) {
	return TaskRunView{}, ErrRunNotFound
}
func (r poolDiscoverRuntime) BindLaunchedSession(context.Context, string, string, string, int) (TaskRunView, error) {
	return TaskRunView{}, ErrRunNotFound
}
func (r poolDiscoverRuntime) ReconcileTaskSnapshot(context.Context, string, TaskDefinitionSnapshot) (TaskRunView, error) {
	return TaskRunView{}, ErrRunNotFound
}
func (r poolDiscoverRuntime) UpdateTaskRun(context.Context, string, TaskRunUpdate) (TaskRunView, error) {
	return TaskRunView{}, ErrRunNotFound
}
func (r poolDiscoverRuntime) RetryTaskRunWorkload(context.Context, string, WorkloadRetryRequest) (TaskRunView, error) {
	return TaskRunView{}, ErrRunNotFound
}

func writeAllocatedPoolMember(t *testing.T, s *Service, seq int, runID string) poolRecord {
	t.Helper()
	checkout := s.poolCheckoutPath(seq)
	if err := os.MkdirAll(checkout, 0o755); err != nil {
		t.Fatalf("mkdir allocated checkout: %v", err)
	}
	record := poolRecord{
		WorktreeID:   s.poolWorktreeID(seq),
		WorktreePath: checkout,
		Repo:         s.poolRepoSegment(),
		RunID:        runID,
	}
	if err := s.writePoolRecord(seq, record); err != nil {
		t.Fatalf("write allocated pool record: %v", err)
	}
	return record
}

// Task-0016 PASS-0001 / AC8 / REG-008: discover-on-startup reconstructs each pool
// member's idle-vs-allocated state from disk + the LIVE per-run workflow across a
// backend restart, with NO bound state lost. The falsifier (a discover that
// reclassifies a live-allocated worktree as idle, or drops it) fails.
func TestDiscoverPoolReconstructsAllocatedAndIdleAcrossRestart(t *testing.T) {
	// Lay the on-disk pool down with one Service.
	layout := newPoolTestService(t, "obsidian")
	liveRunID := ActiveRunIDForRepo("obsidian", "Task-0007")
	allocated := writeAllocatedPoolMember(t, layout, 1, liveRunID)
	idle := writeIdlePoolMember(t, layout, 2)

	// Construct a FRESH Service over the same roots (simulating a backend restart): a
	// new in-memory service, with the runtime still reporting the live run for wt-0001.
	fresh := NewService(layout.declaredWorktreeRoot, filepath.Join(layout.declaredWorktreeRoot, ".runs"),
		poolDiscoverRuntime{liveRunID: liveRunID, liveTaskID: "Task-0007"})
	fresh.SetRepoNamespace("obsidian")
	fresh.ownedLaneRoot = layout.ownedLaneRoot

	pool, err := fresh.ListPoolWorktrees()
	if err != nil {
		t.Fatalf("list pool worktrees after restart: %v", err)
	}
	if len(pool) != 2 {
		t.Fatalf("pool size after restart = %d, want 2: %#v", len(pool), pool)
	}
	byID := map[string]PoolWorktree{}
	for _, wt := range pool {
		byID[wt.WorktreeID] = wt
	}

	gotAllocated, ok := byID[allocated.WorktreeID]
	if !ok {
		t.Fatalf("allocated member %q dropped by discover", allocated.WorktreeID)
	}
	if gotAllocated.Status != poolStatusAllocated {
		t.Fatalf("allocated member status = %q, want allocated (bound state must survive restart)", gotAllocated.Status)
	}
	if gotAllocated.RunID != liveRunID {
		t.Fatalf("allocated member run id = %q, want %q", gotAllocated.RunID, liveRunID)
	}
	if gotAllocated.Binding == nil || gotAllocated.Binding.TaskID != "Task-0007" {
		t.Fatalf("allocated member must keep its bound task: %#v", gotAllocated.Binding)
	}

	gotIdle, ok := byID[idle.WorktreeID]
	if !ok {
		t.Fatalf("idle member %q dropped by discover", idle.WorktreeID)
	}
	if gotIdle.Status != poolStatusIdle {
		t.Fatalf("idle member status = %q, want idle", gotIdle.Status)
	}
}

// A pool member whose bound run has ENDED (the workflow no longer reports it live) is
// reconstructed as idle, and DiscoverPool persists the corrected run_id == "" back onto
// the durable record (the folder is kept).
func TestDiscoverPoolReclassifiesEndedRunToIdleAndPersists(t *testing.T) {
	service := newPoolTestService(t, "obsidian")
	// Runtime reports NOTHING live, so the bound run is treated as ended.
	service.runtime = poolDiscoverRuntime{liveRunID: "no-such-run", liveTaskID: "none"}
	endedRunID := ActiveRunIDForRepo("obsidian", "Task-0007")
	member := writeAllocatedPoolMember(t, service, 1, endedRunID)

	if err := service.DiscoverPool(); err != nil {
		t.Fatalf("discover pool: %v", err)
	}

	// The durable record now reads idle (run_id cleared), and the folder is kept.
	record, ok, err := readPoolRecord(service.poolMemberDir(1))
	if err != nil || !ok {
		t.Fatalf("read pool record after discover: ok=%v err=%v", ok, err)
	}
	if record.RunID != "" {
		t.Fatalf("ended-run member run_id = %q, want cleared to idle", record.RunID)
	}
	if _, err := os.Stat(member.WorktreePath); err != nil {
		t.Fatalf("discover must keep the folder for a reclassified-idle member: %v", err)
	}
}

func TestNextPoolMemberSeqAllocatesStableIncrementingIDs(t *testing.T) {
	service := newPoolTestService(t, "obsidian")

	// Empty pool -> first member is wt-0001.
	seq, err := service.nextPoolMemberSeq()
	if err != nil {
		t.Fatalf("next seq (empty): %v", err)
	}
	if seq != 1 {
		t.Fatalf("first sequence = %d, want 1", seq)
	}

	writeIdlePoolMember(t, service, 1)
	writeIdlePoolMember(t, service, 2)

	seq, err = service.nextPoolMemberSeq()
	if err != nil {
		t.Fatalf("next seq (two members): %v", err)
	}
	if seq != 3 {
		t.Fatalf("next sequence = %d, want 3", seq)
	}
	if service.poolWorktreeID(seq) != "obsidian/wt-0003" {
		t.Fatalf("next worktree id = %q, want obsidian/wt-0003", service.poolWorktreeID(seq))
	}
}
