# Real-Subagent Drain Run — Closing the Spike's One Gap

This artifact supplements [DRAIN-QUEUE-DEMO.md](./DRAIN-QUEUE-DEMO.md). The spike
proved the full drain-queue *orchestration shell* (pull → allocate worktree →
dispatch → concurrency cap → capture → release → commit/test) but its "subagent"
was a deterministic local executor
([apply_modification.py](./apply_modification.py)), not a real AI agent, because
nested agent-dispatch tooling was not exposed to the worker run.

This run closes that one gap: **real AI subagents**, dispatched in parallel under
a concurrency cap of 2, each confined to its own allocated git worktree, each
given only the task (no pre-written patch), making genuine edits to the program.
It was driven by the TaskDispatch **coordinator** as the sanctioned, honestly
labeled fallback for the worker's nested-dispatch tooling limit.

## What ran

- Target repo: `C:\Agent\YourTestRepo`, baseline `9af2adf` on `main`.
- Two worktrees allocated off baseline (`git worktree add`):
  - `C:\Agent\YourTestRepo_rt1` → branch `real/RT-001`
  - `C:\Agent\YourTestRepo_rt2` → branch `real/RT-002`
- Two **real AI subagents** dispatched **concurrently** (concurrency = 2), each
  handed only its worktree path + task + acceptance criteria, told to read the
  code and figure out the edit itself.
- After both returned, the coordinator **independently re-ran** the tests and
  functional spot-checks (did not trust the agents' self-reported pass),
  committed each agent's work on its branch, and **released both worktrees**.

## Tasks (real agent work, not pre-encoded edits)

| Task | Assignment | Branch | Commit | Tests (independently re-run) |
| --- | --- | --- | --- | --- |
| RT-001 | "Add a `mod` (modulo) operation" | `real/RT-001` | `e9919a7` | 4/4 pass (incl. `test_mod`) |
| RT-002 | "Add a `--list` flag that prints operation names" | `real/RT-002` | `165ab8b` | 4/4 pass (incl. `test_list_flag`) |

Evidence the edits were genuine agent judgment, not templated: RT-001 also
updated the usage docstring (the deterministic spike did not), and RT-002 wrote a
proper `redirect_stdout` capture test and placed the flag handler before the
3-argument usage check on its own.

## Independent coordinator verification (not the agents' self-report)

RT-001:
```
python -m unittest test_calc -v  → Ran 4 tests ... OK (test_add, test_mod, test_mul, test_sub)
python calc.py mod 7 4           → 3.0
python calc.py add 2 3           → 5.0   (regression intact)
```

RT-002:
```
python -m unittest test_calc -v  → Ran 4 tests ... OK (test_add, test_list_flag, test_mul, test_sub)
python calc.py --list            → add / sub / mul   (exit 0)
python calc.py add 2 3           → 5.0   (regression intact)
python calc.py mul 4 3           → 12.0  (regression intact)
```

`git worktree list` after release shows only `C:/Agent/YourTestRepo [main]` —
both real-run worktrees removed.

## Diffs produced by the real subagents

RT-001 (`real/RT-001`):
```diff
diff --git a/calc.py b/calc.py
@@ Operations docstring
+    mod   remainder of a divided by b (a % b)
@@ after def mul
+def mod(a, b):
+    return a % b
@@ OPERATIONS
+    "mod": mod,
diff --git a/test_calc.py b/test_calc.py
+    def test_mod(self):
+        self.assertEqual(calc.mod(7, 4), 3)
+        self.assertIn("mod", calc.OPERATIONS)
+        self.assertEqual(calc.OPERATIONS["mod"](7, 4), 3)
```

RT-002 (`real/RT-002`):
```diff
diff --git a/calc.py b/calc.py
@@ def main(argv):
+    if len(argv) == 1 and argv[0] == "--list":
+        for op_name in OPERATIONS:
+            print(op_name)
+        return 0
     if len(argv) != 3:
diff --git a/test_calc.py b/test_calc.py
+import io
+from contextlib import redirect_stdout
+    def test_list_flag(self):
+        buf = io.StringIO()
+        with redirect_stdout(buf):
+            rc = calc.main(["--list"])
+        self.assertEqual(rc, 0)
+        printed = buf.getvalue().splitlines()
+        for op_name in calc.OPERATIONS:
+            self.assertIn(op_name, printed)
```

## What this proves vs. what is still follow-on

Proven now (the part the directive emphasized — "ask *it* to do minor things to
modify some program"):

- Real AI subagents, dispatched concurrently under a cap, each isolated in its
  own allocated worktree, autonomously read the program and produced correct,
  faithful, test-backed edits; worktrees were allocated and released cleanly.

Still follow-on (honest):

- **Dispatch seam wiring.** Here the coordinator dispatched the real agents
  directly. The in-repo consumer (`Drain-Queue.ps1` → `Invoke-Subagent.ps1`)
  still calls the deterministic executor. Production wiring = have the consumer
  shell out to a headless agent CLI per worktree, or run the consumer itself as
  an agent-driven coordinator. The execution shape is identical; only the
  dispatch call changes.
- **Concurrency backfill past the cap** (queue larger than the cap, with
  backfill as slots free) was proven *mechanically* by the spike run
  ([DRAIN-RUN.log](./DRAIN-RUN.log)); this real-agent run used 2 tasks at cap 2
  in parallel, so it demonstrates real concurrent isolation but not backfill.
- **GitHub-backed queue** (vs local JSON) and **merge/PR/push of drained work**
  remain follow-on, same as the spike.
