# Notify typed project clarifications through the bridge E2E

Run from the `forgelet-bridge` checkout:

```sh
bash qa/notify-typed-project-clarifications-through-bridge.sh
```

The procedure uses temporary Forgelet roots, real project and Family
dashboards, fake dashboards and Matrix fixtures. It requires no operator secret
and does not contact or modify an active forge.

The Gherkin run proves one unseen notification, no duplicate after repeated
ticks or restart, seen/answered/withdrawn handling, fresh failure recovery,
project-qualified identities, Family isolation, legacy coexistence, bounded
excerpts, malformed-row failure and one automation read per dashboard per tick.

The real-dashboard procedure then creates a typed clarification through
`clarify-chat`, observes its bounded redacted projection, answers it through the
bridge's normal project-chat route, verifies the answer is queued for the owning
project agent, and verifies the project attention marker clears.
