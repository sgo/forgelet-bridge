# Waiting-facts Matrix backend policy evidence

Selected feature: `features/waiting-facts-as-state.feature`
Recorded: `20261109T162023Z`

| Backend | Ordinary | Wall | Mutation total | Killed | Survived | Errors |
|---|---:|---:|---:|---:|---:|---:|
| fresh | pass | 36.79s | 0 | 0 | 0 | 0 |
| memory | pass | 20.56s | 0 | 0 | 0 | 0 |

Fresh compatibility selection for the unlisted feature: `fresh`.
Ordinary equivalence: pass.
Mutation classification equivalence: pass.

## Timing

| Measure | Before | After |
|---|---:|---:|
| Selected waiting-facts feature | 40.1s fresh | 20.6s memory |
| Full QA | 856.9s | 833.9s |
| Full harden | 567.0s | 14.6s |

The selected-feature before/after ratio is 1.95x.
The after harden time is this card's changed-feature differential gate.
The before figure is the broader migration reference recorded by the card,
so only the selected-feature comparison is like-for-like.

The operator's original selected-feature baseline was 32.9s; the current-tree
fresh measurement is retained separately so both comparisons are visible.

## Recommendation

Keep memory for the waiting-facts feature, keep every unlisted feature on
fresh by default, and retain the required fresh-Synapse device, encryption
and restart compatibility lane. The policy validator refuses an attempt to
move that required lane to a non-fresh backend.
