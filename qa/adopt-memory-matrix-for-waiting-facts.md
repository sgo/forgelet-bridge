# Adopt the memory backend for waiting-facts scenarios

This procedure verifies the per-feature Matrix backend policy through the same
acceptance entry points used by routine gates:

1. Run `features/waiting-facts-as-state.feature` with the checked-in policy,
   which selects memory.
2. Force fresh Synapse and require the same ordinary outcome.
3. Run the feature's Gherkin mutation classification through both backends and
   require identical totals.
4. Run an unlisted feature and require its generated acceptance worlds to select
   fresh Synapse.
5. Run the policy acceptance feature, including fresh fallback, focused
   overrides, checked-in compatibility selection and malformed or unsafe policy
   rejection.
6. Verify the checked-in evidence has equivalent outcomes and retains the
   before/after selected-feature, full QA and full harden timings.

No operator-supplied secret is required.
