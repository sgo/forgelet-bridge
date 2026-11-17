# Accept superseded automation resolutions E2E

Run from the `forgelet-bridge` checkout:

```sh
bash qa/accept-superseded-automation-resolutions.sh
```

The procedure first runs the focused Gherkin feature through the real bridge
and its fixture Matrix backend. Its resolved-approval outline covers
`approved`, `sent_back`, `automatic_spec_approval`, `auto_approved` and
`superseded`; only the first two select manual Matrix marks.

It then executes one real `Bridge.Tick` through the production dashboard
adapters and the currently composed bridge configuration. A test room boundary
stands in for Matrix, so the tick reads the composed Family dashboards without
contacting or changing the live homeserver. All four configured Families must
be reached, with no unhappy Family, projection fallback or resolved-approval
error.

The focused unit checks prove that a fixture carrying `superseded` preserves
that exact value and produces the distinct threaded reply, while an unknown
ending remains rejected and cannot enter the compatibility fallback. The
procedure reads the local bridge configuration but prints only Family names and
counts, never credentials.
