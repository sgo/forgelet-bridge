# Waiting-facts Matrix backend policy evidence

Selected feature: `features/waiting-facts-as-state.feature`
Recorded: `20261109T152610Z`

| Backend | Ordinary | Wall | Mutation total | Killed | Survived | Errors |
|---|---:|---:|---:|---:|---:|---:|
| fresh | pass | 39.75s | 0 | 0 | 0 | 0 |
| memory | pass | 22.14s | 0 | 0 | 0 | 0 |

Fresh compatibility selection for the unlisted feature: `fresh`.
Ordinary equivalence: pass.
Mutation classification equivalence: pass.

## Timing

| Measure | Before | After |
|---|---:|---:|
| Selected waiting-facts feature | 40.1s fresh | 22.1s memory |
| Full QA | 856.9s | pending |
| Full harden | 567.0s | pending |

The selected-feature before/after ratio is 1.81x.
The operator's original selected-feature baseline was 32.9s; the current-tree
fresh measurement is retained separately so both comparisons are visible.

## Recommendation

Keep memory for the waiting-facts feature, keep every unlisted feature on
fresh by default, and retain the required fresh-Synapse device, encryption
and restart compatibility lane. The policy validator refuses an attempt to
move that required lane to a non-fresh backend.
