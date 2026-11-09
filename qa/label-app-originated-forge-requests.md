# Label app-originated forge requests

This procedure exercises the completed-card report presentation through the
real bridge and encrypted Matrix path:

1. Start the pinned Synapse fixture, a fixture forge dashboard and the current
   bridge binary.
2. Seed dashboard requests with and without
   `source: app.completed-card-report`.
3. Decrypt the bridge's Matrix events as the configured operator and verify the
   exact `From the Forgelet app:` presentation and encrypted transport.
4. Answer the dashboard requests and verify the Lieutenant's replies remain in
   the correct request threads.
5. Run a prior-state case and a restart case, checking that the app report is
   not re-threaded, duplicated or converted into a second dashboard request.
6. Send an ordinary operator Matrix message and verify its existing pairing and
   thread behavior.

The procedure uses only project-local fixtures and needs no operator-supplied
secret.
