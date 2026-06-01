# PASS-0000 — Pool record + stable identity

Backend pass 1 of the Task-0016 backend batch (single-context implementation; see
[HANDOFF.md](../../HANDOFF.md) "Process gaps" — no nested dispatch tool, so this was
implemented directly by the task leader, not a delegated implementer/QA lane).

## Objective (TASK.md §2, Goal 2; AC2 foundation)

Replace the random-temp model's *identity* with a **stable path + stable
`worktree_id`** and a durable **pool record** that can carry an **idle**
(`run_id == ""`) member.

## Changes

- New file [pool.go](../../../../backend/orchestration/internal/taskrun/pool.go):
  - Stable layout: `worktree_path = <ownedLaneRoot>/<repoSegment>/wt-<NNNN>/w`,
    `worktree_id = <repoSegment>/wt-<NNNN>` (zero-pad width 4 — an impl detail).
    `repoSegment` prefers the Service's `repoNamespace` (registry repo id) and falls
    back to a sanitized `repoIdentity()` for the manual/single-repo control plane.
  - Durable per-folder pool record `worktree-pool.json` (sibling of the `w` checkout)
    carrying the four mandatory fields: `worktree_id`, stable `worktree_path`, `repo`,
    and `run_id`-or-empty (empty = idle). A **sibling per-folder record** was chosen
    over extending `owned-lane-bootstrap.json` (TASK.md Open-Questions allows either)
    because the bootstrap breadcrumb only exists once a run has bootstrapped a lane,
    whereas an idle member must persist on disk with **no run bound** — the sibling
    record is the cleaner home for idle persistence.
  - `nextPoolMemberSeq()` allocates the next free `wt-<NNNN>` by scanning existing
    member folders; `enumeratePoolRecords()` surfaces idle members (a folder that
    exists with `run_id == ""`) instead of dropping them.
  - `poolMemberDirForID()` resolves a `<repo>/wt-<NNNN>` id to its folder and rejects
    an id that does not belong to this Service's repo segment (so a caller can never
    act on another repo's member or an arbitrary path).
  - `PoolWorktree` full-pool view type with the `idle`/`allocated` `status`
    discriminator (consumed by the §8 read in later passes). Added as a NEW type rather
    than mutating the existing active-only `WorktreeBinding`, keeping the diff surgical.

No existing behavior changed in this pass: `pool.go` is additive. The random-temp
dispatch path, `queue_workers`, and the existing `/worktrees` active-only read are
untouched (they change in PASS-0002/0003).

## Proof — Go unit tests

[pool_test.go](../../../../backend/orchestration/internal/taskrun/pool_test.go):

- `TestPoolRecordPersistsIdleMemberAndStableID` — writes a pool record with
  `run_id == ""`, reads it back twice, asserts the record (and the id
  `obsidian/wt-0001`) is byte-stable across reads and that the idle member persists
  with an empty `run_id`. (Falsifier: a record that cannot represent `run_id == ""`,
  or an unstable id, fails.)
- `TestEnumeratePoolRecordsSurfacesIdleMembers` — one idle + one allocated member;
  asserts BOTH are enumerated (the idle member is not dropped just because no run is
  bound).
- `TestNextPoolMemberSeqAllocatesStableIncrementingIDs` — empty pool ⇒ `wt-0001`; two
  members ⇒ next is `wt-0003` with id `obsidian/wt-0003`.

### Commands

```
cd C:\Agent\CodexDashboard\backend\orchestration
go build ./...                                   # exit 0
go test ./internal/taskrun/...                   # ok (full package, 14.3s)
```

Both green. This is producer testing (the independent clean-context QA verdict is
coordinator-arranged after the backend batch — not claimed here).
