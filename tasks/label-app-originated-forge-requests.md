# label-app-originated-forge-requests

# Label app-originated forge requests

## Operator-observed problem

When the app submits a completed-card report, the bridge discovers the pending
dashboard request and posts it into Element X under the bridge account. It
therefore looks like an outgoing message from the bridge rather than a report
the operator sent from the app. Matrix cannot truthfully impersonate the
operator for an app-created event.

## Desired behaviour

Read the safe request source exposed by the dashboard. For a request marked
`app.completed-card-report`, post it as an explicit app-relayed report, for
example with a clear `From the Forgelet app` prefix or an equivalent distinct
notice, while retaining a usable thread root for the Lieutenant's answer.

Requests originating as real operator Matrix messages retain their current
pairing and thread behavior. The bridge must not create a second dashboard
request merely to change presentation.

## Dependency

This card depends on the shared-layer request source metadata and the host
setting that source. Do not infer sender identity from message text.

## Testing

Cover app-originated reports with and without prior bridge state, preserved
threading for the answer, restart idempotence, ordinary operator messages, and
requests without source metadata. Keep encrypted room behavior and the
existing operator allowlist unchanged.

## Out of scope

Changing the report contents, adding Matrix user impersonation, or redesigning
the app's Lieutenant Chat panel.

## Source contract

The dashboard request file may carry an optional `source` header. This card
uses only the exact safe value `app.completed-card-report`; the bridge never
infers an origin from the request or report text. A request without source
metadata, or with any other source value, keeps the existing presentation.

An app-originated completed-card report is posted under the bridge Matrix
account with the exact prefix:

```text
From the Forgelet app:
```

followed by one blank line and the unchanged report body. The posted event is
the dashboard request's thread root, so the Lieutenant's answer uses the same
thread. The bridge does not queue another dashboard request to change how the
report is presented.

The source survives the request moving from pending to done so an answered
report is not mistaken for an ordinary request on first delivery after a
restart.

## Acceptance

`features/label-app-originated-forge-requests.feature` is the executable
contract. Its value-driven scenarios cover:

- app-originated reports with and without prior bridge thread bookkeeping;
- the explicit prefix and encrypted delivery;
- the Lieutenant's answer remaining in the app-relay message's thread;
- exactly one dashboard request before and after delivery;
- restart idempotence;
- ordinary requests without source metadata;
- ordinary operator Matrix messages retaining their own pairing and thread.

The existing acceptance suites continue to prove the operator allowlist and
encrypted chat, approvals, activity, and clarification rooms. No allowlist or
room-encryption rule changes in this card.

## E2E procedure

The card registers `qa/e2e.edn` and ships
`qa/label-app-originated-forge-requests.{md,sh}`. The procedure starts the
pinned Synapse fixture, the real forge dashboard and bridge, and the encrypted
operator client, then runs this card's Gherkin feature through the public
Matrix path. It needs no operator-supplied secret.

## Documentation

- `README.md` describes app-relayed completed-card reports in quick-start
  channel behavior.
- `docs/manual.md` documents the `source` value, exact presentation, thread
  behavior and ordinary fallback.
- `ARCHITECTURE.md` records that source metadata is normalized in the
  dashboard/relay policy layer and Matrix remains a transport.
- `docs/manual-contract.edn` links the chat source behavior to an executable
  API example.
