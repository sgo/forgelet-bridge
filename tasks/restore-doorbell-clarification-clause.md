# restore-doorbell-clarification-clause

# Restore the doorbell clarification clause

## Operator-observed need

Creating the `tic-tac-toe` Family failed in the Host's `composing` phase.
`get-swarm-forge lieutenant` installed the bridge kit, then its doorbell
self-check failed:

`the ring it would type for a clarification never says "the answer is the
operator's to give"`

An isolated reproduction produced:

```text
[self-check] Clarification for forgelet-bridge from coder
Answer with: pack_dashboard_request.sh answer self-check ./tmp/answer.txt
```

`forgelet-bridge` commit `c16efd8` (`Sync SwarmForge runtime`) removed
`clarification-request?`, the clarification branch in `answer-reminder`, and the
operator-answer clause from `swarmforge/scripts/doorbell.bb`. The stricter
installer self-check was left intact, so the kit now rejects its own doorbell.
This regressed previously completed doorbell behavior and blocks every fresh
Family composition.

## Desired outcome

The bridge's canonical doorbell source emits the operator-answer clause again
for clarification-shaped rings. The existing installer self-check continues to
reject any doorbell that loses the clause. A fresh Family composition completes
through bridge kit installation and can then be started and registered normally.

## Plan

1. Restore clarification recognition in the bridge's canonical
   `swarmforge/scripts/doorbell.bb`.
2. Restore the clarification-specific `answer-reminder` text, including that
   the answer is the operator's to give and must not be supplied unless the
   operator says so.
3. Keep approval-gate wording independent and unchanged.
4. Keep the installer's approval and clarification self-checks strict; do not
   weaken them to accept a clause-free doorbell.
5. Add or restore focused unit and Gherkin coverage for both ring forms and for
   a kit whose doorbell has lost the clause.
6. Build the installer from the committed source and reproduce the exact
   `get-swarm-forge lieutenant` composition in a disposable directory with
   `SWARMFORGE_LIEUTENANT_AGENT=claude`.
7. Verify the composition reaches its successful completion message, then move
   the disposable directory to Trash or otherwise remove only that verified
   scratch path.
8. Push the reviewed bridge source. The operator can then retry the Host's
   normal Family creation.

## Boundaries

- Do not weaken, skip or ignore the installer self-check.
- Do not create a fake `tic-tac-toe-forge` or register a partial Family.
- Do not use `SWARMFORGE_SKIP_BRIDGE` for acceptance; that would bypass the
  failing path.
- Do not modify an active forge root or restart unrelated projects.
- Do not combine this regression repair with broader request-scheduling work.
- Do not treat a unit test of `print-ring` alone as sufficient: the built
  installer must exercise the shipped kit.
- Preserve existing approval semantics and generated kit permissions.

## Testing and evidence

- The bridge tests prove an approval ring carries the operator-gate clause.
- The bridge tests prove a clarification ring carries the operator-answer
  clause.
- A doorbell missing either clause still fails its installer self-check.
- The installer test proves a valid kit succeeds and reports the actual reads
  it performed.
- The focused tests and required bridge gates pass on the exact committed tree.
- A disposable `get-swarm-forge lieutenant` run passes kit installation,
  finishes composition and records the resulting commit/tree under test.
- The failing Host error is no longer reproducible after the fix.

## Verification

The canonical doorbell again emits the clarification operator-answer clause
while preserving the approval operator-gate clause. The same runtime sync had
also removed the existing role-health clarification wait; that previously
covered behavior was restored so the required QA suite could pass.

- Focused Go tests passed for `internal/doorbell`, `internal/kit` and
  `cmd/install-kit`.
- The focused Gherkin outline passed all four examples: both ring shapes and
  independent loss of either installer-checked clause.
- `code`, `clean`, `architect`, `harden`, `clean-final` and `qa` passed on the
  implementation tree. The full QA run passed 258 scenario runs; mutation killed
  every selected Go mutant and all four Gherkin example mutations.
- The disposable `get-swarm-forge lieutenant` E2E used a `git archive HEAD`
  snapshot rather than a live checkout, set
  `SWARMFORGE_LIEUTENANT_AGENT=claude`, and did not set
  `SWARMFORGE_SKIP_BRIDGE`. It installed the kit, passed the approval and
  clarification self-checks, and reached the composition success message at
  bridge commit `43d4559` and tree `46f7daa`.
- A failed first E2E attempt was retained for diagnosis, then moved to Trash.
  The passing E2E run removed only its verified scratch and snapshot paths.
- Documentation: `README.md` now tells installers that fresh Family composition
  carries both doorbell clauses; `docs/manual.md` documents the public
  `install-kit` behavior; `docs/adding-a-forge.md` documents both ring checks.
  `ARCHITECTURE.md` did not change because this repair restores the existing
  kit boundary and adds no package or ownership decision.

## Operator acceptance

- owner: operator | environment: Forgelet Host | checks: retry the normal `tic-tac-toe` Family creation now that the repaired bridge is pushed, and confirm composition completes without disabling bridge installation | status: pending
