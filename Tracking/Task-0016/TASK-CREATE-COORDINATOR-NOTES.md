# Task-0016 — TaskCreate Coordinator Review Notes

Reviewer: TaskCreate coordinator (keeper of human intention). Lens:
`TASK-AUDIT.md` concreteness checks (mechanism blur, weak acceptance, ambiguous
identities, proxy closure, missing falsifiers). Scope was **not** narrowed.

## Verdict

The writer's [`TASK.md`](./TASK.md) draft is concrete and audit-shaped: one live
solution shape, named files/routes/shapes, [BE]/[FE]-tagged falsifiable criteria,
explicit `What Does Not Count` and `Proof Plan`, exclusions E1–E7 under Non-Goals,
D1=replace enforced. Strong enough to present.

One **human-gate question** remains (a model ambiguity, not a concreteness fix I
may make unilaterally), plus two minor polish edits held until the model is
confirmed (to avoid churn if the model changes).

## Human-gate question — the idle-worktree model

The draft adopts a **free-slot/capacity model**: per repo, `idle = queue_workers −
allocated`; idle rows are free-capacity placeholders with **no checkout path**;
**Assign** provisions a fresh checkout into a free slot; **Eject** removes the
checkout and frees the slot. This is the only model coherent with exclusion **E1
(no Spawn Worktree)** — without a spawn/seed mechanism, no persistent idle
worktree *directory* can exist, so an idle row cannot carry a path.

Tension to confirm with the human:
- Bullet 1 of the spec asks every worktree row to show "local dir (copy icon to
  copy local directory path)". Under the capacity model, **idle rows have no path**
  (the checkout doesn't exist until Assign). The Stitch mockup *does* show idle
  rows with paths (`/var/lib/codex/worktrees/agent-pool-01`), but the mockup also
  has the now-excluded Spawn Worktree button that would have created them.
- Alternative (persistent idle-directory pool) would let idle rows show paths, but
  needs a way to seed idle directories — which contradicts E1. So it cannot be
  adopted without re-opening E1.

Recommendation: confirm the **capacity model** (idle rows show repo + "free slot",
no path/copy until assigned; copy-path applies to allocated rows). If the human
wants persistent idle directories with paths, that re-opens E1 (needs a
seed/spawn) and the Assign/Eject/idle sections must be rewritten accordingly.

## Minor polish (held until model confirmed)

1. **Idle count identity:** state `idle = max(0, queue_workers − allocated)` and
   how an over-cap repo (allocated > cap, e.g. a stale lane) is displayed, so
   acceptance criterion 2's "allocated + idle = capacity" is well-defined at the
   edge.
2. **Assign vs. Dispatch:** make explicit that Assign is essentially cap-guarded,
   repo-targeted dispatch (request `{task_id, repo}`, reuses
   provision→bootstrap→start, rejects at cap) — so a reader does not think a brand
   new mechanism distinct from `Dispatch()` is required.

## Provider-binding gate (post-approval)

Not yet done (correctly). After human approval, before calling Task-0016 created
/ enqueue-ready: create the GitHub issue #16 and write `TASK-META.json` binding it
(issue-first, id == task number) via the `obsidian-operator` skill.

---

## v2 review — 2026-05-31 (manual-pool pivot; supersedes v1 above)

The human pivoted the model (manual persistent worktree pool; `queue_workers`
removed; E1 reversed) and split scope (Q1=a): **Task-0016 is now backend-only**;
the Tk UI is Task-0017. The v1 capacity-model question and v1 polish items above
are **resolved/void** by that pivot. The draft was rewritten accordingly
([`TASK.md`](./TASK.md)).

Re-review verdict: **ready to present.** Concrete and audit-shaped — one live
solution shape (one model swap), merge earned, 11 pass/fail [BE] criteria, the
`queue_workers` removal enumerated across `RepoEntry` / `RepoSlotLimit` /
`SlotSizer` / `EvaluateSlot` / `EffectiveFreeConcurrency` / `NewServiceForRepo`,
Eject-keeps-folder vs Destroy-deletes sharp, discover-on-restart pinned to the
Landing-2 live binding, REG-007/008/009 reinterpreted ("cap=1" → "pool of 1").
No blockers; open questions carry non-decisive defaults.

Items to surface to the human (not blockers):

1. **Eject is a worktree-pool op, not a task-state op.** Eject frees the worktree
   but does not touch the GitHub issue/Queue state (task lifecycle stays on the
   GitHub surface). Consequence: if the issue is still `Queue=Ready`, the consumer
   may **re-dispatch** the task into another idle worktree on a later poll. This is
   internally consistent with human-only closure (Eject is operator-initiated and
   does not auto-close), but the operator should know "eject ≠ stop the task; set
   Queue=Never / close the issue to stop it." Worth a one-line confirm.

Small recordings made directly (coordinator edits, no writer relaunch needed):

- Recorded the human's tab-rename request (`TASKS` → `WORKTREES`) as a **Task-0017**
  (UI) requirement on the follow-on line of `TASK.md`. It is frontend, so out of
  this backend task; it will be formally captured when Task-0017 is drafted.

---

## v3 review — 2026-05-31 (Eject dequeues via the task provider)

The human resolved the v2 Eject behavioral item: Eject must take the freed task
out of the queue, routed **through the task provider** ("dequeue"), not internal
state. Recorded in HUMAN-DIRECTIVES "UPDATE 2"; the writer made a targeted revision
([`TASK.md`](./TASK.md)).

Re-review verdict: **ready to present.** The addition is concrete and clean —
the dequeue is a new write on the existing `QueueProvider` interface
(`provider.go`, symmetric to the existing `CloseIssue` write and the `Queue` read),
via the injectable `run` func so tests never touch GitHub; Eject couples to it;
the standalone `POST /api/v1/worktrees/dequeue` exposes it. Falsifiers are strong:
criterion 13 includes the **load-bearing** variant (Eject-without-dequeue must show
re-dispatch), and criterion 14 asserts `CloseIssue` is never invoked (dequeue ≠
close → human-only closure preserved). The testing nuance (product `Queue=Never`
write vs the REG-007 human-UI Ready-flip rule, both on throwaway reg007 testbeds)
is recorded in the Proof Plan. No blockers; open questions are non-decisive
defaults. Awaiting human approval → then the provider-binding gate (issue #16 +
`TASK-META.json`).

