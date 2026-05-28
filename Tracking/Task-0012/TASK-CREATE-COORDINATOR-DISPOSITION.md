# TaskCreate Coordinator Disposition

## Human Directive

The human rejected the agent-audit-driven revision chain and directed the
coordinator to restore the first draft:

- active draft: [TASK.md](./TASK.md)
- restored source snapshot:
  [TASK-DRAFT-0001-BEFORE-AUDIT.md](./TASK-DRAFT-0001-BEFORE-AUDIT.md)

The human also directed that the durable process correction is:

- auditor preferences are second to human directives

## Disposition

The coordinator restored `TASK.md` from
`TASK-DRAFT-0001-BEFORE-AUDIT.md`.

This bypasses the later agent-audit revision chain by human direction. The
current active draft must not be presented as the draft that passed
`TASK-AUDIT-0009.md`; that ready verdict applied to a later draft that is now
preserved only as history.

## Preserved History

The prior audit/revision artifacts remain in the task directory as historical
evidence:

- [TASK-AUDIT-0001.md](./TASK-AUDIT-0001.md)
- [TASK-AUDIT-0002.md](./TASK-AUDIT-0002.md)
- [TASK-AUDIT-0003.md](./TASK-AUDIT-0003.md)
- [TASK-AUDIT-0004.md](./TASK-AUDIT-0004.md)
- [TASK-AUDIT-0005.md](./TASK-AUDIT-0005.md)
- [TASK-AUDIT-0006.md](./TASK-AUDIT-0006.md)
- [TASK-AUDIT-0007.md](./TASK-AUDIT-0007.md)
- [TASK-AUDIT-0008.md](./TASK-AUDIT-0008.md)
- [TASK-AUDIT-0009.md](./TASK-AUDIT-0009.md)

## Shared Process Update

The durable precedence rule was promoted into shared TaskCreate and task-audit
process docs under the user-level `.codex` orchestration docs.
