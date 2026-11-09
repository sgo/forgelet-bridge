# Prototype an in-memory Matrix homeserver

This procedure reruns the measured Matrix fixture experiment:

1. Run the selected `forge-display-names` feature three times against fresh
   Synapse, process-shared Synapse and the in-memory Matrix server.
2. Measure wall time, CPU time, maximum resident size and per-world setup,
   bridge startup, scenario and teardown phases.
3. Run all eight selected-feature Gherkin mutations against every backend.
4. Require ordinary and mutation outcomes to match.
5. Regenerate the checked-in evidence summary and verify that it retains the
   fresh-Synapse compatibility recommendation and does not change production
   defaults.

No operator-supplied secret is required.
