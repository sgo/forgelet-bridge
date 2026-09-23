# runs-clean-up-after-themselves

The project's throwaway output grows without limit, the way nothing under a build
tool's directory would.

Every acceptance scenario starts its own Synapse and clients and leaves them under
`<worktree>/build/acceptance/run/<scenario>/`; every mutant in a mutation run leaves
a whole run under `<worktree>/build/acceptance-mutation/<feature>/mutations/mN/`.
Neither is ever removed. Measured today in one project: 8,700 scenario directories
and 4.8 GB of mutant runs — 32 GB across its worktrees, against 1.5 MB of code and
500 MB of binaries. A Maven project cannot get there: the debris would sit under
`target/` and the first `mvn clean` would erase it. This project's Makefile has
build, test, property, acceptance and acceptance-mutation and no `clean`, so nothing
wipes anything, and the same debris is already growing in every forge that runs the
suite.

Desired behaviour: the runs clean up after themselves. Keep the newest few of each
kind — a scenario run is a few megabytes, a mutant run is hundreds, so they deserve
different limits — and remove the rest as the run finishes, so a failure can still be
looked at without the directory becoming something that grows without limit. Report
what was removed, or that nothing needed removing, so the behaviour is visible
rather than assumed.

And a `clean` target beside build and test, so a person can do by hand what the runs
should do anyway. Pairing it with a rebuild has to stay safe: the bridge binary lives
in `build/acceptance/bin`, beside the debris, and the bridge is running from it.

Whatever pruning exists must never touch a run that is still in flight — the newest
of each kind is exactly what a running suite is using.
